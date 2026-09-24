"""Context management API endpoints."""

from fastapi import APIRouter, Depends, HTTPException
from pydantic import BaseModel
from sqlalchemy.orm import Session

from cbox.domain.context import Context
from cbox.storage.database import default_db
from cbox.storage.repositories import ContextRepository

router = APIRouter(prefix="/contexts", tags=["contexts"])


class CreateContextRequest(BaseModel):
    name: str
    provider: str = "colab"
    profile: str = "default"


@router.post("", response_model=Context)
def create_context(req: CreateContextRequest, session: Session = Depends(default_db.get_session)):
    repo = ContextRepository(session)
    ctx = Context(name=req.name, provider=req.provider, profile=req.profile, active=False)
    repo.save_context(ctx)
    return ctx


@router.get("", response_model=list[Context])
def list_contexts(session: Session = Depends(default_db.get_session)):
    repo = ContextRepository(session)
    return repo.list_contexts()


@router.post("/{name}/use")
def use_context(name: str, session: Session = Depends(default_db.get_session)):
    repo = ContextRepository(session)
    success = repo.set_active(name)
    if not success:
        raise HTTPException(status_code=404, detail=f"Context {name} not found")
    return {"status": "active", "context": name}


@router.delete("/{name}")
def delete_context(name: str, session: Session = Depends(default_db.get_session)):
    repo = ContextRepository(session)
    success = repo.delete_context(name)
    if not success:
        raise HTTPException(status_code=404, detail=f"Context {name} not found")
    return {"status": "deleted", "context": name}
