"""Compose API endpoints."""

from typing import Optional
from fastapi import APIRouter, Depends, HTTPException
from pydantic import BaseModel
from sqlalchemy.orm import Session

from cbox.compose.service import ComposeService
from cbox.domain.container import ContainerSummary
from cbox.storage.database import default_db
from cbox.utils.errors import CBoxError

router = APIRouter(prefix="/compose", tags=["compose"])


class ComposeUpRequest(BaseModel):
    compose_file: str = "cbox-compose.yaml"
    project_name: Optional[str] = None
    detach: bool = True


class ComposeDownRequest(BaseModel):
    compose_file: str = "cbox-compose.yaml"
    project_name: Optional[str] = None


@router.post("/up")
async def compose_up(req: ComposeUpRequest, session: Session = Depends(default_db.get_session)):
    svc = ComposeService(session)
    try:
        started = await svc.up(compose_file=req.compose_file, project_name=req.project_name, detach=req.detach)
        return {"status": "started", "services": started}
    except CBoxError as e:
        raise HTTPException(status_code=400, detail=str(e))


@router.post("/down")
async def compose_down(req: ComposeDownRequest, session: Session = Depends(default_db.get_session)):
    svc = ComposeService(session)
    try:
        stopped = await svc.down(compose_file=req.compose_file, project_name=req.project_name)
        return {"status": "down", "services": stopped}
    except CBoxError as e:
        raise HTTPException(status_code=400, detail=str(e))


@router.get("/ps", response_model=list[ContainerSummary])
def compose_ps(
    compose_file: str = "cbox-compose.yaml",
    project_name: Optional[str] = None,
    session: Session = Depends(default_db.get_session),
):
    svc = ComposeService(session)
    return svc.ps(compose_file=compose_file, project_name=project_name)
