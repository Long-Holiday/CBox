"""Google Colab compute provider implementation."""

import asyncio
from datetime import datetime, timezone
from typing import Optional

from cbox.domain.enums import RuntimeState
from cbox.domain.runtime import Runtime
from cbox.providers.base import Provider, RuntimeRequest
from cbox.providers.colab.auth import ColabProfile
from cbox.providers.colab.cli import ColabCLI
from cbox.transport.ssh import SSHTransport
from cbox.utils.errors import AllocationError, ProviderError
from cbox.utils.paths import default_paths


class ColabProvider(Provider):
    """Provider for provisioning and managing Google Colab runtimes."""

    name = "colab"

    def __init__(self, profile_name: str = "default"):
        self.profile = ColabProfile(profile_name)
        self.cli = ColabCLI(self.profile)

    async def create_runtime(self, request: RuntimeRequest) -> Runtime:
        """Provision a Colab runtime trying requested accelerators in preference order."""
        last_error: Optional[Exception] = None
        allocated_acc: Optional[str] = None

        for acc in request.accelerators:
            try:
                await self.cli.new_session(
                    session_name=request.session_name,
                    accelerator=acc,
                    high_mem=request.high_mem,
                )
                allocated_acc = acc
                break
            except AllocationError as e:
                last_error = e
                continue
            except Exception as e:
                last_error = e
                break

        if not allocated_acc:
            raise AllocationError(
                f"Could not allocate runtime with accelerators {request.accelerators}: {last_error}"
            )

        runtime = Runtime(
            id=f"rt-{request.session_name}",
            provider="colab",
            provider_session=request.session_name,
            profile=request.profile,
            accelerator=allocated_acc,
            high_mem=request.high_mem,
            state=RuntimeState.PROVISIONING,
            ssh_host=f"cbox-{request.session_name}",
        )

        # Open transport and wait for SSH reachability (up to 45s)
        transport = self.open_transport(runtime)
        transport.write_ssh_config()

        ssh_ready = False
        for _ in range(9):
            await asyncio.sleep(5.0)
            if await transport.ping(timeout=10.0):
                ssh_ready = True
                break

        if ssh_ready:
            runtime.state = RuntimeState.READY
        else:
            runtime.state = RuntimeState.UNREACHABLE

        return runtime

    async def stop_runtime(self, runtime: Runtime) -> None:
        """Terminate Colab session."""
        await self.cli.stop_session(runtime.provider_session)
        runtime.state = RuntimeState.STOPPED

    async def get_runtime_status(self, runtime: Runtime) -> RuntimeState:
        """Check live Colab session status and SSH ping."""
        res = await self.cli.session_status(runtime.provider_session)
        if not res.success or "not found" in res.stdout.lower() or "stopped" in res.stdout.lower():
            return RuntimeState.LOST

        transport = self.open_transport(runtime)
        if await transport.ping(timeout=5.0):
            return RuntimeState.READY
        return RuntimeState.UNREACHABLE

    def open_transport(self, runtime: Runtime) -> SSHTransport:
        """Create SSHTransport with Colab proxy configuration."""
        return SSHTransport(
            session_name=runtime.provider_session,
            colab_config=str(self.profile.sessions_file),
            colab_oauth_config=str(self.profile.oauth_config_file) if self.profile.oauth_config_file.exists() else None,
        )
