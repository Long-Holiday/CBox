"""Async shell execution utilities for CBox."""

import asyncio
import os
import shutil
from typing import AsyncGenerator, Callable, Optional


class ProcessResult:
    """Outcome of an executed command."""

    def __init__(self, returncode: int, stdout: str, stderr: str):
        self.returncode = returncode
        self.stdout = stdout
        self.stderr = stderr

    @property
    def success(self) -> bool:
        return self.returncode == 0


async def run_command_async(
    cmd: list[str],
    cwd: Optional[str] = None,
    env: Optional[dict[str, str]] = None,
    timeout: Optional[float] = None,
    input_text: Optional[str] = None,
) -> ProcessResult:
    """Execute a command asynchronously and return returncode, stdout, and stderr."""
    merged_env = os.environ.copy()
    if env:
        merged_env.update(env)

    proc = await asyncio.create_subprocess_exec(
        *cmd,
        cwd=cwd,
        env=merged_env,
        stdin=asyncio.subprocess.PIPE if input_text is not None else None,
        stdout=asyncio.subprocess.PIPE,
        stderr=asyncio.subprocess.PIPE,
    )

    stdin_bytes = input_text.encode("utf-8") if input_text is not None else None
    try:
        stdout_bytes, stderr_bytes = await asyncio.wait_for(
            proc.communicate(stdin_bytes), timeout=timeout
        )
    except asyncio.TimeoutError:
        try:
            proc.kill()
        except OSError:
            pass
        raise TimeoutError(f"Command {' '.join(cmd)} timed out after {timeout} seconds")

    return ProcessResult(
        returncode=proc.returncode if proc.returncode is not None else -1,
        stdout=stdout_bytes.decode("utf-8", errors="replace"),
        stderr=stderr_bytes.decode("utf-8", errors="replace"),
    )


async def stream_command_async(
    cmd: list[str],
    cwd: Optional[str] = None,
    env: Optional[dict[str, str]] = None,
    on_stdout: Optional[Callable[[str], None]] = None,
    on_stderr: Optional[Callable[[str], None]] = None,
) -> int:
    """Stream process stdout and stderr line-by-line."""
    merged_env = os.environ.copy()
    if env:
        merged_env.update(env)

    proc = await asyncio.create_subprocess_exec(
        *cmd,
        cwd=cwd,
        env=merged_env,
        stdout=asyncio.subprocess.PIPE,
        stderr=asyncio.subprocess.PIPE,
    )

    async def read_stream(stream: asyncio.StreamReader, callback: Optional[Callable[[str], None]]):
        while True:
            line = await stream.readline()
            if not line:
                break
            text = line.decode("utf-8", errors="replace").rstrip("\r\n")
            if callback:
                callback(text)

    await asyncio.gather(
        read_stream(proc.stdout, on_stdout),  # type: ignore
        read_stream(proc.stderr, on_stderr),  # type: ignore
    )
    return await proc.wait()


def check_tool_available(tool_name: str) -> bool:
    """Check if an executable is found in system PATH."""
    return shutil.which(tool_name) is not None
