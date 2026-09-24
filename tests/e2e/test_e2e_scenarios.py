"""End-to-End lifecycle scenario tests simulating plan.md section 72."""

from pathlib import Path
import pytest
from cbox.domain.container import ContainerSpec
from cbox.domain.enums import ContainerState, RestartPolicy, RuntimeState, VolumeMode
from cbox.domain.volume import VolumeMount


@pytest.mark.asyncio
async def test_complete_e2e_lifecycle_and_recovery(services, tmp_path: Path):
    img_svc = services["image"]
    vol_svc = services["volume"]
    cont_svc = services["container"]
    rec_svc = services["recovery"]
    mock_prov = services["provider"]

    # 1. Build image from Cboxfile
    ctx_dir = tmp_path / "build_ctx"
    ctx_dir.mkdir()
    (ctx_dir / "Cboxfile").write_text("""
FROM colab/python:3
APT git
WORKDIR /workspace
CMD ["python3", "-c", "print('hello from cbox')"]
""")
    manifest = img_svc.build_image(ctx_dir, tag="test:latest")
    assert manifest.id.startswith("sha256:")

    # 2. Prepare dataset and output volumes
    data_dir = tmp_path / "dataset"
    data_dir.mkdir()
    (data_dir / "sample.txt").write_text("dataset-data")

    out_dir = tmp_path / "runs"
    out_dir.mkdir()

    vol = vol_svc.create_volume("test-data", data_dir, mode=VolumeMode.RO, immutable=True)

    # 3. Create Container
    mounts = [
        VolumeMount(source="test-data", target="/data", mode=VolumeMode.RO),
        VolumeMount(source=str(out_dir), target="/output", mode=VolumeMode.OUTPUT),
    ]

    spec = ContainerSpec(
        name="training-exp",
        image=manifest.id,
        command=["python3", "-c", "import time; print('step 1'); time.sleep(60)"],
        gpu=["T4"],
        workdir="/workspace",
        mounts=mounts,
        restart_policy=RestartPolicy.UNLESS_STOPPED,
        resume_command=["python3", "-c", "print('resumed training'); sys.exit(0)"],
    )

    cid = cont_svc.create_container(spec)
    assert cid

    # 4. Start Container
    await cont_svc.start_container(cid)
    inspect = cont_svc.inspect_container(cid)
    assert inspect.state.status == ContainerState.RUNNING

    # 5. Exec command in container
    ret, stdout, _ = await cont_svc.exec_in_container(cid, ["python3", "-c", "print('EXEC_SUCCESS')"])
    assert ret == 0
    assert "EXEC_SUCCESS" in stdout

    # 6. Retrieve stats
    stats = await cont_svc.get_stats(cid)
    assert len(stats) >= 1
    assert stats[0]["name"] == "training-exp"

    # 7. Check logs
    logs = await cont_svc.get_logs(cid)
    assert logs is not None

    # 8. Simulate runtime lost and auto-recovery
    model = cont_svc.repo.get_by_id_or_name(cid)
    rt = services["runtime"].repo.get_by_id(model.runtime_id)
    # Force kill runtime state in both provider memory and database
    rt.state = RuntimeState.LOST
    services["runtime"].repo.save_runtime(rt)
    if model.runtime_id in mock_prov.runtimes:
        mock_prov.runtimes[model.runtime_id].state = RuntimeState.LOST

    # Trigger recovery cycle
    await rec_svc.run_health_check_cycle()

    # Verify container has recovered and is back to RUNNING under a new runtime
    recovered = cont_svc.repo.get_by_id_or_name(cid)
    assert recovered.state == ContainerState.RUNNING, f"Recovery failed with error: {recovered.error_message}"

    # 9. Stop and remove container
    await cont_svc.stop_container(cid)
    stopped_inspect = cont_svc.inspect_container(cid)
    assert stopped_inspect.state.status == ContainerState.STOPPED

    removed = cont_svc.remove_container(cid)
    assert removed is True
