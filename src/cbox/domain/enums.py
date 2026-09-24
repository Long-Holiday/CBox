"""Enumerations used across CBox domain entities."""

from enum import Enum


class ContainerState(str, Enum):
    """Lifecycle states of a CBox container."""
    CREATED = "created"
    PROVISIONING = "provisioning"
    PREPARING = "preparing"
    STARTING = "starting"
    RUNNING = "running"
    INTERRUPTED = "interrupted"
    RECOVERING = "recovering"
    STOPPING = "stopping"
    STOPPED = "stopped"
    EXITED = "exited"
    FAILED = "failed"
    REMOVED = "removed"


class RuntimeState(str, Enum):
    """Lifecycle states of a compute runtime."""
    PROVISIONING = "provisioning"
    READY = "ready"
    BUSY = "busy"
    IDLE = "idle"
    UNREACHABLE = "unreachable"
    LOST = "lost"
    STOPPED = "stopped"


class VolumeMode(str, Enum):
    """Synchronization modes for volumes."""
    RO = "ro"
    RW = "rw"
    OUTPUT = "output"
    CACHE = "cache"


class RestartPolicy(str, Enum):
    """Automatic restart policy."""
    NO = "no"
    ON_FAILURE = "on-failure"
    UNLESS_STOPPED = "unless-stopped"
    ALWAYS = "always"


class TransferMode(str, Enum):
    """Transfer mechanism for data synchronization."""
    AUTO = "auto"
    RSYNC = "rsync"
    TAR_STREAM = "tar-stream"


class EventType(str, Enum):
    """System event types."""
    CONTAINER_CREATE = "container.create"
    CONTAINER_START = "container.start"
    CONTAINER_RUNNING = "container.running"
    CONTAINER_STOP = "container.stop"
    CONTAINER_EXIT = "container.exit"
    CONTAINER_INTERRUPTED = "container.interrupted"
    CONTAINER_RECOVER_START = "container.recover.start"
    CONTAINER_RECOVER_DONE = "container.recover.done"
    CONTAINER_REMOVE = "container.remove"

    RUNTIME_PROVISION = "runtime.provision"
    RUNTIME_READY = "runtime.ready"
    RUNTIME_LOST = "runtime.lost"
    RUNTIME_STOP = "runtime.stop"

    IMAGE_BUILD = "image.build"
    IMAGE_MATERIALIZE_START = "image.materialize.start"
    IMAGE_MATERIALIZE_DONE = "image.materialize.done"

    VOLUME_CREATE = "volume.create"
    VOLUME_SYNC_START = "volume.sync.start"
    VOLUME_SYNC_DONE = "volume.sync.done"
