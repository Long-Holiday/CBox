"""Container lifecycle orchestration service."""

import json
import uuid
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Optional
from sqlalchemy.orm import Session

from cbox.domain.container import ContainerInspect, ContainerSpec, ContainerStateInfo, ContainerSummary
from cbox.domain.enums import ContainerState, EventType, VolumeMode
from cbox.domain.volume import VolumeMount
from cbox.services.image_service import ImageService
from cbox.services.runtime_service import RuntimeService
from cbox.services.volume_service import VolumeService
from cbox.storage.repositories import ContainerRepository, EventRepository
from cbox.transport.ssh import SSHTransport
from cbox.utils.errors import ContainerStartError, NotFoundError, UsageError
from cbox.utils.hash import short_id
from cbox.utils.paths import default_paths
from cbox.worker.executor import WorkerExecutor
from cbox.worker.health import WorkerHealth
from cbox.worker.metrics import WorkerMetrics


class ContainerService:
    """Manages container creation, startup, execution, stopping, logs, and stats."""

    def __init__(
        self,
        session: Session,
        image_service: Optional[ImageService] = None,
        volume_service: Optional[VolumeService] = None,
        runtime_service: Optional[RuntimeService] = None,
    ):
        self.session = session
        self.repo = ContainerRepository(session)
        self.event_repo = EventRepository(session)
        self.image_service = image_service or ImageService(session)
        self.volume_service = volume_service or VolumeService(session)
        self.runtime_service = runtime_service or RuntimeService(session)

    def create_container(self, spec: ContainerSpec) -> str:
        """Create container specification without provisioning runtime (cbox create)."""
        cid = short_id(str(uuid.uuid4()))
        # Verify image exists
        manifest = self.image_service.get_image(spec.image)

        # Merge image defaults if not specified in spec
        final_cmd = spec.command or manifest.cmd
        final_workdir = spec.workdir or manifest.workdir

        mount_dicts = [
            {"source": m.source, "target": m.target, "mode": m.mode.value}
            for m in spec.mounts
        ]

        self.repo.save_container(
            id=cid,
            name=spec.name,
            image_id=manifest.id,
            command=final_cmd,
            workdir=final_workdir,
            gpu_requested=spec.gpu,
            restart_policy=spec.restart_policy.value,
            resume_command=spec.resume_command,
            high_mem=spec.high_mem,
            auto_remove=spec.auto_remove,
            context_name=spec.context,
            mounts=mount_dicts,
        )
        self.event_repo.record_event("container", cid, EventType.CONTAINER_CREATE.value, {"name": spec.name})
        return cid

    async def start_container(self, identifier: str) -> None:
        """Start container: provision/reuse runtime, materialize image, sync volumes, launch process."""
        model = self.repo.get_by_id_or_name(identifier)
        if not model:
            raise NotFoundError(f"Container not found: {identifier}")

        cid = model.id
        spec = ContainerSpec(
            name=model.name,
            image=model.image_id,
            command=json.loads(model.command),
            gpu=json.loads(model.gpu_requested),
            high_mem=model.high_mem,
            workdir=model.workdir,
            mounts=[VolumeMount(source=m.source, target=m.target, mode=VolumeMode(m.mode)) for m in model.mounts],
            context=model.context_name,
        )

        try:
            # 1. Transition: PROVISIONING
            self.repo.update_state(cid, ContainerState.PROVISIONING)
            self.event_repo.record_event("container", cid, EventType.CONTAINER_START.value)

            runtime = await self.runtime_service.acquire_runtime(spec, model.image_id, cid)
            provider = self.runtime_service.get_provider(spec.context)
            transport = provider.open_transport(runtime)

            # 2. Transition: PREPARING (Materialize image and volumes)
            self.repo.update_state(cid, ContainerState.PREPARING, runtime_id=runtime.id, gpu_assigned=runtime.accelerator)

            # Materialize Image
            self.event_repo.record_event("container", cid, EventType.IMAGE_MATERIALIZE_START.value)
            await self.image_service.materialize_image(runtime.id, model.image_id, transport)
            self.event_repo.record_event("container", cid, EventType.IMAGE_MATERIALIZE_DONE.value)

            # Synchronize Volumes
            for mount in spec.mounts:
                self.event_repo.record_event("container", cid, EventType.VOLUME_SYNC_START.value, {"volume": mount.source})
                await self.volume_service.sync_mount_to_runtime(mount, runtime.id, transport)
                self.event_repo.record_event("container", cid, EventType.VOLUME_SYNC_DONE.value, {"volume": mount.source})

            # Load any secrets requested
            secrets_dict = {}
            for sec_name in spec.secrets:
                sec_path = default_paths.secrets_dir / sec_name
                if sec_path.exists():
                    secrets_dict[sec_name] = sec_path.read_text(encoding="utf-8").strip()

            # 3. Transition: STARTING
            self.repo.update_state(cid, ContainerState.STARTING)
            await WorkerExecutor.prepare_container(transport, cid, spec, secrets_dict=secrets_dict)
            await WorkerExecutor.start_container(transport, cid)

            # 4. Transition: RUNNING
            self.repo.update_state(cid, ContainerState.RUNNING)
            self.event_repo.record_event("container", cid, EventType.CONTAINER_RUNNING.value)

        except Exception as e:
            self.repo.update_state(cid, ContainerState.FAILED, error_message=str(e))
            raise

    async def stop_container(self, identifier: str, timeout: int = 10) -> None:
        """Stop container: gracefully terminate process, pull output volumes, release runtime."""
        model = self.repo.get_by_id_or_name(identifier)
        if not model:
            raise NotFoundError(f"Container not found: {identifier}")

        cid = model.id
        self.repo.update_state(cid, ContainerState.STOPPING)
        self.event_repo.record_event("container", cid, EventType.CONTAINER_STOP.value)

        runtime = self.runtime_service.repo.get_by_id(model.runtime_id) if model.runtime_id else None
        if runtime:
            provider = self.runtime_service.get_provider(model.context_name)
            transport = provider.open_transport(runtime)

            # Stop worker process
            try:
                await WorkerExecutor.stop_container(transport, cid, timeout=timeout)
            except Exception:
                pass

            # Sync output volumes and rw volumes back to server
            for m in model.mounts:
                mount_mode = VolumeMode(m.mode)
                if mount_mode in (VolumeMode.OUTPUT, VolumeMode.RW):
                    try:
                        vm = VolumeMount(source=m.source, target=m.target, mode=mount_mode)
                        await self.volume_service.sync_mount_from_runtime(vm, transport)
                    except Exception:
                        pass

            # Release runtime to idle pool
            await self.runtime_service.release_runtime(runtime.id, keep_idle=True)

        self.repo.update_state(cid, ContainerState.STOPPED)
        self.event_repo.record_event("container", cid, EventType.CONTAINER_EXIT.value)

    async def restart_container(self, identifier: str) -> None:
        """Restart container (stop + start)."""
        await self.stop_container(identifier)
        await self.start_container(identifier)

    def remove_container(self, identifier: str, force: bool = False) -> bool:
        """Remove container metadata (cbox rm)."""
        model = self.repo.get_by_id_or_name(identifier)
        if not model:
            return False
        if model.state == ContainerState.RUNNING.value and not force:
            raise UsageError(f"Container {identifier} is running. Stop it first or use -f / --force")
        return self.repo.delete_container(identifier)

    def list_containers(self, all_containers: bool = False) -> list[ContainerSummary]:
        return self.repo.list_containers(all_containers=all_containers)

    def inspect_container(self, identifier: str) -> ContainerInspect:
        """Return full inspect structure conforming to plan.md section 32."""
        model = self.repo.get_by_id_or_name(identifier)
        if not model:
            raise NotFoundError(f"Container not found: {identifier}")

        runtime_info = {}
        if model.runtime_id:
            rt = self.runtime_service.repo.get_by_id(model.runtime_id)
            if rt:
                runtime_info = {
                    "Provider": rt.provider,
                    "Session": rt.provider_session,
                    "Account": rt.profile,
                }

        mounts_info = [
            {"Source": m.source, "Destination": m.target, "Mode": m.mode}
            for m in model.mounts
        ]

        state_info = ContainerStateInfo(
            status=ContainerState(model.state),
            exit_code=model.exit_code,
            pid=model.pid,
            error_message=model.error_message,
            started_at=model.started_at,
            finished_at=model.finished_at,
        )

        return ContainerInspect(
            id=model.id,
            name=model.name,
            image=model.image_id,
            state=state_info,
            resource={
                "GPURequested": json.loads(model.gpu_requested),
                "GPUAssigned": model.gpu_assigned,
                "HighMem": model.high_mem,
            },
            runtime=runtime_info,
            mounts=mounts_info,
            command=json.loads(model.command),
            workdir=model.workdir,
            created_at=model.created_at,
        )

    async def get_logs(
        self,
        identifier: str,
        tail: int = 100,
        follow: bool = False,
    ) -> str:
        """Read container logs from remote worker."""
        model = self.repo.get_by_id_or_name(identifier)
        if not model:
            raise NotFoundError(f"Container not found: {identifier}")

        runtime = self.runtime_service.repo.get_by_id(model.runtime_id) if model.runtime_id else None
        if not runtime:
            return "No runtime associated with this container."

        provider = self.runtime_service.get_provider(model.context_name)
        transport = provider.open_transport(runtime)

        log_path = f"/content/.cbox/containers/{model.id}/stdout.log"
        cmd = ["tail", "-n", str(tail), log_path]
        res = await transport.exec(cmd)
        return res.stdout

    async def exec_in_container(
        self,
        identifier: str,
        command: list[str],
    ) -> tuple[int, str, str]:
        """Execute one-off command inside container environment."""
        model = self.repo.get_by_id_or_name(identifier)
        if not model:
            raise NotFoundError(f"Container not found: {identifier}")

        runtime = self.runtime_service.repo.get_by_id(model.runtime_id) if model.runtime_id else None
        if not runtime:
            raise ContainerStartError("Container does not have an active runtime")

        provider = self.runtime_service.get_provider(model.context_name)
        transport = provider.open_transport(runtime)

        exec_cmd = ["/content/.cbox/bin/exec.sh", model.id] + command
        res = await transport.exec(exec_cmd)
        return res.returncode, res.stdout, res.stderr

    async def get_stats(self, identifier: Optional[str] = None) -> list[dict[str, Any]]:
        """Collect resource stats from active container runtimes."""
        containers = self.repo.list_containers(all_containers=False)
        if identifier:
            containers = [c for c in containers if c.id.startswith(identifier) or c.name == identifier]

        results = []
        for c in containers:
            model = self.repo.get_by_id_or_name(c.id)
            if not model or not model.runtime_id:
                continue
            runtime = self.runtime_service.repo.get_by_id(model.runtime_id)
            if not runtime:
                continue

            provider = self.runtime_service.get_provider(model.context_name)
            transport = provider.open_transport(runtime)
            metrics = await WorkerMetrics.get_metrics(transport)
            metrics["name"] = model.name
            metrics["container_id"] = model.id
            metrics["gpu"] = model.gpu_assigned or runtime.accelerator
            results.append(metrics)
        return results
