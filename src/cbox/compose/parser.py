"""Parser for cbox-compose.yaml files."""

from pathlib import Path
from typing import Any, Optional
import yaml
from pydantic import BaseModel, Field

from cbox.domain.container import ContainerSpec
from cbox.domain.enums import RestartPolicy, VolumeMode
from cbox.domain.volume import VolumeMount
from cbox.utils.errors import ConfigError


class ComposeVolumeConfig(BaseModel):
    source: str
    target: str
    mode: str = "ro"


class ComposeGPUConfig(BaseModel):
    preference: list[str] = Field(default_factory=lambda: ["T4"])


class ComposeServiceConfig(BaseModel):
    image: str
    command: list[str] = Field(default_factory=list)
    gpu: Optional[ComposeGPUConfig | list[str]] = None
    high_mem: bool = False
    workdir: str = "/workspace"
    environment: dict[str, str] = Field(default_factory=dict)
    volumes: list[ComposeVolumeConfig | str] = Field(default_factory=list)
    restart: str = "no"
    resume_command: Optional[list[str] | str] = None


class ComposeFile(BaseModel):
    version: str = "1"
    services: dict[str, ComposeServiceConfig] = Field(default_factory=dict)


def parse_compose_file(file_path: Path | str) -> ComposeFile:
    """Parse and validate cbox-compose.yaml."""
    path = Path(file_path).resolve()
    if not path.is_file():
        raise ConfigError(f"Compose file not found: {path}")

    with open(path, "r", encoding="utf-8") as f:
        data = yaml.safe_load(f) or {}

    try:
        return ComposeFile(**data)
    except Exception as e:
        raise ConfigError(f"Invalid cbox-compose.yaml schema: {e}")


def compose_service_to_spec(service_name: str, svc: ComposeServiceConfig, project_name: str) -> ContainerSpec:
    """Convert ComposeServiceConfig to domain ContainerSpec."""
    # GPU preference
    gpu_list = ["T4"]
    if svc.gpu:
        if isinstance(svc.gpu, list):
            gpu_list = svc.gpu
        elif isinstance(svc.gpu, ComposeGPUConfig):
            gpu_list = svc.gpu.preference
        elif isinstance(svc.gpu, dict):
            gpu_list = svc.gpu.get("preference", ["T4"])

    # Volumes
    mounts = []
    for v in svc.volumes:
        if isinstance(v, str):
            mounts.append(VolumeMount.parse(v))
        elif isinstance(v, ComposeVolumeConfig):
            mounts.append(VolumeMount(source=v.source, target=v.target, mode=VolumeMode(v.mode)))

    # Restart policy
    policy = RestartPolicy(svc.restart) if svc.restart in [p.value for p in RestartPolicy] else RestartPolicy.NO

    # Resume command
    resume_cmd = None
    if svc.resume_command:
        resume_cmd = [svc.resume_command] if isinstance(svc.resume_command, str) else svc.resume_command

    return ContainerSpec(
        name=f"{project_name}_{service_name}",
        image=svc.image,
        command=svc.command,
        gpu=gpu_list,
        high_mem=svc.high_mem,
        workdir=svc.workdir,
        env=svc.environment,
        mounts=mounts,
        restart_policy=policy,
        resume_command=resume_cmd,
    )
