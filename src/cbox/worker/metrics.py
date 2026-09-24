"""Resource metrics collection for cbox stats."""

import json
from typing import Any
from cbox.transport.ssh import SSHTransport


class WorkerMetrics:
    """Retrieves live GPU, CPU, and RAM metrics from worker."""

    @staticmethod
    async def get_metrics(transport: SSHTransport) -> dict[str, Any]:
        """Execute health.sh on remote runtime and parse JSON statistics."""
        res = await transport.exec(["/content/.cbox/bin/health.sh"], timeout=10.0)
        if not res.success or not res.stdout.strip():
            return {
                "gpu_name": "None",
                "gpu_mem_used_mb": 0,
                "gpu_mem_total_mb": 0,
                "gpu_util_percent": 0,
                "cpu_util_percent": 0,
                "ram_used_mb": 0,
                "ram_total_mb": 0,
            }
        try:
            return json.loads(res.stdout.strip())
        except Exception:
            return {
                "gpu_name": "Unknown",
                "gpu_mem_used_mb": 0,
                "gpu_mem_total_mb": 0,
                "gpu_util_percent": 0,
                "cpu_util_percent": 0,
                "ram_used_mb": 0,
                "ram_total_mb": 0,
            }
