"""Path resolution adhering to XDG specification for CBox."""

import os
from pathlib import Path


class CBoxPaths:
    """Manages all local filesystem paths used by CBox and cboxd."""

    def __init__(
        self,
        config_dir: str | Path | None = None,
        state_dir: str | Path | None = None,
        socket_path: str | Path | None = None,
    ):
        # Config dir: ~/.config/cbox by default
        if config_dir:
            self.config_dir = Path(config_dir).expanduser().resolve()
        elif "CBOX_CONFIG_DIR" in os.environ:
            self.config_dir = Path(os.environ["CBOX_CONFIG_DIR"]).expanduser().resolve()
        else:
            xdg_config = os.environ.get("XDG_CONFIG_HOME", "~/.config")
            self.config_dir = Path(xdg_config).expanduser() / "cbox"

        # State dir: ~/.local/share/cbox by default
        if state_dir:
            self.state_dir = Path(state_dir).expanduser().resolve()
        elif "CBOX_STATE_DIR" in os.environ:
            self.state_dir = Path(os.environ["CBOX_STATE_DIR"]).expanduser().resolve()
        else:
            xdg_data = os.environ.get("XDG_DATA_HOME", "~/.local/share")
            self.state_dir = Path(xdg_data).expanduser() / "cbox"

        # Socket path: $XDG_RUNTIME_DIR/cbox/cbox.sock or fallback to state_dir/cbox.sock
        if socket_path:
            self.socket_path = Path(socket_path).expanduser().resolve()
        elif "CBOX_SOCKET_PATH" in os.environ:
            self.socket_path = Path(os.environ["CBOX_SOCKET_PATH"]).expanduser().resolve()
        else:
            xdg_runtime = os.environ.get("XDG_RUNTIME_DIR")
            if xdg_runtime and Path(xdg_runtime).exists():
                self.socket_path = Path(xdg_runtime) / "cbox" / "cbox.sock"
            else:
                self.socket_path = self.state_dir / "cbox.sock"

    @property
    def config_file(self) -> Path:
        return self.config_dir / "config.yaml"

    @property
    def contexts_file(self) -> Path:
        return self.config_dir / "contexts.yaml"

    @property
    def secrets_dir(self) -> Path:
        return self.config_dir / "secrets"

    @property
    def db_path(self) -> Path:
        return self.state_dir / "cbox.db"

    @property
    def images_dir(self) -> Path:
        return self.state_dir / "images"

    @property
    def volumes_dir(self) -> Path:
        return self.state_dir / "volumes"

    @property
    def containers_dir(self) -> Path:
        return self.state_dir / "containers"

    @property
    def profiles_dir(self) -> Path:
        return self.state_dir / "profiles"

    @property
    def keys_dir(self) -> Path:
        return self.state_dir / "keys"

    @property
    def worker_ssh_key(self) -> Path:
        return self.keys_dir / "worker"

    @property
    def logs_dir(self) -> Path:
        return self.state_dir / "logs"

    @property
    def ssh_control_dir(self) -> Path:
        return self.state_dir / "ssh"

    def ensure_directories(self) -> None:
        """Create all required directories with secure permissions."""
        for directory in [
            self.config_dir,
            self.secrets_dir,
            self.state_dir,
            self.images_dir,
            self.volumes_dir,
            self.containers_dir,
            self.profiles_dir,
            self.keys_dir,
            self.logs_dir,
            self.ssh_control_dir,
            self.socket_path.parent,
        ]:
            directory.mkdir(parents=True, exist_ok=True, mode=0o700)


default_paths = CBoxPaths()
