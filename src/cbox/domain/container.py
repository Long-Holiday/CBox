"""Container domain model and specification."""

from datetime import datetime, timezone
from typing import Any, Optional
from pydantic import BaseModel, Field

from cbox.domain.enums import ContainerState, RestartPolicy
from cbox.domain.volume import VolumeMount


class ContainerSpec(BaseModel):
    """Specification requested when creating or running a container."""
    name: str
    image: str
    command: list[str] = Field(default_factory=list)
    gpu: list[str] = Field(default_factory=lambda: ["T4"])
    high_mem: bool = False
    workdir: str = "/workspace"
    env: dict[str, str] = Field(default_factory=dict)
    secrets: list[str] = Field(default_factory=list)
    mounts: list[VolumeMount] = Field(default_factory=list)
    restart_policy: RestartPolicy = RestartPolicy.NO
    resume_command: Optional[list[str]] = None
    context: Optional[str] = None
    auto_remove: bool = False


class ContainerStateInfo(BaseModel):
    """Current state of a container."""
    status: ContainerState = ContainerState.CREATED
    exit_code: Optional[int] = None
    error_message: Optional[str] = None
    started_at: Optional[datetime] = None
    finished_at: Optional[datetime] = None
    pid: Optional[int] = None


class ContainerInspect(BaseModel):
    """Detailed inspection conforming to plan.md section 32."""
    id: str
    name: str
    image: str
    state: ContainerStateInfo
    resource: dict[str, Any] = Field(default_factory=dict)
    runtime: dict[str, Any] = Field(default_factory=dict)
    mounts: list[dict[str, Any]] = Field(default_factory=list)
    command: list[str] = Field(default_factory=list)
    workdir: str = "/workspace"
    created_at: datetime = Field(default_factory=lambda: datetime.now(timezone.utc))


class ContainerSummary(BaseModel):
    """Summary for container listing (cbox ps)."""
    id: str
    name: str
    image: str
    gpu: str
    status: str
    state: ContainerState
    created_at: datetime
