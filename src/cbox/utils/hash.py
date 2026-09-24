"""Hash utilities for Image manifests, Cboxfiles, and Volume tracking."""

import hashlib
import json
import os
from pathlib import Path
from typing import Any, Union


def sha256_text(text: str) -> str:
    """Compute sha256 hex digest of a string."""
    return hashlib.sha256(text.encode("utf-8")).hexdigest()


def sha256_bytes(data: bytes) -> str:
    """Compute sha256 hex digest of raw bytes."""
    return hashlib.sha256(data).hexdigest()


def sha256_file(path: Union[str, Path], chunk_size: int = 65536) -> str:
    """Compute sha256 hex digest of a file content."""
    hasher = hashlib.sha256()
    with open(path, "rb") as f:
        while chunk := f.read(chunk_size):
            hasher.update(chunk)
    return hasher.hexdigest()


def canonical_json_hash(data: Any) -> str:
    """Compute sha256 hex digest of canonical JSON serialized object."""
    dumped = json.dumps(data, sort_keys=True, separators=(",", ":"))
    return sha256_text(dumped)


def compute_directory_fast_hash(dir_path: Union[str, Path]) -> str:
    """Compute fast hash of a directory using relative path, size, and mtime.

    Ideal for large datasets (tens/hundreds of GBs) without reading every byte.
    """
    path = Path(dir_path).resolve()
    if not path.exists():
        raise FileNotFoundError(f"Path {path} does not exist")

    if path.is_file():
        stat = path.stat()
        return sha256_text(f"{path.name}:{stat.st_size}:{int(stat.st_mtime)}")

    entries: list[str] = []
    for root, _, files in os.walk(path):
        rel_dir = os.path.relpath(root, path)
        for f in sorted(files):
            file_path = os.path.join(root, f)
            rel_path = os.path.normpath(os.path.join(rel_dir, f))
            try:
                stat = os.stat(file_path)
                entries.append(f"{rel_path}:{stat.st_size}:{int(stat.st_mtime)}")
            except OSError:
                continue

    return sha256_text("\n".join(entries))


def compute_directory_checksum(dir_path: Union[str, Path]) -> str:
    """Compute full sha256 checksum across all files in directory."""
    path = Path(dir_path).resolve()
    if not path.exists():
        raise FileNotFoundError(f"Path {path} does not exist")

    if path.is_file():
        return sha256_file(path)

    hasher = hashlib.sha256()
    for root, _, files in sorted(os.walk(path)):
        rel_dir = os.path.relpath(root, path)
        for f in sorted(files):
            file_path = os.path.join(root, f)
            rel_path = os.path.normpath(os.path.join(rel_dir, f))
            hasher.update(rel_path.encode("utf-8"))
            try:
                with open(file_path, "rb") as fp:
                    while chunk := fp.read(65536):
                        hasher.update(chunk)
            except OSError:
                continue

    return hasher.hexdigest()


def short_id(full_id: str, length: int = 12) -> str:
    """Return a short prefix of an identifier (e.g. ca84bd93582a)."""
    clean = full_id.removeprefix("sha256:")
    return clean[:length]
