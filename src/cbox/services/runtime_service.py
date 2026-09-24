"""Runtime lifecycle management, pooling, and provider coordination."""

import uuid
from datetime import datetime, timezone
from typing import Optional
from sqlalchemy.orm import Session

from cbox.domain.container import ContainerSpec
from cbox.domain.enums import RuntimeState
from cbox.domain.runtime import Runtime
from cbox.providers.base import Provider, RuntimeRequest
from cbox.providers.colab.provider import ColabProvider
from cbox.providers.mock import MockProvider
from cbox.services.scheduler import Scheduler
from cbox.storage.repositories import ContextRepository, RuntimeRepository
from cbox.transport.ssh import SSHTransport
from cbox.worker.bootstrap import bootstrap_remote_worker


class RuntimeService:
    """Coordinates compute runtime allocation, scheduler placement, and idle pools."""

    def __init__(self, session: Session, provider: Optional[Provider] = None):
        self.session = session
        self.repo = RuntimeRepository(session)
        self.context_repo = ContextRepository(session)
        self.scheduler = Scheduler(session)
        self._custom_provider = provider

    def get_provider(self, context_name: Optional[str] = None) -> Provider:
        """Resolve provider from active context or custom injected provider."""
        if self._custom_provider:
            return self._custom_provider

        active_ctx = None
        if context_name:
            all_ctxs = self.context_repo.list_contexts()
            for c in all_ctxs:
                if c.name == context_name:
                    active_ctx = c
                    break
        if not active_ctx:
            active_ctx = self.context_repo.get_active()

        provider_name = active_ctx.provider if active_ctx else "colab"
        profile_name = active_ctx.profile if active_ctx else "default"

        if provider_name == "mock":
            return MockProvider()
        return ColabProvider(profile_name=profile_name)

    async def acquire_runtime(
        self,
        spec: ContainerSpec,
        image_id: str,
        container_id: str,
    ) -> Runtime:
        """Obtain a runtime: reuses an idle runtime or allocates a new one."""
        provider = self.get_provider(spec.context)

        # 1. Try to find matching idle runtime from pool
        candidate = self.scheduler.select_best_runtime(spec, image_id)
        if candidate:
            candidate.state = RuntimeState.BUSY
            candidate.idle_since = None
            candidate.last_seen = datetime.now(timezone.utc)
            self.repo.save_runtime(candidate)
            return candidate

        # 2. Provision new runtime
        session_name = f"cbox-{container_id[:8]}"
        req = RuntimeRequest(
            session_name=session_name,
            accelerators=spec.gpu,
            high_mem=spec.high_mem,
            profile=spec.context or "default",
        )

        runtime = await provider.create_runtime(req)
        runtime.state = RuntimeState.BUSY

        # Bootstrap remote worker environment
        transport = provider.open_transport(runtime)
        await bootstrap_remote_worker(transport)

        self.repo.save_runtime(runtime)
        return runtime

    async def release_runtime(
        self,
        runtime_id: str,
        keep_idle: bool = True,
    ) -> None:
        """Release runtime back to idle pool or terminate it."""
        runtime = self.repo.get_by_id(runtime_id)
        if not runtime:
            return

        provider = self.get_provider()
        if keep_idle:
            runtime.state = RuntimeState.IDLE
            runtime.idle_since = datetime.now(timezone.utc).replace(tzinfo=None)
            self.repo.save_runtime(runtime)
        else:
            await provider.stop_runtime(runtime)
            runtime.state = RuntimeState.STOPPED
            self.repo.save_runtime(runtime)

    async def cleanup_expired_runtimes(self, idle_timeout: int = 1800) -> list[str]:
        """Terminate and clean up runtimes that exceeded idle timeout."""
        terminated = []
        now = datetime.now(timezone.utc).replace(tzinfo=None)
        for r in self.repo.list_runtimes():
            if r.state == RuntimeState.IDLE and r.idle_since:
                idle_seconds = (now - r.idle_since).total_seconds()
                if idle_seconds > idle_timeout:
                    await self.release_runtime(r.id, keep_idle=False)
                    terminated.append(r.id)
        return terminated
