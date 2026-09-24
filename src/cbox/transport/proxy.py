"""Proxy management and high-volume small-file tar streaming."""

import asyncio
import os
from pathlib import Path
from typing import Optional

from cbox.domain.enums import TransferMode
from cbox.transport.ssh import SSHTransport
from cbox.utils.errors import VolumeSyncError


class TarStreamTransport:
    """Streams directories using tar piped over SSH for high file count directories."""

    @classmethod
    async def push_tar_stream(
        cls,
        local_dir: Path | str,
        remote_dir: str,
        ssh_transport: SSHTransport,
    ) -> None:
        """Stream a local directory as a tar archive over SSH directly into remote extraction."""
        local_path = Path(local_dir).resolve()
        if not local_path.is_dir():
            raise VolumeSyncError(f"{local_dir} is not a valid directory for tar stream")

        # Mock transport fallback for testing
        if hasattr(ssh_transport, "mock_content_dir"):
            real_dest = Path(remote_dir.replace("/content", str(getattr(ssh_transport, "mock_content_dir"))))
            real_dest.mkdir(parents=True, exist_ok=True, mode=0o755)
            # Local tar pipeline: tar -C <src> -cf - . | tar -C <dst> -xf -
            tar_src = await asyncio.create_subprocess_exec(
                "tar", "-C", str(local_path), "-cf", "-", ".",
                stdout=asyncio.subprocess.PIPE,
            )
            tar_dst = await asyncio.create_subprocess_exec(
                "tar", "-C", str(real_dest), "-xf", "-",
                stdin=tar_src.stdout,
            )
            await tar_src.wait()
            await tar_dst.wait()
            return

        ssh_args = ssh_transport.get_ssh_args()
        remote_cmd = f"mkdir -p {remote_dir} && tar -C {remote_dir} -xf -"

        # Pipeline: tar -C <local_path> -cf - . | ssh ... "<remote_cmd>"
        tar_proc = await asyncio.create_subprocess_exec(
            "tar",
            "-C",
            str(local_path),
            "-cf",
            "-",
            ".",
            stdout=asyncio.subprocess.PIPE,
            stderr=asyncio.subprocess.PIPE,
        )

        ssh_proc = await asyncio.create_subprocess_exec(
            *ssh_args,
            "--",
            "sh",
            "-c",
            remote_cmd,
            stdin=tar_proc.stdout,
            stdout=asyncio.subprocess.PIPE,
            stderr=asyncio.subprocess.PIPE,
        )

        _, ssh_err = await ssh_proc.communicate()
        tar_ret = await tar_proc.wait()
        ssh_ret = await ssh_proc.wait()

        if tar_ret != 0 or ssh_ret != 0:
            err_msg = ssh_err.decode("utf-8", errors="replace") if ssh_err else "Tar stream error"
            raise VolumeSyncError(f"Tar stream transfer failed: {err_msg}")


def decide_transfer_mode(
    dir_path: Path | str,
    configured_mode: str = "auto",
    threshold_files: int = 5000,
) -> TransferMode:
    """Automatically determine whether to use tar-stream or rsync."""
    if configured_mode == "tar-stream":
        return TransferMode.TAR_STREAM
    if configured_mode == "rsync":
        return TransferMode.RSYNC

    # Auto mode: sample file count
    path = Path(dir_path)
    if not path.is_dir():
        return TransferMode.RSYNC

    file_count = 0
    try:
        for _, _, files in os.walk(path):
            file_count += len(files)
            if file_count > threshold_files:
                return TransferMode.TAR_STREAM
    except OSError:
        pass

    return TransferMode.RSYNC
