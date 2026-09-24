"""Configuration management for CBox engine and CLI."""

from pathlib import Path
from typing import Any, Optional
import yaml
from pydantic import BaseModel, Field

from cbox.utils.paths import default_paths


class EngineConfig(BaseModel):
    state_dir: str = str(default_paths.state_dir)
    socket_path: str = str(default_paths.socket_path)


class RuntimeConfig(BaseModel):
    idle_timeout: int = 1800  # seconds (30 mins)


class SyncConfig(BaseModel):
    output_interval: int = 300  # seconds (5 mins default periodic sync)
    transfer: str = "auto"  # auto, rsync, tar-stream


class SSHConfig(BaseModel):
    control_persist: int = 600  # seconds


class SchedulerConfig(BaseModel):
    gpu_fallback: bool = True


class RecoveryConfig(BaseModel):
    enabled: bool = True
    max_attempts: int = 3
    check_interval: int = 30  # seconds


class CBoxConfig(BaseModel):
    engine: EngineConfig = Field(default_factory=EngineConfig)
    runtime: RuntimeConfig = Field(default_factory=RuntimeConfig)
    sync: SyncConfig = Field(default_factory=SyncConfig)
    ssh: SSHConfig = Field(default_factory=SSHConfig)
    scheduler: SchedulerConfig = Field(default_factory=SchedulerConfig)
    recovery: RecoveryConfig = Field(default_factory=RecoveryConfig)

    @classmethod
    def load(cls, path: Optional[Path] = None) -> "CBoxConfig":
        config_path = path or default_paths.config_file
        if not config_path.exists():
            return cls()
        try:
            with open(config_path, "r", encoding="utf-8") as f:
                data = yaml.safe_load(f) or {}
            return cls(**data)
        except Exception:
            return cls()

    def save(self, path: Optional[Path] = None) -> None:
        config_path = path or default_paths.config_file
        config_path.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
        with open(config_path, "w", encoding="utf-8") as f:
            yaml.safe_dump(self.model_dump(), f, default_flow_style=False)
