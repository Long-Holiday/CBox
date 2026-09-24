"""Google Colab CLI subprocess wrapper conforming to CLI boundary."""

from typing import Optional
from cbox.providers.colab.auth import ColabProfile
from cbox.utils.errors import AllocationError, AuthenticationError, ProviderError
from cbox.utils.shell import ProcessResult, run_command_async


class ColabCLI:
    """Invokes google-colab-cli via subprocess with profile isolation."""

    def __init__(self, profile: Optional[ColabProfile] = None):
        self.profile = profile or ColabProfile("default")

    def _build_cmd(self, subcmd: list[str]) -> list[str]:
        cmd = ["colab"]
        cmd.extend(self.profile.get_colab_cli_flags())
        cmd.extend(subcmd)
        return cmd

    async def new_session(
        self,
        session_name: str,
        accelerator: Optional[str] = "T4",
        high_mem: bool = False,
        timeout: float = 120.0,
    ) -> ProcessResult:
        """Execute `colab new -s <session> --gpu <accelerator>`."""
        args = ["new", "-s", session_name]
        if accelerator and accelerator.upper() != "CPU":
            args.extend(["--gpu", accelerator])
        if high_mem:
            args.append("--high-mem")

        cmd = self._build_cmd(args)
        res = await run_command_async(cmd, env=self.profile.get_env(), timeout=timeout)
        if not res.success:
            err_text = (res.stderr + " " + res.stdout).lower()
            if "gpu" in err_text or "resource" in err_text or "quota" in err_text or "unavailable" in err_text:
                raise AllocationError(f"Failed to allocate GPU {accelerator}: {res.stderr or res.stdout}")
            if "auth" in err_text or "token" in err_text or "login" in err_text:
                raise AuthenticationError(f"Colab authentication required: {res.stderr or res.stdout}")
            raise ProviderError(f"Colab new failed: {res.stderr or res.stdout}")
        return res

    async def stop_session(self, session_name: str, timeout: float = 30.0) -> ProcessResult:
        """Execute `colab stop -s <session>`."""
        cmd = self._build_cmd(["stop", "-s", session_name])
        return await run_command_async(cmd, env=self.profile.get_env(), timeout=timeout)

    async def list_sessions(self, timeout: float = 30.0) -> ProcessResult:
        """Execute `colab sessions`."""
        cmd = self._build_cmd(["sessions"])
        return await run_command_async(cmd, env=self.profile.get_env(), timeout=timeout)

    async def session_status(self, session_name: str, timeout: float = 30.0) -> ProcessResult:
        """Execute `colab status -s <session>`."""
        cmd = self._build_cmd(["status", "-s", session_name])
        return await run_command_async(cmd, env=self.profile.get_env(), timeout=timeout)
