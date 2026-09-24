"""Main Typer CLI entrypoint for CBox."""

import sys
import typer
from rich.console import Console

from cbox import __version__
from cbox.cli.compose import compose_cli
from cbox.cli.container import (
    container_cli,
    copy_file,
    create_container,
    exec_container,
    get_logs,
    inspect_container,
    list_containers,
    remove_container,
    restart_container,
    run_container,
    start_container,
    stats_container,
    stop_container,
)
from cbox.cli.context import context_cli
from cbox.cli.image import build, image_cli, list_images, remove_image
from cbox.cli.volume import volume_cli
from cbox.client.api import default_client

console = Console()

cli_app = typer.Typer(
    name="cbox",
    help="CBox: Docker-like Colab GPU Runtime Engine",
    no_args_is_help=True,
)

# Sub-command groups
cli_app.add_typer(image_cli, name="image")
cli_app.add_typer(volume_cli, name="volume")
cli_app.add_typer(context_cli, name="context")
cli_app.add_typer(compose_cli, name="compose")

# Top-level Docker-like aliases conforming to plan.md section 66
cli_app.command(name="build")(build)
cli_app.command(name="images")(list_images)
cli_app.command(name="rmi")(remove_image)

passthrough_settings = {"allow_extra_args": True, "ignore_unknown_options": True}
cli_app.command(name="create", context_settings=passthrough_settings)(create_container)
cli_app.command(name="run", context_settings=passthrough_settings)(run_container)
cli_app.command(name="start")(start_container)
cli_app.command(name="stop")(stop_container)
cli_app.command(name="restart")(restart_container)
cli_app.command(name="rm")(remove_container)
cli_app.command(name="ps")(list_containers)
cli_app.command(name="inspect")(inspect_container)
cli_app.command(name="logs")(get_logs)
cli_app.command(name="exec", context_settings=passthrough_settings)(exec_container)
cli_app.command(name="cp")(copy_file)
cli_app.command(name="stats")(stats_container)


system_cli = typer.Typer(help="Manage CBox system")
cli_app.add_typer(system_cli, name="system")


@system_cli.command(name="info")
def system_info():
    """Display system-wide information."""
    try:
        info = default_client.get_system_info()
        console.print_json(data=info)
    except Exception as e:
        console.print(f"[bold red]Error:[/bold red] {e}")
        raise typer.Exit(code=1)


@cli_app.command(name="version")
def version():
    """Show the CBox version information."""
    console.print(f"CBox version {__version__}")


if __name__ == "__main__":
    cli_app()
