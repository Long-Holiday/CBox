"""Image domain model and Cboxfile manifest definitions."""

from datetime import datetime, timezone
from typing import Any, Optional
from pydantic import BaseModel, Field


class CopyInstruction(BaseModel):
    src: str
    dest: str


class ImageManifest(BaseModel):
    """Normalized declarative blueprint of an image."""
    id: str  # sha256:...
    tags: list[str] = Field(default_factory=list)
    base: dict[str, str] = Field(default_factory=lambda: {"provider": "colab", "python": "3"})
    apt: list[str] = Field(default_factory=list)
    pip: list[str] = Field(default_factory=list)
    environment: dict[str, str] = Field(default_factory=dict)
    workdir: str = "/workspace"
    copies: list[CopyInstruction] = Field(default_factory=list)
    run_commands: list[str] = Field(default_factory=list)
    cmd: list[str] = Field(default_factory=lambda: ["bash"])
    healthcheck: Optional[str] = None
    created_at: datetime = Field(default_factory=lambda: datetime.now(timezone.utc))

    def to_canonical_dict(self) -> dict[str, Any]:
        """Convert manifest to normalized dict for deterministic hashing."""
        return {
            "base": self.base,
            "apt": sorted(list(set(self.apt))),
            "pip": sorted(list(set(self.pip))),
            "environment": dict(sorted(self.environment.items())),
            "workdir": self.workdir,
            "copies": [c.model_dump() for c in self.copies],
            "run_commands": self.run_commands,
            "cmd": self.cmd,
            "healthcheck": self.healthcheck,
        }


class ImageSummary(BaseModel):
    """Lightweight representation of an image for listing."""
    id: str
    tags: list[str]
    created_at: datetime
    size_description: str = "Blueprint"
