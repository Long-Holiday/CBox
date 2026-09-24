"""Mock compute provider for testing and offline development."""

import asyncio
import os
import shutil
from pathlib import Path
from typing import Optional

from cbox.domain.enums import RuntimeState
from cbox.domain.runtime import Runtime
from cbox.providers.base import Provider, RuntimeRequest
from cbox.transport.ssh import SSHTransport
from cbox.utils.paths import default_paths
from cbox.utils.shell import ProcessResult, run_command_async


class MockTransport(SSHTransport):
    """Local simulation of SSH transport."""

    def __init__(self, session_name: str, root_dir: Path):
        super().__init__(session_name=session_name)
        self.root_dir = root_dir
        self.mock_content_dir = root_dir / "content"

    def write_ssh_config(self) -> Path:
        return self.ssh_config_file

    async def ping(self, timeout: float = 5.0) -> bool:
        return True

    async def exec(
        self,
        command: list[str],
        timeout: Optional[float] = None,
        input_text: Optional[str] = None,
    ) -> ProcessResult:
        """Execute command locally, mapping /content to mock_content_dir."""
        mock_str = str(self.mock_content_dir)
        adjusted_cmd = []
        is_apt = False
        is_pip = False
        for arg in command:
            if "apt-get" in arg:
                is_apt = True
            if "pip install" in arg:
                is_pip = True
            adjusted_cmd.append(arg.replace("/content", mock_str))

        if is_apt:
            return ProcessResult(0, "Mock apt-get success\n", "")
        if is_pip:
            return ProcessResult(0, "Mock pip install success\n", "")

        return await run_command_async(
            adjusted_cmd,
            env={"PATH": os.environ["PATH"]},
            timeout=timeout,
            input_text=input_text,
        )

    async def upload_text(self, content: str, remote_path: str, mode: str = "0644") -> None:
        local_target = Path(remote_path.replace("/content", str(self.mock_content_dir)))
        local_target.parent.mkdir(parents=True, exist_ok=True, mode=0o755)
        local_target.write_text(content, encoding="utf-8")
        os.chmod(local_target, int(mode, 8))


class MockProvider(Provider):
    """Mock Provider simulating Colab runtime lifecycle in local scratch directory."""

    name = "mock"

    def __init__(self, base_dir: Optional[Path] = None):
        self.base_dir = base_dir or (default_paths.state_dir / "mock_runtimes")
        self.base_dir.mkdir(parents=True, exist_ok=True, mode=0o700)
        self.runtimes: dict[str, Runtime] = {}

    async def create_runtime(self, request: RuntimeRequest) -> Runtime:
        runtime_id = f"rt-mock-{request.session_name}"
        rt_dir = self.base_dir / request.session_name
        rt_dir.mkdir(parents=True, exist_ok=True, mode=0o700)
        (rt_dir / "content" / ".cbox").mkdir(parents=True, exist_ok=True, mode=0o755)

        acc = request.accelerators[0] if request.accelerators else "T4"
        runtime = Runtime(
            id=runtime_id,
            provider="mock",
            provider_session=request.session_name,
            profile=request.profile,
            accelerator=acc,
            high_mem=request.high_mem,
            state=RuntimeState.READY,
            ssh_host=f"mock-{request.session_name}",
        )
        self.runtimes[runtime_id] = runtime
        return runtime

    async def stop_runtime(self, runtime: Runtime) -> None:
        if runtime.id in self.runtimes:
            runtime.state = RuntimeState.STOPPED

    async def get_runtime_status(self, runtime: Runtime) -> RuntimeState:
        if runtime.state in (RuntimeState.LOST, RuntimeState.STOPPED):
            return runtime.state
        return self.runtimes.get(runtime.id, runtime).state

    def open_transport(self, runtime: Runtime) -> SSHTransport:
        rt_dir = self.base_dir / runtime.provider_session
        return MockTransport(runtime.provider_session, rt_dir)
