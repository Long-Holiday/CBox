"""CLI commands for multi-container compose projects."""

from typing import Optional
import typer
from rich.console import Console
from rich.table import Table

from cbox.client.api import default_client
from cbox.utils.errors import CBoxError
from cbox.utils.hash import short_id

console = Console()
compose_cli = typer.Typer(help="Manage multi-container CBox Compose projects")


@compose_cli.command(name="up")
def compose_up(
    file: str = typer.Option("cbox-compose.yaml", "-f", "--file", help="Path to compose configuration file"),
    detach: bool = typer.Option(True, "-d", "--detach", help="Detached mode: Run containers in the background"),
    project: Optional[str] = typer.Option(None, "-p", "--project-name", help="Project name (defaults to directory name)"),
):
    """Build, create, and start containers defined in cbox-compose.yaml."""
    try:
        with console.status("[bold green]Starting compose services..."):
            started = default_client.compose_up(compose_file=file, project_name=project, detach=detach)
        console.print(f"[bold green]Successfully started {len(started)} services:[/bold green]")
        for cid in started:
            console.print(f"  - {short_id(cid)}")
    except CBoxError as e:
        console.print(f"[bold red]Compose up failed:[/bold red] {e}")
        raise typer.Exit(code=e.exit_code)
    except Exception as e:
        console.print(f"[bold red]Error:[/bold red] {e}")
        raise typer.Exit(code=1)


@compose_cli.command(name="down")
def compose_down(
    file: str = typer.Option("cbox-compose.yaml", "-f", "--file", help="Path to compose configuration file"),
    project: Optional[str] = typer.Option(None, "-p", "--project-name", help="Project name (defaults to directory name)"),
):
    """Stop and remove containers defined in cbox-compose.yaml."""
    try:
        with console.status("[bold green]Stopping compose services..."):
            stopped = default_client.compose_down(compose_file=file, project_name=project)
        console.print(f"[bold green]Stopped and removed {len(stopped)} services.[/bold green]")
    except Exception as e:
        console.print(f"[bold red]Error:[/bold red] {e}")
        raise typer.Exit(code=1)


@compose_cli.command(name="ps")
def compose_ps(
    file: str = typer.Option("cbox-compose.yaml", "-f", "--file", help="Path to compose configuration file"),
    project: Optional[str] = typer.Option(None, "-p", "--project-name", help="Project name (defaults to directory name)"),
):
    """List containers belonging to the compose project."""
    try:
        containers = default_client.compose_ps(compose_file=file, project_name=project)
        if not containers:
            console.print("No running compose services found.")
            return

        table = Table(title="Compose Services")
        table.add_column("CONTAINER ID", style="cyan", no_wrap=True)
        table.add_column("IMAGE", style="white")
        table.add_column("GPU", style="magenta")
        table.add_column("STATUS", style="green")
        table.add_column("NAMES", style="yellow")

        for c in containers:
            table.add_row(short_id(c.id), c.image, c.gpu, c.status, c.name)
        console.print(table)
    except Exception as e:
        console.print(f"[bold red]Error:[/bold red] {e}")
        raise typer.Exit(code=1)
