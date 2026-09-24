"""HTTP and Unix Socket Client for communicating with cboxd."""

import os
import subprocess
import time
from pathlib import Path
from typing import Any, Optional
import httpx

from cbox.domain.container import ContainerInspect, ContainerSpec, ContainerSummary
from cbox.domain.context import Context
from cbox.domain.image import ImageManifest, ImageSummary
from cbox.domain.volume import VolumeManifest, VolumeSummary
from cbox.utils.errors import DaemonNotRunningError, NotFoundError, UsageError
from cbox.utils.paths import default_paths


class CBoxClient:
    """Client for invoking cboxd REST API via Unix Domain Socket or HTTP."""

    def __init__(self, socket_path: Optional[Path | str] = None, base_url: Optional[str] = None):
        self.socket_path = Path(socket_path or default_paths.socket_path).resolve()
        self.base_url = base_url

    def _get_client(self) -> httpx.Client:
        if self.base_url:
            return httpx.Client(base_url=self.base_url, timeout=300.0)
        transport = httpx.HTTPTransport(uds=str(self.socket_path))
        return httpx.Client(transport=transport, base_url="http://cboxd", timeout=300.0)

    def is_daemon_running(self) -> bool:
        """Check if cboxd daemon is reachable."""
        try:
            with self._get_client() as client:
                res = client.get("/version")
                return res.status_code == 200
        except Exception:
            return False

    def ensure_daemon_running(self, auto_start: bool = True) -> None:
        """Verify daemon is active or spawn it automatically in background."""
        if self.is_daemon_running():
            return

        if not auto_start:
            raise DaemonNotRunningError(
                f"cboxd is not running on {self.socket_path}. Start it with 'cboxd' or systemctl --user start cboxd"
            )

        # Auto spawn cboxd in background
        default_paths.ensure_directories()
        log_file = default_paths.logs_dir / "cboxd.log"
        with open(log_file, "a") as f:
            subprocess.Popen(
                ["cboxd", "-s", str(self.socket_path)],
                stdout=f,
                stderr=subprocess.STDOUT,
                start_new_session=True,
            )

        # Wait up to 5 seconds for socket to be ready
        for _ in range(25):
            time.sleep(0.2)
            if self.is_daemon_running():
                return

        raise DaemonNotRunningError(
            f"Failed to automatically start cboxd. Check logs at {log_file}."
        )

    # System & Version
    def get_version(self) -> dict[str, Any]:
        self.ensure_daemon_running()
        with self._get_client() as c:
            return c.get("/version").json()

    def get_system_info(self) -> dict[str, Any]:
        self.ensure_daemon_running()
        with self._get_client() as c:
            return c.get("/system/info").json()

    # Images
    def build_image(self, context_dir: str, tag: Optional[str] = None, cboxfile: str = "Cboxfile") -> ImageManifest:
        self.ensure_daemon_running()
        with self._get_client() as c:
            res = c.post(
                "/images/build",
                json={"context_dir": context_dir, "tag": tag, "cboxfile_name": cboxfile},
            )
            if res.status_code != 200:
                raise UsageError(res.json().get("detail", res.text))
            return ImageManifest(**res.json())

    def list_images(self) -> list[ImageSummary]:
        self.ensure_daemon_running()
        with self._get_client() as c:
            res = c.get("/images")
            return [ImageSummary(**item) for item in res.json()]

    def delete_image(self, identifier: str) -> None:
        self.ensure_daemon_running()
        with self._get_client() as c:
            res = c.delete(f"/images/{identifier}")
            if res.status_code != 200:
                raise NotFoundError(res.json().get("detail", res.text))

    # Containers
    def create_container(self, spec: ContainerSpec) -> str:
        self.ensure_daemon_running()
        with self._get_client() as c:
            res = c.post("/containers/create", json=spec.model_dump())
            if res.status_code != 200:
                raise UsageError(res.json().get("detail", res.text))
            return res.json()["id"]

    def start_container(self, identifier: str) -> None:
        self.ensure_daemon_running()
        with self._get_client() as c:
            res = c.post(f"/containers/{identifier}/start")
            if res.status_code != 200:
                raise UsageError(res.json().get("detail", res.text))

    def stop_container(self, identifier: str, timeout: int = 10) -> None:
        self.ensure_daemon_running()
        with self._get_client() as c:
            res = c.post(f"/containers/{identifier}/stop?timeout={timeout}")
            if res.status_code != 200:
                raise UsageError(res.json().get("detail", res.text))

    def restart_container(self, identifier: str) -> None:
        self.ensure_daemon_running()
        with self._get_client() as c:
            res = c.post(f"/containers/{identifier}/restart")
            if res.status_code != 200:
                raise UsageError(res.json().get("detail", res.text))

    def remove_container(self, identifier: str, force: bool = False) -> None:
        self.ensure_daemon_running()
        with self._get_client() as c:
            res = c.delete(f"/containers/{identifier}?force={force}")
            if res.status_code != 200:
                raise UsageError(res.json().get("detail", res.text))

    def list_containers(self, all_containers: bool = False) -> list[ContainerSummary]:
        self.ensure_daemon_running()
        with self._get_client() as c:
            res = c.get(f"/containers?all={all_containers}")
            return [ContainerSummary(**item) for item in res.json()]

    def inspect_container(self, identifier: str) -> ContainerInspect:
        self.ensure_daemon_running()
        with self._get_client() as c:
            res = c.get(f"/containers/{identifier}")
            if res.status_code != 200:
                raise NotFoundError(res.json().get("detail", res.text))
            return ContainerInspect(**res.json())

    def get_logs(self, identifier: str, tail: int = 100, follow: bool = False) -> str:
        self.ensure_daemon_running()
        with self._get_client() as c:
            res = c.get(f"/containers/{identifier}/logs?tail={tail}&follow={follow}")
            if res.status_code != 200:
                raise NotFoundError(res.json().get("detail", res.text))
            return res.json()["logs"]

    def exec_command(self, identifier: str, command: list[str]) -> tuple[int, str, str]:
        self.ensure_daemon_running()
        with self._get_client() as c:
            res = c.post(f"/containers/{identifier}/exec", json={"command": command})
            if res.status_code != 200:
                raise UsageError(res.json().get("detail", res.text))
            data = res.json()
            return data["exit_code"], data["stdout"], data["stderr"]

    def get_stats(self, identifier: Optional[str] = None) -> list[dict[str, Any]]:
        self.ensure_daemon_running()
        with self._get_client() as c:
            url = f"/containers/{identifier}/stats" if identifier else "/containers/all/stats"
            res = c.get(url)
            return res.json() if res.status_code == 200 else []

    # Volumes
    def create_volume(
        self,
        name: str,
        source: str,
        mode: str = "ro",
        immutable: bool = False,
        checksum: bool = False,
    ) -> VolumeManifest:
        self.ensure_daemon_running()
        with self._get_client() as c:
            res = c.post(
                "/volumes",
                json={
                    "name": name,
                    "source": source,
                    "mode": mode,
                    "immutable": immutable,
                    "checksum": checksum,
                },
            )
            if res.status_code != 200:
                raise UsageError(res.json().get("detail", res.text))
            return VolumeManifest(**res.json())

    def list_volumes(self) -> list[VolumeSummary]:
        self.ensure_daemon_running()
        with self._get_client() as c:
            res = c.get("/volumes")
            return [VolumeSummary(**item) for item in res.json()]

    def delete_volume(self, name: str) -> None:
        self.ensure_daemon_running()
        with self._get_client() as c:
            res = c.delete(f"/volumes/{name}")
            if res.status_code != 200:
                raise NotFoundError(res.json().get("detail", res.text))

    # Contexts
    def list_contexts(self) -> list[Context]:
        self.ensure_daemon_running()
        with self._get_client() as c:
            res = c.get("/contexts")
            return [Context(**item) for item in res.json()]

    def create_context(self, name: str, provider: str = "colab", profile: str = "default") -> Context:
        self.ensure_daemon_running()
        with self._get_client() as c:
            res = c.post("/contexts", json={"name": name, "provider": provider, "profile": profile})
            return Context(**res.json())

    def use_context(self, name: str) -> None:
        self.ensure_daemon_running()
        with self._get_client() as c:
            res = c.post(f"/contexts/{name}/use")
            if res.status_code != 200:
                raise NotFoundError(res.json().get("detail", res.text))

    def delete_context(self, name: str) -> None:
        self.ensure_daemon_running()
        with self._get_client() as c:
            res = c.delete(f"/contexts/{name}")
            if res.status_code != 200:
                raise NotFoundError(res.json().get("detail", res.text))

    # Compose
    def compose_up(self, compose_file: str, project_name: Optional[str] = None, detach: bool = True) -> list[str]:
        self.ensure_daemon_running()
        with self._get_client() as c:
            res = c.post(
                "/compose/up",
                json={"compose_file": compose_file, "project_name": project_name, "detach": detach},
            )
            if res.status_code != 200:
                raise UsageError(res.json().get("detail", res.text))
            return res.json()["services"]

    def compose_down(self, compose_file: str, project_name: Optional[str] = None) -> list[str]:
        self.ensure_daemon_running()
        with self._get_client() as c:
            res = c.post(
                "/compose/down",
                json={"compose_file": compose_file, "project_name": project_name},
            )
            if res.status_code != 200:
                raise UsageError(res.json().get("detail", res.text))
            return res.json()["services"]

    def compose_ps(self, compose_file: str, project_name: Optional[str] = None) -> list[ContainerSummary]:
        self.ensure_daemon_running()
        with self._get_client() as c:
            res = c.get(f"/compose/ps?compose_file={compose_file}&project_name={project_name or ''}")
            return [ContainerSummary(**item) for item in res.json()]


default_client = CBoxClient()
