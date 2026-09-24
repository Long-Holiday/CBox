"""Compose orchestration service for multi-container projects."""

from pathlib import Path
from typing import Optional
from sqlalchemy.orm import Session

from cbox.compose.parser import compose_service_to_spec, parse_compose_file
from cbox.domain.container import ContainerSummary
from cbox.services.container_service import ContainerService


class ComposeService:
    """Manages multi-container experiments defined in cbox-compose.yaml."""

    def __init__(self, session: Session, container_service: Optional[ContainerService] = None):
        self.session = session
        self.container_service = container_service or ContainerService(session)

    async def up(
        self,
        compose_file: Path | str = "cbox-compose.yaml",
        project_name: Optional[str] = None,
        detach: bool = True,
    ) -> list[str]:
        """Launch all services defined in compose file."""
        file_path = Path(compose_file).resolve()
        project = project_name or file_path.parent.name
        parsed = parse_compose_file(file_path)

        started_ids = []
        for svc_name, svc_conf in parsed.services.items():
            spec = compose_service_to_spec(svc_name, svc_conf, project)
            cid = self.container_service.create_container(spec)
            await self.container_service.start_container(cid)
            started_ids.append(cid)

        return started_ids

    async def down(
        self,
        compose_file: Path | str = "cbox-compose.yaml",
        project_name: Optional[str] = None,
    ) -> list[str]:
        """Stop and remove all containers belonging to project."""
        file_path = Path(compose_file).resolve()
        project = project_name or file_path.parent.name

        stopped_containers = []
        for c in self.container_service.list_containers(all_containers=True):
            if c.name.startswith(f"{project}_"):
                try:
                    await self.container_service.stop_container(c.id)
                except Exception:
                    pass
                self.container_service.remove_container(c.id, force=True)
                stopped_containers.append(c.name)

        return stopped_containers

    def ps(
        self,
        compose_file: Path | str = "cbox-compose.yaml",
        project_name: Optional[str] = None,
    ) -> list[ContainerSummary]:
        """List containers belonging to the compose project."""
        file_path = Path(compose_file).resolve()
        project = project_name or file_path.parent.name
        containers = self.container_service.list_containers(all_containers=True)
        return [c for c in containers if c.name.startswith(f"{project}_")]
