"""Integration tests for database repositories."""

from cbox.domain.context import Context
from cbox.domain.enums import ContainerState
from cbox.domain.image import ImageManifest
from cbox.domain.volume import VolumeManifest
from cbox.storage.repositories import (
    ContainerRepository,
    ContextRepository,
    EventRepository,
    ImageRepository,
    VolumeRepository,
)


def test_image_and_tag_repository(test_db):
    repo = ImageRepository(test_db)
    manifest = ImageManifest(
        id="sha256:abc123456789",
        tags=["mmseg:latest", "mmseg:v1"],
        base={"provider": "colab"},
        cmd=["bash"],
    )
    repo.save_image(manifest)

    # Lookup by ID
    by_id = repo.get_by_id_or_tag("sha256:abc123456789")
    assert by_id is not None
    assert by_id.id == manifest.id

    # Lookup by tag
    by_tag = repo.get_by_id_or_tag("mmseg:v1")
    assert by_tag is not None
    assert by_tag.id == manifest.id

    images = repo.list_images()
    assert len(images) == 1


def test_container_repository(test_db):
    repo = ContainerRepository(test_db)
    model = repo.save_container(
        id="cid-001",
        name="experiment-1",
        image_id="sha256:abc123456789",
        command=["python", "train.py"],
        workdir="/workspace",
        gpu_requested=["L4"],
        restart_policy="unless-stopped",
    )
    assert model.id == "cid-001"
    assert model.state == "created"

    # Update state
    repo.update_state("cid-001", ContainerState.RUNNING, pid=12345, gpu_assigned="L4")
    updated = repo.get_by_id_or_name("cid-001")
    assert updated.state == "running"
    assert updated.pid == 12345
    assert updated.gpu_assigned == "L4"


def test_context_repository(test_db):
    repo = ContextRepository(test_db)
    ctx1 = Context(name="colab-main", provider="colab", profile="account-a")
    ctx2 = Context(name="colab-alt", provider="colab", profile="account-b")
    repo.save_context(ctx1)
    repo.save_context(ctx2)

    repo.set_active("colab-alt")
    active = repo.get_active()
    assert active is not None
    assert active.name == "colab-alt"
