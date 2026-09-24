"""OpenSSH transport with multiplexing and Colab ProxyCommand support."""

import asyncio
import os
import shlex
from pathlib import Path
from typing import Callable, Optional

from cbox.utils.errors import TransportError
from cbox.utils.paths import default_paths
from cbox.utils.shell import ProcessResult, run_command_async, stream_command_async


class SSHTransport:
    """Manages SSH connections, commands, and file transfers to a compute runtime."""

    def __init__(
        self,
        session_name: str,
        identity_file: Optional[Path] = None,
        colab_config: Optional[str] = None,
        colab_oauth_config: Optional[str] = None,
        ssh_config_file: Optional[Path] = None,
    ):
        self.session_name = session_name
        self.host_alias = f"cbox-{session_name}"
        self.identity_file = identity_file or default_paths.worker_ssh_key
        self.colab_config = colab_config
        self.colab_oauth_config = colab_oauth_config
        self.ssh_config_file = ssh_config_file or (default_paths.state_dir / f"ssh_config_{session_name}")

    @classmethod
    def ensure_worker_key(cls, key_path: Optional[Path] = None) -> Path:
        """Ensure dedicated Ed25519 SSH key exists for remote worker authentication."""
        path = key_path or default_paths.worker_ssh_key
        path.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
        if not path.exists():
            cmd = ["ssh-keygen", "-t", "ed25519", "-N", "", "-f", str(path), "-C", "cbox-worker"]
            proc = os.system(" ".join(cmd) + " >/dev/null 2>&1")
            if proc != 0:
                raise TransportError("Failed to generate worker SSH key via ssh-keygen")
        # Ensure permissions
        os.chmod(path, 0o600)
        pub_path = Path(f"{path}.pub")
        if pub_path.exists():
            os.chmod(pub_path, 0o644)
        return path

    def write_ssh_config(self) -> Path:
        """Generate OpenSSH configuration file with ProxyCommand and multiplexing."""
        self.ensure_worker_key(self.identity_file)
        control_dir = default_paths.ssh_control_dir
        control_dir.mkdir(parents=True, exist_ok=True, mode=0o700)

        # Build proxy command
        proxy_cmd_parts = ["colab"]
        if self.colab_oauth_config:
            proxy_cmd_parts.extend(["--client-oauth-config", shlex.quote(self.colab_oauth_config)])
        if self.colab_config:
            proxy_cmd_parts.extend(["--config", shlex.quote(self.colab_config)])
        proxy_cmd_parts.extend([
            "ssh",
            "--proxy-mode",
            "-s",
            shlex.quote(self.session_name),
            "-i",
            shlex.quote(str(self.identity_file)),
        ])
        proxy_command = " ".join(proxy_cmd_parts)

        config_content = f"""Host {self.host_alias}
    HostName localhost
    User root
    IdentityFile {self.identity_file}
    IdentitiesOnly yes
    StrictHostKeyChecking no
    UserKnownHostsFile /dev/null
    LogLevel ERROR
    ControlMaster auto
    ControlPersist 600
    ControlPath {control_dir}/%C
    ServerAliveInterval 30
    ServerAliveCountMax 3
    ProxyCommand {proxy_command}
"""
        self.ssh_config_file.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
        self.ssh_config_file.write_text(config_content, encoding="utf-8")
        os.chmod(self.ssh_config_file, 0o600)
        return self.ssh_config_file

    def get_ssh_args(self) -> list[str]:
        """Return base arguments for invoking ssh with the generated config."""
        if not self.ssh_config_file.exists():
            self.write_ssh_config()
        return ["ssh", "-F", str(self.ssh_config_file), self.host_alias]

    async def ping(self, timeout: float = 15.0) -> bool:
        """Check if SSH connection can be established."""
        try:
            res = await self.exec(["echo", "CBOX_HEALTHY"], timeout=timeout)
            return res.success and "CBOX_HEALTHY" in res.stdout
        except Exception:
            return False

    async def exec(
        self,
        command: list[str],
        timeout: Optional[float] = None,
        input_text: Optional[str] = None,
    ) -> ProcessResult:
        """Execute a remote command over SSH."""
        cmd = self.get_ssh_args() + ["--"] + command
        return await run_command_async(cmd, timeout=timeout, input_text=input_text)

    async def stream_exec(
        self,
        command: list[str],
        on_stdout: Optional[Callable[[str], None]] = None,
        on_stderr: Optional[Callable[[str], None]] = None,
    ) -> int:
        """Execute a remote command and stream outputs asynchronously."""
        cmd = self.get_ssh_args() + ["--"] + command
        return await stream_command_async(cmd, on_stdout=on_stdout, on_stderr=on_stderr)

    async def upload_text(self, content: str, remote_path: str, mode: str = "0644") -> None:
        """Upload string content to a remote file path."""
        # Use python or base64 or direct cat on remote
        escaped_path = shlex.quote(remote_path)
        cmd = ["sh", "-c", f"cat > {escaped_path} && chmod {mode} {escaped_path}"]
        res = await self.exec(cmd, input_text=content)
        if not res.success:
            raise TransportError(f"Failed to upload text to {remote_path}: {res.stderr}")
