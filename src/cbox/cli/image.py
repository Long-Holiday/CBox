"""CLI commands for image management (build, images, rmi)."""

from pathlib import Path
from typing import Optional
import typer
from rich.console import Console
from rich.table import Table

from cbox.client.api import default_client
from cbox.utils.errors import CBoxError
from cbox.utils.hash import short_id

console = Console()
image_cli = typer.Typer(help="Manage CBox images")


@image_cli.command(name="build")
def build(
    context_dir: str = typer.Argument(".", help="Build context directory containing Cboxfile"),
    tag: Optional[str] = typer.Option(None, "-t", "--tag", help="Image name and optionally a tag (format: name:tag)"),
    file: str = typer.Option("Cboxfile", "-f", "--file", help="Name of the Cboxfile"),
):
    """Build an image blueprint from a Cboxfile."""
    try:
        with console.status("[bold green]Parsing Cboxfile and generating blueprint plan..."):
            manifest = default_client.build_image(context_dir=context_dir, tag=tag, cboxfile=file)
        console.print(f"[bold green]Successfully built image:[/bold green] [cyan]{short_id(manifest.id)}[/cyan]")
        if manifest.tags:
            console.print(f"Tagged as: [bold cyan]{', '.join(manifest.tags)}[/bold cyan]")
    except CBoxError as e:
        console.print(f"[bold red]Error building image:[/bold red] {e}")
        raise typer.Exit(code=e.exit_code)
    except Exception as e:
        console.print(f"[bold red]Error:[/bold red] {e}")
        raise typer.Exit(code=1)


@image_cli.command(name="list")
def list_images():
    """List images available locally."""
    try:
        images = default_client.list_images()
        if not images:
            console.print("No images found.")
            return

        table = Table(title="CBox Images")
        table.add_column("REPOSITORY", style="cyan", no_wrap=True)
        table.add_column("TAG", style="magenta")
        table.add_column("IMAGE ID", style="green")
        table.add_column("CREATED", style="white")

        for img in images:
            for t in img.tags:
                repo, tag_str = t.split(":", 1) if ":" in t else (t, "latest")
                created_str = img.created_at.strftime("%Y-%m-%d %H:%M:%S")
                table.add_row(repo, tag_str, short_id(img.id), created_str)

        console.print(table)
    except Exception as e:
        console.print(f"[bold red]Error listing images:[/bold red] {e}")
        raise typer.Exit(code=1)


@image_cli.command(name="rm")
def remove_image(
    image: str = typer.Argument(..., help="Image ID or repository:tag to remove"),
):
    """Remove one or more images."""
    try:
        default_client.delete_image(image)
        console.print(f"Untagged / deleted: [cyan]{image}[/cyan]")
    except CBoxError as e:
        console.print(f"[bold red]Error:[/bold red] {e}")
        raise typer.Exit(code=e.exit_code)
    except Exception as e:
        console.print(f"[bold red]Error:[/bold red] {e}")
        raise typer.Exit(code=1)
