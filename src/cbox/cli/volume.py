"""CLI commands for volume management."""

import json
from typing import Optional
import typer
from rich.console import Console
from rich.table import Table

from cbox.client.api import default_client
from cbox.utils.errors import CBoxError

console = Console()
volume_cli = typer.Typer(help="Manage CBox volumes")


@volume_cli.command(name="create")
def create_volume(
    name: str = typer.Argument(..., help="Volume name"),
    source: str = typer.Option(..., "-s", "--source", help="Source directory on local host"),
    mode: str = typer.Option("ro", "-m", "--mode", help="Volume mode: ro, rw, output, cache"),
    immutable: bool = typer.Option(False, "--immutable", help="Mark dataset volume as immutable"),
    checksum: bool = typer.Option(False, "--checksum", help="Use strict sha256 checksum instead of fast mtime/size hash"),
):
    """Create a new managed named volume."""
    try:
        with console.status(f"[bold green]Hashing and registering volume '{name}'..."):
            vol = default_client.create_volume(
                name=name,
                source=source,
                mode=mode,
                immutable=immutable,
                checksum=checksum,
            )
        console.print(f"[bold green]Volume created:[/bold green] [cyan]{vol.name}[/cyan] (hash: {vol.hash[:12]})")
    except CBoxError as e:
        console.print(f"[bold red]Error creating volume:[/bold red] {e}")
        raise typer.Exit(code=e.exit_code)
    except Exception as e:
        console.print(f"[bold red]Error:[/bold red] {e}")
        raise typer.Exit(code=1)


@volume_cli.command(name="ls")
def list_volumes():
    """List all managed volumes."""
    try:
        vols = default_client.list_volumes()
        if not vols:
            console.print("No volumes found.")
            return

        table = Table(title="CBox Volumes")
        table.add_column("NAME", style="cyan", no_wrap=True)
        table.add_column("SOURCE", style="white")
        table.add_column("MODE", style="magenta")
        table.add_column("IMMUTABLE", style="yellow")
        table.add_column("VERSION", style="green")

        for v in vols:
            table.add_row(
                v.name,
                v.source,
                v.mode.value,
                "yes" if v.immutable else "no",
                str(v.version),
            )
        console.print(table)
    except Exception as e:
        console.print(f"[bold red]Error listing volumes:[/bold red] {e}")
        raise typer.Exit(code=1)


@volume_cli.command(name="inspect")
def inspect_volume(name: str = typer.Argument(..., help="Volume name")):
    """Display detailed volume information."""
    try:
        vol = default_client._get_client().get(f"/volumes/{name}").json()
        console.print_json(json.dumps(vol, default=str))
    except Exception as e:
        console.print(f"[bold red]Error inspecting volume:[/bold red] {e}")
        raise typer.Exit(code=1)


@volume_cli.command(name="rm")
def remove_volume(name: str = typer.Argument(..., help="Volume name to delete")):
    """Delete a managed volume."""
    try:
        default_client.delete_volume(name)
        console.print(f"Deleted volume: [cyan]{name}[/cyan]")
    except CBoxError as e:
        console.print(f"[bold red]Error deleting volume:[/bold red] {e}")
        raise typer.Exit(code=e.exit_code)
    except Exception as e:
        console.print(f"[bold red]Error:[/bold red] {e}")
        raise typer.Exit(code=1)
