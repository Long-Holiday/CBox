"""CLI commands for managing contexts and multi-account profiles."""

import typer
from rich.console import Console
from rich.table import Table

from cbox.client.api import default_client
from cbox.utils.errors import CBoxError

console = Console()
context_cli = typer.Typer(help="Manage CBox contexts (accounts / providers)")


@context_cli.command(name="create")
def create_context(
    name: str = typer.Argument(..., help="Context name"),
    provider: str = typer.Option("colab", "-p", "--provider", help="Compute provider (colab, mock)"),
    profile: str = typer.Option("default", "--profile", help="Account profile directory name"),
):
    """Create a new context configuration."""
    try:
        ctx = default_client.create_context(name=name, provider=provider, profile=profile)
        console.print(f"[bold green]Created context:[/bold green] [cyan]{ctx.name}[/cyan] ({ctx.provider})")
    except Exception as e:
        console.print(f"[bold red]Error creating context:[/bold red] {e}")
        raise typer.Exit(code=1)


@context_cli.command(name="ls")
def list_contexts():
    """List all available contexts."""
    try:
        ctxs = default_client.list_contexts()
        if not ctxs:
            console.print("No contexts found.")
            return

        table = Table(title="CBox Contexts")
        table.add_column("CURRENT", style="green", no_wrap=True)
        table.add_column("NAME", style="cyan", no_wrap=True)
        table.add_column("PROVIDER", style="magenta")
        table.add_column("PROFILE", style="white")

        for c in ctxs:
            curr_marker = "*" if c.active else ""
            table.add_row(curr_marker, c.name, c.provider, c.profile)
        console.print(table)
    except Exception as e:
        console.print(f"[bold red]Error listing contexts:[/bold red] {e}")
        raise typer.Exit(code=1)


@context_cli.command(name="use")
def use_context(name: str = typer.Argument(..., help="Context name to activate")):
    """Switch active context."""
    try:
        default_client.use_context(name)
        console.print(f"Switched to context: [bold green]{name}[/bold green]")
    except CBoxError as e:
        console.print(f"[bold red]Error:[/bold red] {e}")
        raise typer.Exit(code=e.exit_code)
    except Exception as e:
        console.print(f"[bold red]Error:[/bold red] {e}")
        raise typer.Exit(code=1)


@context_cli.command(name="rm")
def remove_context(name: str = typer.Argument(..., help="Context name to delete")):
    """Delete a context configuration."""
    try:
        default_client.delete_context(name)
        console.print(f"Deleted context: [cyan]{name}[/cyan]")
    except CBoxError as e:
        console.print(f"[bold red]Error:[/bold red] {e}")
        raise typer.Exit(code=e.exit_code)
    except Exception as e:
        console.print(f"[bold red]Error:[/bold red] {e}")
        raise typer.Exit(code=1)
