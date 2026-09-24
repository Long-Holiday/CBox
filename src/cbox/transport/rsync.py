"""Rsync transport for remote file and volume synchronization."""

import os
from pathlib import Path
from typing import Optional

from cbox.transport.ssh import SSHTransport
from cbox.utils.errors import VolumeSyncError
from cbox.utils.shell import run_command_async


class RsyncTransport:
    """Manages file synchronization over SSH using rsync."""

    @staticmethod
    def _get_rsync_ssh_cmd(ssh_transport: SSHTransport) -> str:
        """Construct SSH command string for rsync -e flag."""
        ssh_transport.write_ssh_config()
        return f"ssh -F {ssh_transport.ssh_config_file}"

    @classmethod
    async def push(
        cls,
        local_path: Path | str,
        remote_path: str,
        ssh_transport: SSHTransport,
        delete: bool = False,
        excludes: Optional[list[str]] = None,
    ) -> None:
        """Push local directory or file to remote runtime."""
        loc_str = str(local_path)
        if Path(loc_str).is_dir() and not loc_str.endswith("/"):
            loc_str += "/"

        # Mock mode fallback for testing
        if hasattr(ssh_transport, "mock_content_dir"):
            real_dest = remote_path.replace("/content", str(getattr(ssh_transport, "mock_content_dir")))
            Path(real_dest).mkdir(parents=True, exist_ok=True, mode=0o755)
            cmd = ["rsync", "-avz", "--partial"]
            if delete:
                cmd.append("--delete")
            if excludes:
                for exc in excludes:
                    cmd.extend(["--exclude", exc])
            cmd.extend([loc_str, real_dest])
            res = await run_command_async(cmd)
            if not res.success:
                raise VolumeSyncError(f"Mock rsync push failed: {res.stderr}")
            return

        ssh_cmd = cls._get_rsync_ssh_cmd(ssh_transport)
        cmd = [
            "rsync",
            "-avz",
            "--partial",
            "-e",
            ssh_cmd,
        ]
        if delete:
            cmd.append("--delete")
        if excludes:
            for exc in excludes:
                cmd.extend(["--exclude", exc])

        remote_dest = f"{ssh_transport.host_alias}:{remote_path}"
        cmd.extend([loc_str, remote_dest])

        res = await run_command_async(cmd)
        if not res.success:
            raise VolumeSyncError(f"Rsync push failed from {loc_str} to {remote_dest}: {res.stderr}")

    @classmethod
    async def pull(
        cls,
        remote_path: str,
        local_path: Path | str,
        ssh_transport: SSHTransport,
        delete: bool = False,
        excludes: Optional[list[str]] = None,
    ) -> None:
        """Pull remote directory or file from remote runtime to local host."""
        Path(local_path).parent.mkdir(parents=True, exist_ok=True, mode=0o755)
        rem_str = remote_path
        if not rem_str.endswith("/"):
            rem_str += "/"
        loc_str = str(local_path)
        if not loc_str.endswith("/"):
            loc_str += "/"

        # Mock mode fallback for testing
        if hasattr(ssh_transport, "mock_content_dir"):
            real_src = remote_path.replace("/content", str(getattr(ssh_transport, "mock_content_dir")))
            if Path(real_src).is_dir() and not real_src.endswith("/"):
                real_src += "/"
            cmd = ["rsync", "-avz", "--partial"]
            if delete:
                cmd.append("--delete")
            if excludes:
                for exc in excludes:
                    cmd.extend(["--exclude", exc])
            cmd.extend([real_src, loc_str])
            res = await run_command_async(cmd)
            if not res.success:
                raise VolumeSyncError(f"Mock rsync pull failed: {res.stderr}")
            return

        ssh_cmd = cls._get_rsync_ssh_cmd(ssh_transport)
        cmd = [
            "rsync",
            "-avz",
            "--partial",
            "-e",
            ssh_cmd,
        ]
        if delete:
            cmd.append("--delete")
        if excludes:
            for exc in excludes:
                cmd.extend(["--exclude", exc])

        remote_src = f"{ssh_transport.host_alias}:{rem_str}"
        cmd.extend([remote_src, loc_str])

        res = await run_command_async(cmd)
        if not res.success:
            raise VolumeSyncError(f"Rsync pull failed from {remote_src} to {loc_str}: {res.stderr}")
