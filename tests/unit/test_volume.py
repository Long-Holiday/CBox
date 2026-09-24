"""Unit tests for Volume parsing, hashing, and manifest creation."""

from pathlib import Path
from cbox.domain.enums import VolumeMode
from cbox.domain.volume import VolumeMount
from cbox.services.volume_service import VolumeService
from cbox.utils.hash import compute_directory_checksum, compute_directory_fast_hash


def test_volume_mount_parsing():
    # Syntax: src:dst:mode
    m1 = VolumeMount.parse("/data/Potsdam:/data:ro")
    assert m1.source == "/data/Potsdam"
    assert m1.target == "/data"
    assert m1.mode == VolumeMode.RO

    # Syntax: src:dst
    m2 = VolumeMount.parse("./runs/exp1:/output")
    assert m2.source == "./runs/exp1"
    assert m2.target == "/output"
    assert m2.mode == VolumeMode.RO  # default

    # Key-value syntax
    m3 = VolumeMount.parse("source=potsdam,target=/data,mode=output")
    assert m3.source == "potsdam"
    assert m3.target == "/data"
    assert m3.mode == VolumeMode.OUTPUT


def test_directory_hashing(tmp_path: Path):
    data_dir = tmp_path / "dataset"
    data_dir.mkdir()
    (data_dir / "file1.txt").write_text("hello")
    (data_dir / "file2.txt").write_text("world")

    fast1 = compute_directory_fast_hash(data_dir)
    check1 = compute_directory_checksum(data_dir)

    assert fast1
    assert check1

    # Modify file content without changing size
    (data_dir / "file1.txt").write_text("hallo")
    check2 = compute_directory_checksum(data_dir)
    assert check1 != check2


def test_volume_service_creation(test_db, tmp_path: Path):
    data_dir = tmp_path / "potsdam"
    data_dir.mkdir()
    (data_dir / "train.png").write_bytes(b"\x89PNG\r\n\x1a\n")

    svc = VolumeService(test_db)
    vol = svc.create_volume("potsdam", data_dir, mode=VolumeMode.RO, immutable=True)
    assert vol.name == "potsdam"
    assert vol.immutable is True
    assert vol.mode == VolumeMode.RO

    fetched = svc.get_volume("potsdam")
    assert fetched.id == vol.id
    assert fetched.hash == vol.hash
