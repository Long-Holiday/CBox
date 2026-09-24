"""Integration tests for CBox CLI commands using Typer CliRunner."""

from pathlib import Path
from typer.testing import CliRunner
from cbox.cli.main import cli_app
from cbox.storage.database import default_db
from cbox.utils.paths import default_paths

runner = CliRunner()


def setup_module():
    default_paths.ensure_directories()
    default_db.init_db()


def test_cli_version():
    res = runner.invoke(cli_app, ["version"])
    assert res.exit_code == 0
    assert "0.1.0" in res.output


def test_cli_system_info():
    res = runner.invoke(cli_app, ["system", "info"])
    assert res.exit_code == 0
    assert "state_dir" in res.output


def test_cli_context_lifecycle():
    # 1. Create context
    res1 = runner.invoke(cli_app, ["context", "create", "test-cli-ctx", "-p", "mock", "--profile", "cli-prof"])
    assert res1.exit_code == 0
    assert "Created context" in res1.output

    # 2. List contexts
    res2 = runner.invoke(cli_app, ["context", "ls"])
    assert res2.exit_code == 0
    assert "test-cli-ctx" in res2.output

    # 3. Use context
    res3 = runner.invoke(cli_app, ["context", "use", "test-cli-ctx"])
    assert res3.exit_code == 0
    assert "Switched to context" in res3.output

    # 4. Remove context
    res4 = runner.invoke(cli_app, ["context", "rm", "test-cli-ctx"])
    assert res4.exit_code == 0


def test_cli_volume_lifecycle(tmp_path: Path):
    vol_dir = tmp_path / "cli_vol_data"
    vol_dir.mkdir()
    (vol_dir / "item.txt").write_text("content")

    # 1. Create volume
    res1 = runner.invoke(cli_app, ["volume", "create", "cli-vol", "-s", str(vol_dir), "-m", "ro"])
    assert res1.exit_code == 0
    assert "Volume created" in res1.output

    # 2. List volumes
    res2 = runner.invoke(cli_app, ["volume", "ls"])
    assert res2.exit_code == 0
    assert "cli-vol" in res2.output

    # 3. Inspect volume
    res3 = runner.invoke(cli_app, ["volume", "inspect", "cli-vol"])
    assert res3.exit_code == 0
    assert "cli-vol" in res3.output

    # 4. Remove volume
    res4 = runner.invoke(cli_app, ["volume", "rm", "cli-vol"])
    assert res4.exit_code == 0


def test_cli_image_build_and_list(tmp_path: Path):
    ctx_dir = tmp_path / "build_test"
    ctx_dir.mkdir()
    (ctx_dir / "Cboxfile").write_text("""
FROM colab/python:3
APT git
WORKDIR /workspace
CMD ["python3", "-V"]
""")

    # 1. Build
    res1 = runner.invoke(cli_app, ["build", "-t", "cli-test:latest", str(ctx_dir)])
    assert res1.exit_code == 0
    assert "Successfully built image" in res1.output

    # 2. List
    res2 = runner.invoke(cli_app, ["images"])
    assert res2.exit_code == 0
    assert "cli-test" in res2.output


def test_cli_container_create_ps_rm(tmp_path: Path):
    ctx_dir = tmp_path / "img_ctx"
    ctx_dir.mkdir()
    (ctx_dir / "Cboxfile").write_text("""
FROM colab/python:3
CMD ["bash"]
""")
    runner.invoke(cli_app, ["build", "-t", "cont-base:v1", str(ctx_dir)])

    # 1. Create container
    res1 = runner.invoke(cli_app, ["create", "--name", "cli-cont-1", "cont-base:v1", "--gpu", "T4"])
    assert res1.exit_code == 0
    cid = res1.output.strip()

    # 2. List (ps -a)
    res2 = runner.invoke(cli_app, ["ps", "-a"])
    assert res2.exit_code == 0
    assert "cli-cont-1" in res2.output

    # 3. Inspect
    res3 = runner.invoke(cli_app, ["inspect", "cli-cont-1"])
    assert res3.exit_code == 0
    assert "cli-cont-1" in res3.output

    # 4. Remove
    res4 = runner.invoke(cli_app, ["rm", "cli-cont-1"])
    assert res4.exit_code == 0
