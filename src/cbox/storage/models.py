"""SQLAlchemy models for CBox state persistence."""

from datetime import datetime, timezone
from sqlalchemy import (
    Boolean,
    Column,
    DateTime,
    ForeignKey,
    Integer,
    String,
    Text,
)
from sqlalchemy.orm import relationship

from cbox.storage.database import Base


def utc_now() -> datetime:
    return datetime.now(timezone.utc).replace(tzinfo=None)


class ImageModel(Base):
    __tablename__ = "images"

    id = Column(String(64), primary_key=True)  # sha256:...
    created_at = Column(DateTime, default=utc_now)
    manifest = Column(Text, nullable=False)  # JSON string

    tags = relationship("ImageTagModel", back_populates="image", cascade="all, delete-orphan")


class ImageTagModel(Base):
    __tablename__ = "image_tags"

    id = Column(Integer, primary_key=True, autoincrement=True)
    repository = Column(String(255), nullable=False)
    tag = Column(String(128), nullable=False)
    image_id = Column(String(64), ForeignKey("images.id"), nullable=False)

    image = relationship("ImageModel", back_populates="tags")


class VolumeModel(Base):
    __tablename__ = "volumes"

    id = Column(String(64), primary_key=True)
    name = Column(String(255), unique=True, nullable=False)
    source = Column(Text, nullable=False)
    hash = Column(String(64), nullable=False)
    mode = Column(String(32), default="ro")
    immutable = Column(Boolean, default=False)
    version = Column(Integer, default=1)
    created_at = Column(DateTime, default=utc_now)


class ContainerModel(Base):
    __tablename__ = "containers"

    id = Column(String(64), primary_key=True)
    name = Column(String(255), unique=True, nullable=False)
    image_id = Column(String(64), ForeignKey("images.id"), nullable=False)
    state = Column(String(32), default="created")
    command = Column(Text, nullable=False)  # JSON string
    workdir = Column(String(512), default="/workspace")
    restart_policy = Column(String(32), default="no")
    resume_command = Column(Text, nullable=True)  # JSON string
    exit_code = Column(Integer, nullable=True)
    pid = Column(Integer, nullable=True)
    error_message = Column(Text, nullable=True)
    created_at = Column(DateTime, default=utc_now)
    started_at = Column(DateTime, nullable=True)
    finished_at = Column(DateTime, nullable=True)
    runtime_id = Column(String(64), nullable=True)
    gpu_requested = Column(Text, nullable=False)  # JSON string
    gpu_assigned = Column(String(64), nullable=True)
    high_mem = Column(Boolean, default=False)
    auto_remove = Column(Boolean, default=False)
    context_name = Column(String(64), nullable=True)

    mounts = relationship("ContainerMountModel", back_populates="container", cascade="all, delete-orphan")


class ContainerMountModel(Base):
    __tablename__ = "container_mounts"

    id = Column(Integer, primary_key=True, autoincrement=True)
    container_id = Column(String(64), ForeignKey("containers.id"), nullable=False)
    volume_id = Column(String(64), nullable=False)
    source = Column(Text, nullable=False)
    target = Column(String(512), nullable=False)
    mode = Column(String(32), default="ro")

    container = relationship("ContainerModel", back_populates="mounts")


class RuntimeModel(Base):
    __tablename__ = "runtimes"

    id = Column(String(64), primary_key=True)
    provider = Column(String(64), default="colab")
    provider_session = Column(String(128), nullable=False)
    profile = Column(String(128), default="default")
    accelerator = Column(String(64), default="T4")
    high_mem = Column(Boolean, default=False)
    state = Column(String(32), default="provisioning")
    ssh_host = Column(String(128), default="")
    created_at = Column(DateTime, default=utc_now)
    last_seen = Column(DateTime, default=utc_now)
    idle_since = Column(DateTime, nullable=True)


class RuntimeCacheModel(Base):
    __tablename__ = "runtime_cache"

    id = Column(Integer, primary_key=True, autoincrement=True)
    runtime_id = Column(String(64), ForeignKey("runtimes.id"), nullable=False)
    object_type = Column(String(32), nullable=False)  # "image" or "volume"
    object_id = Column(String(64), nullable=False)
    hash = Column(String(64), nullable=False)


class ContextModel(Base):
    __tablename__ = "contexts"

    name = Column(String(64), primary_key=True)
    provider = Column(String(64), default="colab")
    profile = Column(String(128), default="default")
    active = Column(Boolean, default=False)
    created_at = Column(DateTime, default=utc_now)


class EventModel(Base):
    __tablename__ = "events"

    id = Column(Integer, primary_key=True, autoincrement=True)
    object_type = Column(String(32), nullable=False)
    object_id = Column(String(64), nullable=False)
    event = Column(String(64), nullable=False)
    timestamp = Column(DateTime, default=utc_now)
    payload = Column(Text, nullable=True)  # JSON string
