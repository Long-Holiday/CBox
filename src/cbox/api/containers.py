"""Container management API endpoints."""

from typing import Any, Optional
from fastapi import APIRouter, Depends, HTTPException, Query
from pydantic import BaseModel
from sqlalchemy.orm import Session

from cbox.domain.container import ContainerInspect, ContainerSpec, ContainerSummary
from cbox.services.container_service import ContainerService
from cbox.storage.database import default_db
from cbox.utils.errors import CBoxError, UsageError

router = APIRouter(prefix="/containers", tags=["containers"])


class ExecRequest(BaseModel):
    command: list[str]


@router.post("/create")
def create_container(spec: ContainerSpec, session: Session = Depends(default_db.get_session)):
    svc = ContainerService(session)
    try:
        cid = svc.create_container(spec)
        return {"id": cid, "name": spec.name}
    except CBoxError as e:
        raise HTTPException(status_code=400, detail=str(e))


@router.post("/{id}/start")
async def start_container(id: str, session: Session = Depends(default_db.get_session)):
    svc = ContainerService(session)
    try:
        await svc.start_container(id)
        return {"status": "started", "id": id}
    except CBoxError as e:
        raise HTTPException(status_code=400, detail=str(e))


@router.post("/{id}/stop")
async def stop_container(id: str, timeout: int = 10, session: Session = Depends(default_db.get_session)):
    svc = ContainerService(session)
    try:
        await svc.stop_container(id, timeout=timeout)
        return {"status": "stopped", "id": id}
    except CBoxError as e:
        raise HTTPException(status_code=400, detail=str(e))


@router.post("/{id}/restart")
async def restart_container(id: str, session: Session = Depends(default_db.get_session)):
    svc = ContainerService(session)
    try:
        await svc.restart_container(id)
        return {"status": "restarted", "id": id}
    except CBoxError as e:
        raise HTTPException(status_code=400, detail=str(e))


@router.delete("/{id}")
def delete_container(id: str, force: bool = False, session: Session = Depends(default_db.get_session)):
    svc = ContainerService(session)
    try:
        success = svc.remove_container(id, force=force)
        if not success:
            raise HTTPException(status_code=404, detail=f"Container {id} not found")
        return {"status": "removed", "id": id}
    except UsageError as e:
        raise HTTPException(status_code=409, detail=str(e))


@router.get("", response_model=list[ContainerSummary])
def list_containers(all: bool = False, session: Session = Depends(default_db.get_session)):
    svc = ContainerService(session)
    return svc.list_containers(all_containers=all)


@router.get("/{id}", response_model=ContainerInspect)
def inspect_container(id: str, session: Session = Depends(default_db.get_session)):
    svc = ContainerService(session)
    try:
        return svc.inspect_container(id)
    except CBoxError as e:
        raise HTTPException(status_code=404, detail=str(e))


@router.get("/{id}/logs")
async def get_logs(
    id: str,
    tail: int = 100,
    follow: bool = False,
    session: Session = Depends(default_db.get_session),
):
    svc = ContainerService(session)
    try:
        logs = await svc.get_logs(id, tail=tail, follow=follow)
        return {"id": id, "logs": logs}
    except CBoxError as e:
        raise HTTPException(status_code=404, detail=str(e))


@router.post("/{id}/exec")
async def exec_in_container(id: str, req: ExecRequest, session: Session = Depends(default_db.get_session)):
    svc = ContainerService(session)
    try:
        code, stdout, stderr = await svc.exec_in_container(id, req.command)
        return {"exit_code": code, "stdout": stdout, "stderr": stderr}
    except CBoxError as e:
        raise HTTPException(status_code=400, detail=str(e))


@router.get("/{id}/stats")
async def get_container_stats(id: str, session: Session = Depends(default_db.get_session)):
    svc = ContainerService(session)
    ident = None if id in ("all", "") else id
    stats = await svc.get_stats(identifier=ident)
    return stats
