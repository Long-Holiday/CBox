"""Data access repositories for database entities."""

import json
from datetime import datetime, timezone
from typing import Optional
from sqlalchemy import select, update, delete
from sqlalchemy.orm import Session

from cbox.domain.enums import ContainerState, RuntimeState, VolumeMode
from cbox.domain.image import ImageManifest, ImageSummary
from cbox.domain.volume import VolumeManifest, VolumeSummary
from cbox.domain.container import ContainerSummary, ContainerInspect, ContainerStateInfo
from cbox.domain.runtime import Runtime
from cbox.domain.context import Context
from cbox.storage.models import (
    ImageModel,
    ImageTagModel,
    VolumeModel,
    ContainerModel,
    ContainerMountModel,
    RuntimeModel,
    RuntimeCacheModel,
    ContextModel,
    EventModel,
)


def _utc_now() -> datetime:
    return datetime.now(timezone.utc).replace(tzinfo=None)


class ImageRepository:
    def __init__(self, session: Session):
        self.session = session

    def save_image(self, manifest: ImageManifest) -> None:
        raw_json = manifest.model_dump_json()
        model = self.session.get(ImageModel, manifest.id)
        if not model:
            model = ImageModel(id=manifest.id, created_at=manifest.created_at, manifest=raw_json)
            self.session.add(model)
        else:
            model.manifest = raw_json

        # Update tags
        self.session.execute(delete(ImageTagModel).where(ImageTagModel.image_id == manifest.id))
        for tag_str in manifest.tags:
            repo, tag = tag_str.split(":", 1) if ":" in tag_str else (tag_str, "latest")
            self.session.add(ImageTagModel(repository=repo, tag=tag, image_id=manifest.id))
        self.session.flush()

    def get_by_id_or_tag(self, identifier: str) -> Optional[ImageManifest]:
        # Try by ID directly (or prefix)
        stmt = select(ImageModel).where(ImageModel.id.startswith(identifier))
        model = self.session.execute(stmt).scalars().first()
        if model:
            return ImageManifest.model_validate_json(model.manifest)

        # Try by tag
        repo, tag = identifier.split(":", 1) if ":" in identifier else (identifier, "latest")
        tag_stmt = select(ImageTagModel).where(ImageTagModel.repository == repo, ImageTagModel.tag == tag)
        tag_model = self.session.execute(tag_stmt).scalars().first()
        if tag_model:
            img = self.session.get(ImageModel, tag_model.image_id)
            if img:
                return ImageManifest.model_validate_json(img.manifest)
        return None

    def list_images(self) -> list[ImageSummary]:
        stmt = select(ImageModel).order_by(ImageModel.created_at.desc())
        models = self.session.execute(stmt).scalars().all()
        summaries: list[ImageSummary] = []
        for m in models:
            tags = [f"{t.repository}:{t.tag}" for t in m.tags]
            summaries.append(
                ImageSummary(
                    id=m.id,
                    tags=tags if tags else ["<none>:<none>"],
                    created_at=m.created_at,
                )
            )
        return summaries

    def delete_image(self, identifier: str) -> bool:
        manifest = self.get_by_id_or_tag(identifier)
        if not manifest:
            return False
        model = self.session.get(ImageModel, manifest.id)
        if model:
            self.session.delete(model)
            self.session.flush()
            return True
        return False


class VolumeRepository:
    def __init__(self, session: Session):
        self.session = session

    def save_volume(self, volume: VolumeManifest) -> None:
        model = self.session.get(VolumeModel, volume.id)
        if not model:
            model = VolumeModel(
                id=volume.id,
                name=volume.name,
                source=volume.source,
                hash=volume.hash,
                mode=volume.mode.value,
                immutable=volume.immutable,
                version=volume.version,
                created_at=volume.created_at,
            )
            self.session.add(model)
        else:
            model.hash = volume.hash
            model.version = volume.version
            model.mode = volume.mode.value
        self.session.flush()

    def get_by_name_or_id(self, identifier: str) -> Optional[VolumeManifest]:
        stmt = select(VolumeModel).where(
            (VolumeModel.id == identifier) | (VolumeModel.name == identifier)
        )
        model = self.session.execute(stmt).scalars().first()
        if not model:
            return None
        return VolumeManifest(
            id=model.id,
            name=model.name,
            source=model.source,
            hash=model.hash,
            mode=VolumeMode(model.mode),
            immutable=model.immutable,
            version=model.version,
            created_at=model.created_at,
        )

    def list_volumes(self) -> list[VolumeSummary]:
        stmt = select(VolumeModel).order_by(VolumeModel.created_at.desc())
        models = self.session.execute(stmt).scalars().all()
        return [
            VolumeSummary(
                name=m.name,
                source=m.source,
                mode=VolumeMode(m.mode),
                immutable=m.immutable,
                version=m.version,
                created_at=m.created_at,
            )
            for m in models
        ]

    def delete_volume(self, identifier: str) -> bool:
        vol = self.get_by_name_or_id(identifier)
        if not vol:
            return False
        model = self.session.get(VolumeModel, vol.id)
        if model:
            self.session.delete(model)
            self.session.flush()
            return True
        return False


class ContainerRepository:
    def __init__(self, session: Session):
        self.session = session

    def save_container(
        self,
        id: str,
        name: str,
        image_id: str,
        command: list[str],
        workdir: str,
        gpu_requested: list[str],
        restart_policy: str,
        resume_command: Optional[list[str]] = None,
        high_mem: bool = False,
        auto_remove: bool = False,
        context_name: Optional[str] = None,
        mounts: Optional[list[dict]] = None,
    ) -> ContainerModel:
        model = self.session.get(ContainerModel, id)
        if not model:
            model = ContainerModel(
                id=id,
                name=name,
                image_id=image_id,
                command=json.dumps(command),
                workdir=workdir,
                gpu_requested=json.dumps(gpu_requested),
                restart_policy=restart_policy,
                resume_command=json.dumps(resume_command) if resume_command else None,
                high_mem=high_mem,
                auto_remove=auto_remove,
                context_name=context_name,
                state=ContainerState.CREATED.value,
            )
            self.session.add(model)
            self.session.flush()

        if mounts is not None:
            self.session.execute(delete(ContainerMountModel).where(ContainerMountModel.container_id == id))
            for m in mounts:
                self.session.add(
                    ContainerMountModel(
                        container_id=id,
                        volume_id=m.get("volume_id", m.get("source", "")),
                        source=m.get("source", ""),
                        target=m.get("target", ""),
                        mode=m.get("mode", "ro"),
                    )
                )
            self.session.flush()
        return model

    def get_by_id_or_name(self, identifier: str) -> Optional[ContainerModel]:
        stmt = select(ContainerModel).where(
            (ContainerModel.id.startswith(identifier)) | (ContainerModel.name == identifier)
        )
        return self.session.execute(stmt).scalars().first()

    def list_containers(self, all_containers: bool = False) -> list[ContainerSummary]:
        stmt = select(ContainerModel).order_by(ContainerModel.created_at.desc())
        models = self.session.execute(stmt).scalars().all()
        results: list[ContainerSummary] = []
        for m in models:
            state_enum = ContainerState(m.state)
            if not all_containers and state_enum in [ContainerState.STOPPED, ContainerState.EXITED, ContainerState.REMOVED]:
                continue
            
            # format status nicely, e.g. Up 2h or Exited (0)
            status_str = state_enum.value.capitalize()
            if state_enum == ContainerState.RUNNING and m.started_at:
                delta = _utc_now() - m.started_at
                status_str = f"Up {int(delta.total_seconds() // 60)}m"
            elif state_enum == ContainerState.EXITED and m.exit_code is not None:
                status_str = f"Exited ({m.exit_code})"

            results.append(
                ContainerSummary(
                    id=m.id,
                    name=m.name,
                    image=m.image_id,
                    gpu=m.gpu_assigned or (json.loads(m.gpu_requested)[0] if m.gpu_requested else "None"),
                    status=status_str,
                    state=state_enum,
                    created_at=m.created_at,
                )
            )
        return results

    def update_state(
        self,
        id: str,
        state: ContainerState,
        exit_code: Optional[int] = None,
        pid: Optional[int] = None,
        error_message: Optional[str] = None,
        runtime_id: Optional[str] = None,
        gpu_assigned: Optional[str] = None,
    ) -> None:
        model = self.session.get(ContainerModel, id)
        if model:
            model.state = state.value
            if exit_code is not None:
                model.exit_code = exit_code
            if pid is not None:
                model.pid = pid
            if error_message is not None:
                model.error_message = error_message
            if runtime_id is not None:
                model.runtime_id = runtime_id
            if gpu_assigned is not None:
                model.gpu_assigned = gpu_assigned
            if state == ContainerState.RUNNING and not model.started_at:
                model.started_at = _utc_now()
            elif state in (ContainerState.STOPPED, ContainerState.EXITED, ContainerState.FAILED):
                model.finished_at = _utc_now()
            self.session.flush()

    def delete_container(self, identifier: str) -> bool:
        model = self.get_by_id_or_name(identifier)
        if not model:
            return False
        self.session.delete(model)
        self.session.flush()
        return True


class RuntimeRepository:
    def __init__(self, session: Session):
        self.session = session

    def save_runtime(self, runtime: Runtime) -> None:
        model = self.session.get(RuntimeModel, runtime.id)
        if not model:
            model = RuntimeModel(
                id=runtime.id,
                provider=runtime.provider,
                provider_session=runtime.provider_session,
                profile=runtime.profile,
                accelerator=runtime.accelerator,
                high_mem=runtime.high_mem,
                state=runtime.state.value,
                ssh_host=runtime.ssh_host,
                created_at=runtime.created_at,
                last_seen=runtime.last_seen,
                idle_since=runtime.idle_since,
            )
            self.session.add(model)
        else:
            model.state = runtime.state.value
            model.last_seen = runtime.last_seen
            model.idle_since = runtime.idle_since
            model.ssh_host = runtime.ssh_host
        self.session.flush()

    def get_by_id(self, id: str) -> Optional[Runtime]:
        model = self.session.get(RuntimeModel, id)
        if not model:
            return None
        return Runtime(
            id=model.id,
            provider=model.provider,
            provider_session=model.provider_session,
            profile=model.profile,
            accelerator=model.accelerator,
            high_mem=model.high_mem,
            state=RuntimeState(model.state),
            ssh_host=model.ssh_host,
            created_at=model.created_at,
            last_seen=model.last_seen,
            idle_since=model.idle_since,
        )

    def find_idle_runtime(self, profile: str, accelerator: str) -> Optional[Runtime]:
        stmt = select(RuntimeModel).where(
            RuntimeModel.profile == profile,
            RuntimeModel.accelerator == accelerator,
            RuntimeModel.state == RuntimeState.IDLE.value,
        )
        model = self.session.execute(stmt).scalars().first()
        if not model:
            return None
        return self.get_by_id(model.id)

    def list_runtimes(self) -> list[Runtime]:
        stmt = select(RuntimeModel).order_by(RuntimeModel.created_at.desc())
        models = self.session.execute(stmt).scalars().all()
        return [
            Runtime(
                id=m.id,
                provider=m.provider,
                provider_session=m.provider_session,
                profile=m.profile,
                accelerator=m.accelerator,
                high_mem=m.high_mem,
                state=RuntimeState(m.state),
                ssh_host=m.ssh_host,
                created_at=m.created_at,
                last_seen=m.last_seen,
                idle_since=m.idle_since,
            )
            for m in models
        ]

    def add_cache_entry(self, runtime_id: str, object_type: str, object_id: str, hash_val: str) -> None:
        self.session.add(
            RuntimeCacheModel(
                runtime_id=runtime_id,
                object_type=object_type,
                object_id=object_id,
                hash=hash_val,
            )
        )
        self.session.flush()

    def has_cache(self, runtime_id: str, object_type: str, object_id: str, hash_val: str) -> bool:
        stmt = select(RuntimeCacheModel).where(
            RuntimeCacheModel.runtime_id == runtime_id,
            RuntimeCacheModel.object_type == object_type,
            RuntimeCacheModel.object_id == object_id,
            RuntimeCacheModel.hash == hash_val,
        )
        return self.session.execute(stmt).scalars().first() is not None


class ContextRepository:
    def __init__(self, session: Session):
        self.session = session

    def get_active(self) -> Optional[Context]:
        stmt = select(ContextModel).where(ContextModel.active == True)
        model = self.session.execute(stmt).scalars().first()
        if not model:
            return None
        return Context(
            name=model.name,
            provider=model.provider,
            profile=model.profile,
            active=model.active,
            created_at=model.created_at,
        )

    def set_active(self, name: str) -> bool:
        model = self.session.get(ContextModel, name)
        if not model:
            return False
        # Deactivate all others
        self.session.execute(update(ContextModel).values(active=False))
        model.active = True
        self.session.flush()
        return True

    def save_context(self, context: Context) -> None:
        model = self.session.get(ContextModel, context.name)
        if not model:
            model = ContextModel(
                name=context.name,
                provider=context.provider,
                profile=context.profile,
                active=context.active,
                created_at=context.created_at,
            )
            self.session.add(model)
        else:
            model.provider = context.provider
            model.profile = context.profile
            model.active = context.active
        self.session.flush()

    def list_contexts(self) -> list[Context]:
        stmt = select(ContextModel).order_by(ContextModel.name)
        models = self.session.execute(stmt).scalars().all()
        return [
            Context(
                name=m.name,
                provider=m.provider,
                profile=m.profile,
                active=m.active,
                created_at=m.created_at,
            )
            for m in models
        ]

    def delete_context(self, name: str) -> bool:
        model = self.session.get(ContextModel, name)
        if not model:
            return False
        self.session.delete(model)
        self.session.flush()
        return True


class EventRepository:
    def __init__(self, session: Session):
        self.session = session

    def record_event(self, object_type: str, object_id: str, event: str, payload: Optional[dict] = None) -> None:
        self.session.add(
            EventModel(
                object_type=object_type,
                object_id=object_id,
                event=event,
                payload=json.dumps(payload) if payload else None,
            )
        )
        self.session.flush()
