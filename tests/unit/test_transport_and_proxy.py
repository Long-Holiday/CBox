"""Unit tests for SSH configuration and transfer mode decisions."""

from pathlib import Path
from cbox.domain.enums import TransferMode
from cbox.transport.proxy import decide_transfer_mode
from cbox.transport.ssh import SSHTransport


def test_ssh_config_generation(tmp_path: Path):
    key_file = tmp_path / "worker_key"
    key_file.write_text("dummy-private-key")
    config_file = tmp_path / "test_ssh_config"

    transport = SSHTransport(
        session_name="test-session",
        identity_file=key_file,
        ssh_config_file=config_file,
    )
    transport.write_ssh_config()

    assert config_file.exists()
    content = config_file.read_text()
    assert "Host cbox-test-session" in content
    assert "IdentityFile" in content
    assert "ProxyCommand colab" in content
    assert "ControlMaster auto" in content
    assert "ControlPersist 600" in content


def test_decide_transfer_mode(tmp_path: Path):
    test_dir = tmp_path / "small_files"
    test_dir.mkdir()
    for i in range(10):
        (test_dir / f"f_{i}.txt").write_text("x")

    # Few files -> RSYNC
    mode = decide_transfer_mode(test_dir, configured_mode="auto", threshold_files=5)
    assert mode == TransferMode.TAR_STREAM

    mode2 = decide_transfer_mode(test_dir, configured_mode="auto", threshold_files=50)
    assert mode2 == TransferMode.RSYNC
