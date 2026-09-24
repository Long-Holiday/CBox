"""Volume domain model and mount specifications."""

from datetime import datetime, timezone
from typing import Optional
from pydantic import BaseModel, Field

from cbox.domain.enums import VolumeMode


class VolumeMount(BaseModel):
    """Specification of a mount in a container."""
    source: str  # host path or named volume
    target: str  # remote container path (e.g. /data or /output)
    mode: VolumeMode = VolumeMode.RO

    @classmethod
    def parse(cls, mount_str: str) -> "VolumeMount":
        """Parse docker-like volume string:
        e.g.:
        /data/Potsdam:/data:ro
        ./runs/exp01:/output:output
        source=potsdam,target=/data,mode=ro
        """
        if "=" in mount_str:
            # key-value syntax: source=...,target=...,mode=...
            parts = dict(item.split("=", 1) for item in mount_str.split(","))
            return cls(
                source=parts.get("source", parts.get("src", "")),
                target=parts.get("target", parts.get("dst", parts.get("destination", ""))),
                mode=VolumeMode(parts.get("mode", "ro")),
            )
        # colon syntax: src:dst[:mode]
        parts = mount_str.split(":")
        if len(parts) == 1:
            raise ValueError(f"Invalid volume syntax: {mount_str}")
        source = parts[0]
        target = parts[1]
        mode = VolumeMode(parts[2]) if len(parts) > 2 else VolumeMode.RO
        return cls(source=source, target=target, mode=mode)


class VolumeManifest(BaseModel):
    """Metadata representing a managed named or anonymous volume."""
    id: str
    name: str
    source: str
    hash: str
    mode: VolumeMode = VolumeMode.RO
    immutable: bool = False
    version: int = 1
    created_at: datetime = Field(default_factory=lambda: datetime.now(timezone.utc))


class VolumeSummary(BaseModel):
    """Lightweight volume representation for listing."""
    name: str
    source: str
    mode: VolumeMode
    immutable: bool
    version: int
    created_at: datetime
