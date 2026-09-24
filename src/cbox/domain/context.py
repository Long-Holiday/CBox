"""Context domain model for multi-account and multi-provider isolation."""

from datetime import datetime, timezone
from pydantic import BaseModel, Field


class Context(BaseModel):
    """Context representing a specific provider and account profile configuration."""
    name: str
    provider: str = "colab"
    profile: str = "default"
    active: bool = False
    created_at: datetime = Field(default_factory=lambda: datetime.now(timezone.utc))
