"""Parser for Colab CLI session outputs."""

import re
from typing import Optional


class ColabSessionInfo:
    def __init__(self, name: str, status: str, accelerator: Optional[str] = None):
        self.name = name
        self.status = status
        self.accelerator = accelerator


def parse_sessions_output(output: str) -> list[ColabSessionInfo]:
    """Parse output from `colab sessions`."""
    sessions = []
    for line in output.splitlines():
        line = line.strip()
        if not line or line.startswith("NAME") or line.startswith("──"):
            continue
        parts = re.split(r"\s{2,}", line)
        if len(parts) >= 2:
            name = parts[0]
            status = parts[1]
            acc = parts[2] if len(parts) > 2 else None
            sessions.append(ColabSessionInfo(name=name, status=status, accelerator=acc))
    return sessions
