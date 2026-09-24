"""FastAPI application factory for cboxd daemon."""

from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

from cbox import __version__
from cbox.api.compose import router as compose_router
from cbox.api.containers import router as containers_router
from cbox.api.contexts import router as contexts_router
from cbox.api.images import router as images_router
from cbox.api.system import router as system_router
from cbox.api.volumes import router as volumes_router
from cbox.daemon.lifespan import lifespan


def create_app() -> FastAPI:
    """Create and configure FastAPI engine application."""
    app = FastAPI(
        title="CBox Engine Daemon",
        description="Docker-like Colab GPU Runtime Engine REST API",
        version=__version__,
        lifespan=lifespan,
    )

    app.add_middleware(
        CORSMiddleware,
        allow_origins=["*"],
        allow_credentials=True,
        allow_methods=["*"],
        allow_headers=["*"],
    )

    # Register API routers
    app.include_router(system_router)
    app.include_router(images_router)
    app.include_router(containers_router)
    app.include_router(volumes_router)
    app.include_router(contexts_router)
    app.include_router(compose_router)

    return app


app = create_app()
