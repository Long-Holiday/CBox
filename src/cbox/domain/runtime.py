"""Compute runtime domain model."""

from datetime import datetime, timezone
from typing import Optional
from pydantic import BaseModel, Field

from cbox.domain.enums import RuntimeState


class Runtime(BaseModel):
    """Represents an active or pooled compute runtime (e.g. Colab VM instance)."""
    id: str
    provider: str = "colab"
    provider_session: str
    profile: str = "default"
    accelerator: str = "T4"
    high_mem: bool = False
    state: RuntimeState = RuntimeState.PROVISIONING
    ssh_host: str = ""
    created_at: datetime = Field(default_factory=lambda: datetime.now(timezone.utc))
    last_seen: datetime = Field(default_factory=lambda: datetime.now(timezone.utc))
    idle_since: Optional[datetime] = None
