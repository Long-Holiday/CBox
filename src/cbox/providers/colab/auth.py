"""Profile and authentication management for Colab accounts."""

import os
from pathlib import Path
from typing import Optional

from cbox.utils.paths import default_paths


class ColabProfile:
    """Manages isolated home and session configuration for a Colab account."""

    def __init__(self, profile_name: str = "default"):
        self.profile_name = profile_name
        self.profile_dir = default_paths.profiles_dir / profile_name
        self.home_dir = self.profile_dir / "home"
        self.sessions_file = self.profile_dir / "sessions.json"
        self.oauth_config_file = self.profile_dir / "oauth.json"

    def ensure(self) -> None:
        """Create isolated profile directories with restricted permissions."""
        self.profile_dir.mkdir(parents=True, exist_ok=True, mode=0o700)
        self.home_dir.mkdir(parents=True, exist_ok=True, mode=0o700)

    def get_env(self) -> dict[str, str]:
        """Return environment variables with isolated HOME."""
        self.ensure()
        return {
            "HOME": str(self.home_dir),
        }

    def get_colab_cli_flags(self) -> list[str]:
        """Return CLI flags for isolating sessions and oauth config."""
        self.ensure()
        flags = []
        if self.sessions_file.exists() or True:
            flags.extend(["--config", str(self.sessions_file)])
        if self.oauth_config_file.exists():
            flags.extend(["--client-oauth-config", str(self.oauth_config_file)])
        return flags
