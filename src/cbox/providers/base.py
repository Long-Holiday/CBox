"""Compute Provider interface definition."""

from abc import ABC, abstractmethod
from typing import Optional
from pydantic import BaseModel, Field

from cbox.domain.enums import RuntimeState
from cbox.domain.runtime import Runtime
from cbox.transport.ssh import SSHTransport


class RuntimeRequest(BaseModel):
    """Request for allocating a new compute runtime."""
    session_name: str
    accelerators: list[str] = Field(default_factory=lambda: ["T4"])  # fallback list, e.g. ["L4", "T4"]
    high_mem: bool = False
    profile: str = "default"


class Provider(ABC):
    """Abstract base class for compute providers (Colab, RunPod, GCP, Mock, etc.)."""

    name: str

    @abstractmethod
    async def create_runtime(self, request: RuntimeRequest) -> Runtime:
        """Provision a new compute instance with GPU accelerator preference fallback."""
        pass

    @abstractmethod
    async def stop_runtime(self, runtime: Runtime) -> None:
        """Terminate and release a compute instance."""
        pass

    @abstractmethod
    async def get_runtime_status(self, runtime: Runtime) -> RuntimeState:
        """Check live reachability and state of the compute instance."""
        pass

    @abstractmethod
    def open_transport(self, runtime: Runtime) -> SSHTransport:
        """Obtain an SSHTransport instance configured for this runtime."""
        pass
