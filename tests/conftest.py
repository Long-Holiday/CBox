"""Pytest fixtures and test environment configuration."""

import os
import shutil
import tempfile
from pathlib import Path
import pytest
from sqlalchemy import create_engine
from sqlalchemy.orm import sessionmaker

from cbox.domain.enums import RuntimeState
from cbox.domain.runtime import Runtime
from cbox.providers.mock import MockProvider
from cbox.services.container_service import ContainerService
from cbox.services.image_service import ImageService
from cbox.services.recovery_service import RecoveryService
from cbox.services.runtime_service import RuntimeService
from cbox.services.volume_service import VolumeService
from cbox.storage.database import Base
from cbox.utils.paths import CBoxPaths


@pytest.fixture
def tmp_workspace(tmp_path: Path):
    """Temporary test directory simulating XDG home."""
    cbox_dir = tmp_path / "cbox"
    cbox_dir.mkdir(parents=True, exist_ok=True)
    return cbox_dir


@pytest.fixture
def test_db(tmp_workspace: Path):
    """Temporary SQLite database session."""
    db_file = tmp_workspace / "test.db"
    engine = create_engine(f"sqlite:///{db_file}", echo=False)
    Base.metadata.create_all(bind=engine)
    SessionClass = sessionmaker(bind=engine)
    session = SessionClass()
    yield session
    session.close()


@pytest.fixture
def mock_provider(tmp_workspace: Path):
    """Mock compute provider."""
    return MockProvider(base_dir=tmp_workspace / "mock_runtimes")


@pytest.fixture
def services(test_db, mock_provider):
    """Initialized service suite using mock provider and test DB."""
    img_svc = ImageService(test_db)
    vol_svc = VolumeService(test_db)
    rt_svc = RuntimeService(test_db, provider=mock_provider)
    cont_svc = ContainerService(
        test_db,
        image_service=img_svc,
        volume_service=vol_svc,
        runtime_service=rt_svc,
    )
    rec_svc = RecoveryService(
        test_db,
        container_service=cont_svc,
        image_service=img_svc,
        volume_service=vol_svc,
        runtime_service=rt_svc,
    )
    return {
        "image": img_svc,
        "volume": vol_svc,
        "runtime": rt_svc,
        "container": cont_svc,
        "recovery": rec_svc,
        "provider": mock_provider,
        "db": test_db,
    }
