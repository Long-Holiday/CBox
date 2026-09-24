"""Container execution and tmux session orchestration on remote worker."""

import json
import shlex
from typing import Optional

from cbox.domain.container import ContainerSpec
from cbox.transport.ssh import SSHTransport
from cbox.utils.errors import ContainerStartError


class WorkerExecutor:
    """Manages remote container processes inside tmux sessions."""

    @staticmethod
    async def prepare_container(
        transport: SSHTransport,
        container_id: str,
        spec: ContainerSpec,
        secrets_dict: Optional[dict[str, str]] = None,
    ) -> None:
        """Create remote container directory and upload configuration and startup scripts."""
        remote_cbox = "/content/.cbox"
        container_dir = f"{remote_cbox}/containers/{container_id}"

        # Create container directory structure
        await transport.exec(["mkdir", "-p", container_dir])

        # Write workdir
        await transport.upload_text(spec.workdir, f"{container_dir}/workdir")

        # Write environment variables
        env_lines = ["#!/usr/bin/env bash"]
        for k, v in spec.env.items():
            env_lines.append(f"export {k}={shlex.quote(v)}")
        if secrets_dict:
            for k, v in secrets_dict.items():
                env_lines.append(f"export {k}={shlex.quote(v)}")
        # Add basic paths
        env_lines.append("export PATH=/usr/local/bin:/usr/bin:/bin:$PATH")
        await transport.upload_text("\n".join(env_lines) + "\n", f"{container_dir}/env.sh", mode="0700")

        # Write command script
        cmd_str = " ".join(shlex.quote(arg) for arg in spec.command)
        cmd_script = f"""#!/usr/bin/env bash
set -e
cd "{spec.workdir}"
exec {cmd_str}
"""
        await transport.upload_text(cmd_script, f"{container_dir}/cmd.sh", mode="0755")

        # Create symlinks for mounted volumes
        for mount in spec.mounts:
            remote_vol_path = f"{remote_cbox}/volumes/{mount.source}"
            target_path = mount.target
            link_cmd = [
                "sh",
                "-c",
                f"mkdir -p $(dirname {shlex.quote(target_path)}) && rm -rf {shlex.quote(target_path)} && ln -sf {shlex.quote(remote_vol_path)} {shlex.quote(target_path)}"
            ]
            await transport.exec(link_cmd)

    @staticmethod
    async def start_container(transport: SSHTransport, container_id: str) -> None:
        """Start container command in a detached tmux session."""
        session_name = f"cbox-{container_id}"
        entrypoint = f"/content/.cbox/bin/entrypoint.sh {container_id}"
        
        # Kill any pre-existing tmux session with the same name
        await transport.exec(["tmux", "kill-session", "-t", session_name])

        # Start tmux session running entrypoint
        cmd = ["tmux", "new-session", "-d", "-s", session_name, entrypoint]
        res = await transport.exec(cmd)
        if not res.success:
            raise ContainerStartError(f"Failed to start tmux session {session_name}: {res.stderr}")

    @staticmethod
    async def stop_container(
        transport: SSHTransport,
        container_id: str,
        timeout: int = 10,
    ) -> None:
        """Gracefully stop container process inside tmux session."""
        container_dir = f"/content/.cbox/containers/{container_id}"
        session_name = f"cbox-{container_id}"

        # Read PID
        pid_res = await transport.exec(["cat", f"{container_dir}/pid"])
        if pid_res.success and pid_res.stdout.strip().isdigit():
            pid = pid_res.stdout.strip()
            # Send SIGTERM
            await transport.exec(["kill", "-15", pid])
            # Wait up to timeout seconds for process to exit
            check_cmd = ["sh", "-c", f"for i in $(seq 1 {timeout}); do kill -0 {pid} 2>/dev/null || exit 0; sleep 1; done; exit 1"]
            res = await transport.exec(check_cmd)
            if not res.success:
                # Force kill
                await transport.exec(["kill", "-9", pid])

        # Kill tmux session
        await transport.exec(["tmux", "kill-session", "-t", session_name])
        await transport.upload_text("stopped", f"{container_dir}/state")
