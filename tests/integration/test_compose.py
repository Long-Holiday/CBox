"""Integration tests for CBox Compose parser and lifecycle."""

from pathlib import Path
import pytest
from cbox.compose.parser import parse_compose_file, compose_service_to_spec
from cbox.compose.service import ComposeService


def test_parse_compose_file(tmp_path: Path):
    compose_yaml = tmp_path / "cbox-compose.yaml"
    compose_yaml.write_text("""
version: "1"

services:
  train:
    image: test:latest
    gpu:
      preference:
        - L4
        - T4
    volumes:
      - source: /data/input
        target: /data
        mode: ro
      - source: ./output
        target: /output
        mode: output
    command:
      - python
      - train.py
    restart: unless-stopped
    resume_command: python train.py --resume
""")

    parsed = parse_compose_file(compose_yaml)
    assert parsed.version == "1"
    assert "train" in parsed.services
    svc = parsed.services["train"]
    assert svc.image == "test:latest"
    assert svc.restart == "unless-stopped"

    spec = compose_service_to_spec("train", svc, "myproject")
    assert spec.name == "myproject_train"
    assert spec.gpu == ["L4", "T4"]
    assert len(spec.mounts) == 2
    assert spec.resume_command == ["python train.py --resume"]


@pytest.mark.asyncio
async def test_compose_up_down_mock(services, tmp_path: Path):
    img_svc = services["image"]
    ctx_dir = tmp_path / "build"
    ctx_dir.mkdir()
    (ctx_dir / "Cboxfile").write_text("FROM colab/python:3\nCMD ['bash']")
    img_svc.build_image(ctx_dir, tag="myimage:latest")

    data_dir = tmp_path / "data"
    data_dir.mkdir()
    out_dir = tmp_path / "out"
    out_dir.mkdir()

    compose_yaml = tmp_path / "cbox-compose.yaml"
    compose_yaml.write_text(f"""
version: "1"
services:
  s1:
    image: myimage:latest
    command: ["python3", "-c", "import time; time.sleep(60)"]
    volumes:
      - source: {data_dir}
        target: /data
        mode: ro
      - source: {out_dir}
        target: /output
        mode: output
""")

    compose_svc = ComposeService(services["db"], container_service=services["container"])

    # 1. Compose Up
    started = await compose_svc.up(compose_file=compose_yaml, project_name="testproj")
    assert len(started) == 1

    # 2. Compose PS
    ps_list = compose_svc.ps(compose_file=compose_yaml, project_name="testproj")
    assert len(ps_list) == 1
    assert ps_list[0].name == "testproj_s1"

    # 3. Compose Down
    stopped = await compose_svc.down(compose_file=compose_yaml, project_name="testproj")
    assert len(stopped) == 1
    assert stopped[0] == "testproj_s1"
