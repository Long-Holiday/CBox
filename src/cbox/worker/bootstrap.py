"""Worker bootstrapping orchestration on remote runtime."""

import os
from pathlib import Path
from cbox.transport.ssh import SSHTransport
from cbox.utils.errors import TransportError


async def bootstrap_remote_worker(transport: SSHTransport) -> bool:
    """Deploy bootstrap scripts and initialize /content/.cbox on remote VM."""
    worker_script_dir = Path(__file__).parent.parent.parent.parent / "worker"
    if not worker_script_dir.exists():
        # Fallback if running as installed package
        worker_script_dir = Path("/home/default_user/CBox/worker")

    # 1. Create remote directory
    mkdir_res = await transport.exec(["mkdir", "-p", "/content/.cbox/bin"])
    if not mkdir_res.success:
        raise TransportError(f"Failed to create /content/.cbox/bin: {mkdir_res.stderr}")

    # 2. Upload worker scripts
    scripts = ["bootstrap.sh", "entrypoint.sh", "exec.sh", "health.sh"]
    for s in scripts:
        local_script = worker_script_dir / s
        if local_script.exists():
            await transport.upload_text(local_script.read_text(encoding="utf-8"), f"/content/.cbox/bin/{s}", mode="0755")

    # 3. Run bootstrap.sh
    res = await transport.exec(["/content/.cbox/bin/bootstrap.sh"])
    return res.success and "CBOX_BOOTSTRAP_COMPLETE" in res.stdout
