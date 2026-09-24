"""System information and version endpoints."""

import sys
from fastapi import APIRouter
from cbox import __version__
from cbox.utils.paths import default_paths

router = APIRouter(tags=["system"])


@router.get("/version")
def get_version():
    """Return version and environment metadata."""
    return {
        "version": __version__,
        "python": sys.version,
        "api_version": "v1",
    }


@router.get("/system/info")
def get_system_info():
    """Return system layout and runtime paths."""
    return {
        "version": __version__,
        "state_dir": str(default_paths.state_dir),
        "config_dir": str(default_paths.config_dir),
        "socket_path": str(default_paths.socket_path),
        "db_path": str(default_paths.db_path),
    }
