"""CLI commands for container operations (create, run, start, stop, restart, ps, logs, exec, cp, stats)."""

import json
import os
import sys
import time
from typing import List, Optional
import typer
from rich.console import Console
from rich.table import Table

from cbox.client.api import default_client
from cbox.domain.container import ContainerSpec
from cbox.domain.enums import RestartPolicy
from cbox.domain.volume import VolumeMount
from cbox.utils.errors import CBoxError
from cbox.utils.hash import short_id

console = Console()
container_cli = typer.Typer(help="Manage CBox containers")


def _parse_key_values(items: Optional[List[str]]) -> dict[str, str]:
    res = {}
    if items:
        for item in items:
            if "=" in item:
                k, v = item.split("=", 1)
                res[k.strip()] = v.strip()
    return res


def _parse_mounts(volumes: Optional[List[str]], mounts: Optional[List[str]]) -> list[VolumeMount]:
    res = []
    if volumes:
        for v in volumes:
            res.append(VolumeMount.parse(v))
    if mounts:
        for m in mounts:
            res.append(VolumeMount.parse(m))
    return res


@container_cli.command(
    name="create",
    context_settings={"allow_extra_args": True, "ignore_unknown_options": True},
)
def create_container(
    ctx: typer.Context,
    image: str = typer.Argument(..., help="Image name or ID"),
    command: List[str] = typer.Argument(None, help="Command to run"),
    name: str = typer.Option(..., "--name", help="Assign a name to the container"),
    gpu: str = typer.Option("T4", "--gpu", help="Preferred GPU accelerator(s), e.g. 'L4,T4' or 'T4'"),
    high_mem: bool = typer.Option(False, "--high-mem", help="Request high-memory instance"),
    volume: Optional[List[str]] = typer.Option(None, "-v", "--volume", help="Bind mount a volume (-v src:dst[:mode])"),
    mount: Optional[List[str]] = typer.Option(None, "--mount", help="Mount spec (source=...,target=...,mode=...)"),
    env: Optional[List[str]] = typer.Option(None, "-e", "--env", help="Set environment variable (KEY=VALUE)"),
    secret: Optional[List[str]] = typer.Option(None, "--secret", help="Inject secret from ~/.config/cbox/secrets/"),
    workdir: str = typer.Option("/workspace", "-w", "--workdir", help="Working directory inside container"),
    restart: str = typer.Option("no", "--restart", help="Restart policy (no, on-failure, unless-stopped, always)"),
    resume_command: Optional[str] = typer.Option(None, "--resume-command", help="Command to execute during recovery"),
    context: Optional[str] = typer.Option(None, "--context", help="Compute context profile"),
    auto_remove: bool = typer.Option(False, "--rm", help="Automatically remove the container when it exits"),
):
    """Create a new container without starting it."""
    try:
        gpu_list = [g.strip() for g in gpu.split(",") if g.strip()]
        mounts_list = _parse_mounts(volume, mount)
        env_dict = _parse_key_values(env)
        resume_cmd_list = [resume_command] if resume_command else None
        full_command = (command or []) + ctx.args

        spec = ContainerSpec(
            name=name,
            image=image,
            command=full_command,
            gpu=gpu_list,
            high_mem=high_mem,
            workdir=workdir,
            env=env_dict,
            secrets=secret or [],
            mounts=mounts_list,
            restart_policy=RestartPolicy(restart),
            resume_command=resume_cmd_list,
            context=context,
            auto_remove=auto_remove,
        )

        cid = default_client.create_container(spec)
        console.print(short_id(cid))
    except CBoxError as e:
        console.print(f"[bold red]Error:[/bold red] {e}")
        raise typer.Exit(code=e.exit_code)
    except Exception as e:
        console.print(f"[bold red]Error:[/bold red] {e}")
        raise typer.Exit(code=1)


@container_cli.command(
    name="run",
    context_settings={"allow_extra_args": True, "ignore_unknown_options": True},
)
def run_container(
    ctx: typer.Context,
    image: str = typer.Argument(..., help="Image name or ID"),
    command: List[str] = typer.Argument(None, help="Command to run"),
    name: Optional[str] = typer.Option(None, "--name", help="Assign a name to the container"),
    detach: bool = typer.Option(False, "-d", "--detach", help="Run container in background and print container ID"),
    interactive: bool = typer.Option(False, "-i", "-it", "--interactive", help="Keep STDIN open and allocate pseudo-TTY"),
    gpu: str = typer.Option("T4", "--gpu", help="Preferred GPU accelerator(s), e.g. 'L4,T4' or 'T4'"),
    high_mem: bool = typer.Option(False, "--high-mem", help="Request high-memory instance"),
    volume: Optional[List[str]] = typer.Option(None, "-v", "--volume", help="Bind mount a volume (-v src:dst[:mode])"),
    mount: Optional[List[str]] = typer.Option(None, "--mount", help="Mount spec (source=...,target=...,mode=...)"),
    env: Optional[List[str]] = typer.Option(None, "-e", "--env", help="Set environment variable (KEY=VALUE)"),
    secret: Optional[List[str]] = typer.Option(None, "--secret", help="Inject secret from ~/.config/cbox/secrets/"),
    workdir: str = typer.Option("/workspace", "-w", "--workdir", help="Working directory inside container"),
    restart: str = typer.Option("no", "--restart", help="Restart policy (no, on-failure, unless-stopped, always)"),
    resume_command: Optional[str] = typer.Option(None, "--resume-command", help="Command to execute during recovery"),
    context: Optional[str] = typer.Option(None, "--context", help="Compute context profile"),
    auto_remove: bool = typer.Option(False, "--rm", help="Automatically remove the container when it exits"),
):
    """Run a command in a new container (create + start + attach logs)."""
    import uuid

    cont_name = name or f"cbox_{short_id(str(uuid.uuid4()))}"
    gpu_list = [g.strip() for g in gpu.split(",") if g.strip()]
    mounts_list = _parse_mounts(volume, mount)
    env_dict = _parse_key_values(env)
    resume_cmd_list = [resume_command] if resume_command else None
    full_command = (command or []) + ctx.args

    spec = ContainerSpec(
        name=cont_name,
        image=image,
        command=full_command,
        gpu=gpu_list,
        high_mem=high_mem,
        workdir=workdir,
        env=env_dict,
        secrets=secret or [],
        mounts=mounts_list,
        restart_policy=RestartPolicy(restart),
        resume_command=resume_cmd_list,
        context=context,
        auto_remove=auto_remove,
    )

    try:
        cid = default_client.create_container(spec)

        if detach:
            # Detached mode: start and immediately print container ID
            default_client.start_container(cid)
            console.print(short_id(cid))
            return

        # Attached mode: display status and follow logs
        with console.status(f"[bold green]Provisioning runtime and starting {cont_name}..."):
            default_client.start_container(cid)

        console.print(f"[bold green]Container started:[/bold green] [cyan]{short_id(cid)}[/cyan] ({cont_name})")

        # Stream logs until container exits
        last_log_len = 0
        while True:
            time.sleep(2.0)
            logs = default_client.get_logs(cid, tail=500)
            if len(logs) > last_log_len:
                sys.stdout.write(logs[last_log_len:])
                sys.stdout.flush()
                last_log_len = len(logs)

            inspect = default_client.inspect_container(cid)
            if inspect.state.status.value in ("exited", "failed", "stopped"):
                exit_code = inspect.state.exit_code or 0
                if auto_remove:
                    default_client.remove_container(cid)
                raise typer.Exit(code=exit_code)

    except CBoxError as e:
        console.print(f"[bold red]Error running container:[/bold red] {e}")
        raise typer.Exit(code=e.exit_code)


@container_cli.command(name="start")
def start_container(identifier: str = typer.Argument(..., help="Container ID or name")):
    """Start one or more stopped containers."""
    try:
        with console.status(f"[bold green]Starting container '{identifier}'..."):
            default_client.start_container(identifier)
        console.print(f"[bold green]Started:[/bold green] [cyan]{identifier}[/cyan]")
    except CBoxError as e:
        console.print(f"[bold red]Error starting container:[/bold red] {e}")
        raise typer.Exit(code=e.exit_code)


@container_cli.command(name="stop")
def stop_container(
    identifier: str = typer.Argument(..., help="Container ID or name"),
    timeout: int = typer.Option(10, "-t", "--time", help="Seconds to wait before killing process"),
):
    """Stop a running container and synchronize output volumes."""
    try:
        with console.status(f"[bold green]Stopping container '{identifier}' and synchronizing outputs..."):
            default_client.stop_container(identifier, timeout=timeout)
        console.print(f"[bold green]Stopped:[/bold green] [cyan]{identifier}[/cyan]")
    except CBoxError as e:
        console.print(f"[bold red]Error stopping container:[/bold red] {e}")
        raise typer.Exit(code=e.exit_code)


@container_cli.command(name="restart")
def restart_container(identifier: str = typer.Argument(..., help="Container ID or name")):
    """Restart a container."""
    try:
        with console.status(f"[bold green]Restarting container '{identifier}'..."):
            default_client.restart_container(identifier)
        console.print(f"[bold green]Restarted:[/bold green] [cyan]{identifier}[/cyan]")
    except CBoxError as e:
        console.print(f"[bold red]Error restarting container:[/bold red] {e}")
        raise typer.Exit(code=e.exit_code)


@container_cli.command(name="rm")
def remove_container(
    identifier: str = typer.Argument(..., help="Container ID or name"),
    force: bool = typer.Option(False, "-f", "--force", help="Force the removal of a running container"),
):
    """Remove one or more containers."""
    try:
        default_client.remove_container(identifier, force=force)
        console.print(f"Removed container: [cyan]{identifier}[/cyan]")
    except CBoxError as e:
        console.print(f"[bold red]Error removing container:[/bold red] {e}")
        raise typer.Exit(code=e.exit_code)


@container_cli.command(name="ps")
def list_containers(all: bool = typer.Option(False, "-a", "--all", help="Show all containers (default shows just running)")):
    """List containers conforming to plan.md section 31."""
    try:
        containers = default_client.list_containers(all_containers=all)
        if not containers:
            console.print("No containers found.")
            return

        table = Table(box=None, header_style="bold white")
        table.add_column("CONTAINER ID", style="cyan", no_wrap=True)
        table.add_column("IMAGE", style="white")
        table.add_column("GPU", style="magenta")
        table.add_column("STATUS", style="green")
        table.add_column("NAMES", style="yellow")

        for c in containers:
            status_style = "green" if c.state.value == "running" else "white"
            table.add_row(
                short_id(c.id),
                short_id(c.image),
                c.gpu,
                f"[{status_style}]{c.status}[/{status_style}]",
                c.name,
            )
        console.print(table)
    except Exception as e:
        console.print(f"[bold red]Error listing containers:[/bold red] {e}")
        raise typer.Exit(code=1)


@container_cli.command(name="inspect")
def inspect_container(identifier: str = typer.Argument(..., help="Container ID or name")):
    """Return low-level information on CBox objects (plan.md section 32)."""
    try:
        inspect = default_client.inspect_container(identifier)
        console.print_json(inspect.model_dump_json(indent=2))
    except CBoxError as e:
        console.print(f"[bold red]Error inspecting container:[/bold red] {e}")
        raise typer.Exit(code=e.exit_code)


@container_cli.command(name="logs")
def get_logs(
    identifier: str = typer.Argument(..., help="Container ID or name"),
    follow: bool = typer.Option(False, "-f", "--follow", help="Follow log output"),
    tail: int = typer.Option(100, "-n", "--tail", help="Number of lines to show from the end of the logs"),
):
    """Fetch the logs of a container."""
    try:
        if not follow:
            logs = default_client.get_logs(identifier, tail=tail)
            console.print(logs, end="")
            return

        # Follow mode
        last_len = 0
        while True:
            logs = default_client.get_logs(identifier, tail=500)
            if len(logs) > last_len:
                sys.stdout.write(logs[last_len:])
                sys.stdout.flush()
                last_len = len(logs)
            time.sleep(1.0)
    except KeyboardInterrupt:
        pass
    except CBoxError as e:
        console.print(f"[bold red]Error fetching logs:[/bold red] {e}")
        raise typer.Exit(code=e.exit_code)


@container_cli.command(
    name="exec",
    context_settings={"allow_extra_args": True, "ignore_unknown_options": True},
)
def exec_container(
    ctx: typer.Context,
    identifier: str = typer.Argument(..., help="Container ID or name"),
    command: List[str] = typer.Argument(..., help="Command and arguments to execute"),
    interactive: bool = typer.Option(False, "-i", "-it", "--interactive", help="Interactive TTY mode"),
):
    """Run a command in a running container."""
    try:
        full_command = command + ctx.args
        code, stdout, stderr = default_client.exec_command(identifier, full_command)
        if stdout:
            sys.stdout.write(stdout)
        if stderr:
            sys.stderr.write(stderr)
        raise typer.Exit(code=code)
    except typer.Exit:
        raise
    except CBoxError as e:
        console.print(f"[bold red]Error executing command:[/bold red] {e}")
        raise typer.Exit(code=e.exit_code)


@container_cli.command(name="stats")
def stats_container(
    identifier: Optional[str] = typer.Argument(None, help="Optional container ID or name to inspect"),
):
    """Display a live stream of container(s) resource usage statistics (plan.md section 33)."""
    try:
        stats = default_client.get_stats(identifier)
        if not stats:
            console.print("No stats available for running containers.")
            return

        table = Table(title="CBox Live Resource Stats")
        table.add_column("NAME", style="cyan", no_wrap=True)
        table.add_column("GPU", style="magenta")
        table.add_column("GPU MEM", style="white")
        table.add_column("GPU UTIL", style="yellow")
        table.add_column("CPU", style="white")
        table.add_column("RAM", style="green")

        for s in stats:
            gpu_mem = f"{s.get('gpu_mem_used_mb', 0) / 1024:.1f}/{s.get('gpu_mem_total_mb', 0) / 1024:.1f} GB"
            gpu_util = f"{s.get('gpu_util_percent', 0)}%"
            cpu_util = f"{s.get('cpu_util_percent', 0)}%"
            ram = f"{s.get('ram_used_mb', 0) / 1024:.1f} GB"
            table.add_row(
                s.get("name", "unknown"),
                s.get("gpu", "None"),
                gpu_mem,
                gpu_util,
                cpu_util,
                ram,
            )
        console.print(table)
    except Exception as e:
        console.print(f"[bold red]Error retrieving stats:[/bold red] {e}")
        raise typer.Exit(code=1)


@container_cli.command(name="cp")
def copy_file(
    src: str = typer.Argument(..., help="Source path (e.g. ./local.file or container:/workspace/file)"),
    dest: str = typer.Argument(..., help="Destination path"),
):
    """Copy files/folders between a container and the local filesystem."""
    try:
        # container:path syntax
        if ":" in src:
            cid, rem_path = src.split(":", 1)
            # Pull via exec/transport or client
            console.print(f"[bold green]Copying {src} -> {dest}...[/bold green]")
            # Use client inspect to find runtime transport
            inspect = default_client.inspect_container(cid)
            rt_session = inspect.runtime.get("Session")
            if not rt_session:
                raise CBoxError("Container runtime is not active")
            # Pull with rsync
            cmd = f"rsync -avz cbox-{rt_session}:{rem_path} {dest}"
            os.system(cmd)
        elif ":" in dest:
            cid, rem_path = dest.split(":", 1)
            console.print(f"[bold green]Copying {src} -> {dest}...[/bold green]")
            inspect = default_client.inspect_container(cid)
            rt_session = inspect.runtime.get("Session")
            if not rt_session:
                raise CBoxError("Container runtime is not active")
            cmd = f"rsync -avz {src} cbox-{rt_session}:{rem_path}"
            os.system(cmd)
        else:
            raise CBoxError("One of source or destination must be in 'container:path' format")
    except Exception as e:
        console.print(f"[bold red]Error during cp:[/bold red] {e}")
        raise typer.Exit(code=1)
