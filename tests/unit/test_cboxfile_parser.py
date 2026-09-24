"""Unit tests for Cboxfile parser and Image manifest generation."""

from pathlib import Path
import pytest
from cbox.services.image_service import ImageService
from cbox.utils.errors import ImageBuildError


def test_parse_valid_cboxfile(tmp_path: Path):
    cboxfile = tmp_path / "Cboxfile"
    cboxfile.write_text("""
FROM colab/python:3

APT git rsync
APT libgl1

PIP torch
PIP mmsegmentation

ENV PYTHONPATH=/workspace
WORKDIR /workspace

CMD ["python", "train.py"]
HEALTHCHECK nvidia-smi
""")

    manifest = ImageService.parse_cboxfile(cboxfile)
    assert manifest.id.startswith("sha256:")
    assert manifest.base["image"] == "colab/python:3"
    assert "git" in manifest.apt
    assert "rsync" in manifest.apt
    assert "libgl1" in manifest.apt
    assert "torch" in manifest.pip
    assert "mmsegmentation" in manifest.pip
    assert manifest.environment["PYTHONPATH"] == "/workspace"
    assert manifest.workdir == "/workspace"
    assert manifest.cmd == ["python", "train.py"]
    assert manifest.healthcheck == "nvidia-smi"


def test_deterministic_image_hash(tmp_path: Path):
    cboxfile1 = tmp_path / "Cboxfile1"
    cboxfile2 = tmp_path / "Cboxfile2"
    content = """
FROM colab/python:3
APT git
PIP torch
CMD ["bash"]
"""
    cboxfile1.write_text(content)
    cboxfile2.write_text(content)

    m1 = ImageService.parse_cboxfile(cboxfile1)
    m2 = ImageService.parse_cboxfile(cboxfile2)
    assert m1.id == m2.id


def test_invalid_directive(tmp_path: Path):
    cboxfile = tmp_path / "Cboxfile"
    cboxfile.write_text("""
FROM colab/python:3
INVALID_COMMAND foo bar
""")
    with pytest.raises(ImageBuildError) as exc_info:
        ImageService.parse_cboxfile(cboxfile)
    assert "Unsupported Cboxfile directive" in str(exc_info.value)
