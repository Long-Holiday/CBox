"""Process and runtime health status inspection on remote worker."""

from typing import Optional, Tuple
from cbox.domain.enums import ContainerState
from cbox.transport.ssh import SSHTransport


class WorkerHealth:
    """Checks remote container process status."""

    @staticmethod
    async def inspect_container_process(
        transport: SSHTransport,
        container_id: str,
    ) -> Tuple[ContainerState, Optional[int], Optional[int]]:
        """Inspect remote container state via tmux and status files.

        Returns (ContainerState, exit_code, pid).
        """
        container_dir = f"/content/.cbox/containers/{container_id}"
        session_name = f"cbox-{container_id}"

        # 1. Check tmux session
        tmux_res = await transport.exec(["tmux", "has-session", "-t", session_name])
        tmux_alive = tmux_res.returncode == 0

        # 2. Read PID
        pid_res = await transport.exec(["cat", f"{container_dir}/pid"])
        pid = int(pid_res.stdout.strip()) if pid_res.success and pid_res.stdout.strip().isdigit() else None

        # 3. Read exit code
        exit_res = await transport.exec(["cat", f"{container_dir}/exit_code"])
        exit_code = int(exit_res.stdout.strip()) if exit_res.success and exit_res.stdout.strip().lstrip("-").isdigit() else None

        # 4. Read state file
        state_res = await transport.exec(["cat", f"{container_dir}/state"])
        state_str = state_res.stdout.strip().lower() if state_res.success else ""

        if tmux_alive:
            return ContainerState.RUNNING, None, pid

        # Session has ended
        if state_str == "stopped":
            return ContainerState.STOPPED, exit_code, pid

        if exit_code is not None:
            if exit_code == 0:
                return ContainerState.EXITED, 0, pid
            return ContainerState.EXITED, exit_code, pid

        if state_str == "failed":
            return ContainerState.FAILED, exit_code or 1, pid

        return ContainerState.EXITED, exit_code, pid
