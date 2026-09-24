"""Integration tests for FastAPI REST API endpoints."""

from fastapi.testclient import TestClient
from cbox.daemon.app import app
from cbox.storage.database import default_db
from cbox.utils.paths import default_paths

default_paths.ensure_directories()
default_db.init_db()

client = TestClient(app)


def test_api_version():
    res = client.get("/version")
    assert res.status_code == 200
    data = res.json()
    assert data["version"] == "0.1.0"
    assert data["api_version"] == "v1"


def test_api_system_info():
    res = client.get("/system/info")
    assert res.status_code == 200
    data = res.json()
    assert "state_dir" in data
    assert "db_path" in data


def test_api_contexts():
    # Create context
    res = client.post("/contexts", json={"name": "test-ctx", "provider": "mock", "profile": "test"})
    assert res.status_code == 200

    # List contexts
    res2 = client.get("/contexts")
    assert res2.status_code == 200
    assert any(c["name"] == "test-ctx" for c in res2.json())

    # Use context
    res3 = client.post("/contexts/test-ctx/use")
    assert res3.status_code == 200

    # Delete context
    res4 = client.delete("/contexts/test-ctx")
    assert res4.status_code == 200
