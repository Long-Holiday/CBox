"""Automatic container health monitoring, periodic output sync, and recovery service."""

import json
from datetime import datetime, timezone
from typing import Optional
from sqlalchemy.orm import Session

from cbox.domain.container import ContainerSpec
from cbox.domain.enums import ContainerState, EventType, RestartPolicy, RuntimeState, VolumeMode
from cbox.domain.volume import VolumeMount
from cbox.services.container_service import ContainerService
from cbox.services.image_service import ImageService
from cbox.services.runtime_service import RuntimeService
from cbox.services.volume_service import VolumeService
from cbox.storage.repositories import ContainerRepository, EventRepository, RuntimeRepository
from cbox.transport.ssh import SSHTransport
from cbox.worker.executor import WorkerExecutor
from cbox.worker.health import WorkerHealth


class RecoveryService:
    """Monitors live containers, performs periodic output synchronization, and manages recovery."""

    def __init__(
        self,
        session: Session,
        container_service: Optional[ContainerService] = None,
        image_service: Optional[ImageService] = None,
        volume_service: Optional[VolumeService] = None,
        runtime_service: Optional[RuntimeService] = None,
    ):
        self.session = session
        self.container_repo = ContainerRepository(session)
        self.runtime_repo = RuntimeRepository(session)
        self.event_repo = EventRepository(session)
        self.container_service = container_service or ContainerService(session)
        self.image_service = image_service or ImageService(session)
        self.volume_service = volume_service or VolumeService(session)
        self.runtime_service = runtime_service or RuntimeService(session)

    async def sync_all_active_outputs(self) -> None:
        """Periodically sync output volumes from worker to server for all running containers."""
        running_containers = self.container_repo.list_containers(all_containers=False)
        for c in running_containers:
            model = self.container_repo.get_by_id_or_name(c.id)
            if not model or model.state != ContainerState.RUNNING.value or not model.runtime_id:
                continue

            runtime = self.runtime_repo.get_by_id(model.runtime_id)
            if not runtime:
                continue

            provider = self.runtime_service.get_provider(model.context_name)
            transport = provider.open_transport(runtime)

            for m in model.mounts:
                if m.mode == VolumeMode.OUTPUT.value:
                    try:
                        vm = VolumeMount(source=m.source, target=m.target, mode=VolumeMode.OUTPUT)
                        await self.volume_service.sync_mount_from_runtime(vm, transport)
                    except Exception:
                        pass

    async def run_health_check_cycle(self) -> None:
        """Single iteration of container and runtime health checking and auto-recovery."""
        all_models = self.container_repo.list_containers(all_containers=False)

        for item in all_models:
            model = self.container_repo.get_by_id_or_name(item.id)
            if not model or model.state != ContainerState.RUNNING.value or not model.runtime_id:
                continue

            cid = model.id
            runtime = self.runtime_repo.get_by_id(model.runtime_id)
            if not runtime:
                continue

            provider = self.runtime_service.get_provider(model.context_name)
            transport = provider.open_transport(runtime)

            # Check runtime status
            rt_status = await provider.get_runtime_status(runtime)

            if rt_status in (RuntimeState.LOST, RuntimeState.STOPPED):
                # Runtime was lost or preempted!
                self.container_repo.update_state(cid, ContainerState.INTERRUPTED)
                self.event_repo.record_event("container", cid, EventType.CONTAINER_INTERRUPTED.value)

                restart_pol = RestartPolicy(model.restart_policy)
                should_recover = restart_pol in (RestartPolicy.ALWAYS, RestartPolicy.UNLESS_STOPPED)

                if should_recover:
                    await self.recover_container(model.id)
                else:
                    self.container_repo.update_state(cid, ContainerState.FAILED, error_message="Runtime lost")
                continue

            # Runtime is alive, check process status inside tmux
            try:
                proc_state, exit_code, pid = await WorkerHealth.inspect_container_process(transport, cid)
                if proc_state in (ContainerState.EXITED, ContainerState.STOPPED, ContainerState.FAILED):
                    # Process completed! Sync output volumes
                    for m in model.mounts:
                        if m.mode in (VolumeMode.OUTPUT.value, VolumeMode.RW.value):
                            vm = VolumeMount(source=m.source, target=m.target, mode=VolumeMode(m.mode))
                            await self.volume_service.sync_mount_from_runtime(vm, transport)

                    # Update state
                    final_state = ContainerState.EXITED if exit_code == 0 else ContainerState.FAILED
                    self.container_repo.update_state(cid, final_state, exit_code=exit_code, pid=pid)
                    self.event_repo.record_event("container", cid, EventType.CONTAINER_EXIT.value, {"exit_code": exit_code})

                    # Release runtime
                    await self.runtime_service.release_runtime(runtime.id, keep_idle=True)

                    # Check restart policy on failure
                    if final_state == ContainerState.FAILED and RestartPolicy(model.restart_policy) == RestartPolicy.ON_FAILURE:
                        await self.recover_container(cid)
            except Exception:
                pass

    async def recover_container(self, container_id: str) -> None:
        """Execute automated recovery workflow according to plan.md section 37 & 38."""
        model = self.container_repo.get_by_id_or_name(container_id)
        if not model:
            return

        cid = model.id
        self.container_repo.update_state(cid, ContainerState.RECOVERING)
        self.event_repo.record_event("container", cid, EventType.CONTAINER_RECOVER_START.value)

        # Build container spec
        resume_cmd = json.loads(model.resume_command) if model.resume_command else json.loads(model.command)
        spec = ContainerSpec(
            name=model.name,
            image=model.image_id,
            command=resume_cmd,
            gpu=json.loads(model.gpu_requested),
            high_mem=model.high_mem,
            workdir=model.workdir,
            mounts=[VolumeMount(source=m.source, target=m.target, mode=VolumeMode(m.mode)) for m in model.mounts],
            context=model.context_name,
        )

        try:
            # 1. Provision new runtime
            runtime = await self.runtime_service.acquire_runtime(spec, model.image_id, cid)
            provider = self.runtime_service.get_provider(spec.context)
            transport = provider.open_transport(runtime)

            # 2. Materialize Image
            await self.image_service.materialize_image(runtime.id, model.image_id, transport)

            # 3. Restore Volumes (including restored checkpoints from server)
            for mount in spec.mounts:
                await self.volume_service.sync_mount_to_runtime(mount, runtime.id, transport)

            # 4. Prepare container and restart command (resume command)
            await WorkerExecutor.prepare_container(transport, cid, spec)
            await WorkerExecutor.start_container(transport, cid)

            # 5. Container back to RUNNING!
            self.container_repo.update_state(
                cid,
                ContainerState.RUNNING,
                runtime_id=runtime.id,
                gpu_assigned=runtime.accelerator,
            )
            self.event_repo.record_event("container", cid, EventType.CONTAINER_RECOVER_DONE.value)

        except Exception as e:
            self.container_repo.update_state(cid, ContainerState.FAILED, error_message=f"Recovery failed: {e}")
