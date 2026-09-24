"""Entrypoint CLI for cboxd daemon."""

import os
import sys
from pathlib import Path
from typing import Optional
import typer
import uvicorn

from cbox.daemon.app import app
from cbox.utils.paths import default_paths

daemon_cli = typer.Typer(help="CBox Engine Daemon (cboxd)")


@daemon_cli.command()
def main(
    socket: Optional[str] = typer.Option(
        None,
        "--socket",
        "-s",
        help="Path to unix domain socket (default: $XDG_RUNTIME_DIR/cbox/cbox.sock)",
    ),
    host: Optional[str] = typer.Option(
        None,
        "--host",
        "-h",
        help="Bind HTTP server to TCP host instead of unix socket (e.g. 127.0.0.1)",
    ),
    port: int = typer.Option(
        8080,
        "--port",
        "-p",
        help="TCP port when --host is provided",
    ),
    log_level: str = typer.Option(
        "info",
        "--log-level",
        help="Logging level (debug, info, warning, error)",
    ),
):
    """Start the cboxd engine daemon."""
    default_paths.ensure_directories()

    if host:
        print(f"[cboxd] Starting HTTP server on http://{host}:{port} ...")
        uvicorn.run(app, host=host, port=port, log_level=log_level)
    else:
        sock_path = Path(socket).resolve() if socket else default_paths.socket_path
        sock_path.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
        # Clean up stale socket file if it exists
        if sock_path.exists():
            try:
                sock_path.unlink()
            except OSError:
                pass

        print(f"[cboxd] Starting CBox Engine Daemon on unix://{sock_path} ...")
        try:
            uvicorn.run(app, uds=str(sock_path), log_level=log_level)
        finally:
            if sock_path.exists():
                try:
                    sock_path.unlink()
                except OSError:
                    pass


if __name__ == "__main__":
    daemon_cli()
