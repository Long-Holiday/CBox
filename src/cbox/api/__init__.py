"""API routers package."""

from cbox.api.system import router as system_router
from cbox.api.images import router as images_router
from cbox.api.volumes import router as volumes_router
from cbox.api.contexts import router as contexts_router
from cbox.api.containers import router as containers_router
from cbox.api.compose import router as compose_router

__all__ = [
    "system_router",
    "images_router",
    "volumes_router",
    "contexts_router",
    "containers_router",
    "compose_router",
]
