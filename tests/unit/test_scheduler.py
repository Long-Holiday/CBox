"""Unit tests for GPU matching and affinity scoring in Scheduler."""

from cbox.domain.container import ContainerSpec
from cbox.domain.enums import RuntimeState, VolumeMode
from cbox.domain.runtime import Runtime
from cbox.domain.volume import VolumeMount
from cbox.services.scheduler import Scheduler
from cbox.storage.repositories import RuntimeRepository


def test_scheduler_scoring(test_db):
    repo = RuntimeRepository(test_db)
    scheduler = Scheduler(test_db)

    # Runtime 1: L4 GPU, IDLE, has image cache
    rt1 = Runtime(
        id="rt-1",
        provider_session="sess-1",
        accelerator="L4",
        state=RuntimeState.IDLE,
    )
    repo.save_runtime(rt1)
    repo.add_cache_entry("rt-1", "image", "sha256-img1", "sha256-img1")

    # Runtime 2: T4 GPU, IDLE, no cache
    rt2 = Runtime(
        id="rt-2",
        provider_session="sess-2",
        accelerator="T4",
        state=RuntimeState.IDLE,
    )
    repo.save_runtime(rt2)

    spec = ContainerSpec(
        name="test-cont",
        image="sha256-img1",
        gpu=["L4", "T4"],
        mounts=[VolumeMount(source="data", target="/data", mode=VolumeMode.RO)],
    )

    # Score rt1: GPU match (100) + Image cache (30) + Idle (100) = 230
    score1 = scheduler.score_runtime(rt1, spec, "sha256-img1")
    assert score1 == 230

    # Score rt2: GPU match (100) + Idle (100) = 200
    score2 = scheduler.score_runtime(rt2, spec, "sha256-img1")
    assert score2 == 200

    # Best selection should pick rt1
    best = scheduler.select_best_runtime(spec, "sha256-img1")
    assert best is not None
    assert best.id == "rt-1"
