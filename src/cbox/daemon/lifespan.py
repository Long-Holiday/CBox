"""Lifespan event handler managing background engine workers."""

import asyncio
from contextlib import asynccontextmanager
from fastapi import FastAPI

from cbox.services.recovery_service import RecoveryService
from cbox.services.runtime_service import RuntimeService
from cbox.storage.database import default_db
from cbox.utils.config import CBoxConfig
from cbox.utils.paths import default_paths


async def background_health_worker(stop_event: asyncio.Event, config: CBoxConfig):
    """Background task running container health check and auto-recovery cycle."""
    while not stop_event.is_set():
        try:
            with next(default_db.get_session()) as session:
                rec_svc = RecoveryService(session)
                await rec_svc.run_health_check_cycle()
        except Exception:
            pass
        try:
            await asyncio.wait_for(stop_event.wait(), timeout=config.recovery.check_interval)
        except asyncio.TimeoutError:
            pass


async def background_sync_worker(stop_event: asyncio.Event, config: CBoxConfig):
    """Background task performing periodic output volume synchronization."""
    interval = config.sync.output_interval
    while not stop_event.is_set():
        try:
            with next(default_db.get_session()) as session:
                rec_svc = RecoveryService(session)
                await rec_svc.sync_all_active_outputs()
        except Exception:
            pass
        try:
            await asyncio.wait_for(stop_event.wait(), timeout=interval)
        except asyncio.TimeoutError:
            pass


async def background_idle_cleaner(stop_event: asyncio.Event, config: CBoxConfig):
    """Background task cleaning up expired idle runtimes in pool."""
    timeout = config.runtime.idle_timeout
    while not stop_event.is_set():
        try:
            with next(default_db.get_session()) as session:
                rt_svc = RuntimeService(session)
                await rt_svc.cleanup_expired_runtimes(idle_timeout=timeout)
        except Exception:
            pass
        try:
            await asyncio.wait_for(stop_event.wait(), timeout=60.0)
        except asyncio.TimeoutError:
            pass


@asynccontextmanager
async def lifespan(app: FastAPI):
    # Startup: ensure directories and database tables
    default_paths.ensure_directories()
    default_db.init_db()

    config = CBoxConfig.load()
    stop_event = asyncio.Event()

    tasks = [
        asyncio.create_task(background_health_worker(stop_event, config)),
        asyncio.create_task(background_sync_worker(stop_event, config)),
        asyncio.create_task(background_idle_cleaner(stop_event, config)),
    ]

    try:
        yield
    finally:
        # Shutdown
        stop_event.set()
        for t in tasks:
            t.cancel()
        await asyncio.gather(*tasks, return_exceptions=True)
