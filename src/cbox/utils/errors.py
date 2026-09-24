"""Error taxonomy and exit code mapping for CBox."""

from typing import Optional


class CBoxError(Exception):
    """Base exception for all CBox errors."""

    exit_code: int = 1

    def __init__(self, message: str, details: Optional[str] = None):
        super().__init__(message)
        self.message = message
        self.details = details

    def __str__(self) -> str:
        if self.details:
            return f"{self.message}\nDetails: {self.details}"
        return self.message


class ConfigError(CBoxError):
    """Configuration error or invalid CLI option."""

    exit_code: int = 2


class UsageError(CBoxError):
    """Invalid command line arguments or invalid usage."""

    exit_code: int = 2


class ProviderError(CBoxError):
    """Failure communicating with backend compute provider (e.g. Colab)."""

    exit_code: int = 10


class AllocationError(ProviderError):
    """GPU / machine allocation failure on provider."""

    exit_code: int = 11


class AuthenticationError(ProviderError):
    """Authentication or OAuth token invalid/missing."""

    exit_code: int = 10


class TransportError(CBoxError):
    """SSH or network transport error."""

    exit_code: int = 12


class ImageBuildError(CBoxError):
    """Error parsing Cboxfile or generating image build plan."""

    exit_code: int = 20


class ImageMaterializationError(CBoxError):
    """Error materializing packages/files on the remote worker runtime."""

    exit_code: int = 20


class VolumeSyncError(CBoxError):
    """Error creating, hashing, or synchronizing remote volume."""

    exit_code: int = 30


class ContainerStartError(CBoxError):
    """Error starting remote worker process or tmux session."""

    exit_code: int = 40


class ContainerLostError(CBoxError):
    """Remote runtime or process unexpectedly terminated."""

    exit_code: int = 40


class RecoveryError(CBoxError):
    """Error executing recovery plan for interrupted container."""

    exit_code: int = 40


class NotFoundError(CBoxError):
    """Requested resource (image, container, volume, context) not found."""

    exit_code: int = 1


class DaemonNotRunningError(CBoxError):
    """cboxd daemon is not running or socket is unreachable."""

    exit_code: int = 1
