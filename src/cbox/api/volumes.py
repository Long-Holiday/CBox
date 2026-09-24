"""Volume management API endpoints."""

from fastapi import APIRouter, Depends, HTTPException
from pydantic import BaseModel
from sqlalchemy.orm import Session

from cbox.domain.enums import VolumeMode
from cbox.domain.volume import VolumeManifest, VolumeSummary
from cbox.services.volume_service import VolumeService
from cbox.storage.database import default_db
from cbox.utils.errors import CBoxError

router = APIRouter(prefix="/volumes", tags=["volumes"])


class CreateVolumeRequest(BaseModel):
    name: str
    source: str
    mode: str = "ro"
    immutable: bool = False
    checksum: bool = False


@router.post("", response_model=VolumeManifest)
def create_volume(req: CreateVolumeRequest, session: Session = Depends(default_db.get_session)):
    svc = VolumeService(session)
    try:
        return svc.create_volume(
            name=req.name,
            source=req.source,
            mode=VolumeMode(req.mode),
            immutable=req.immutable,
            use_checksum=req.checksum,
        )
    except CBoxError as e:
        raise HTTPException(status_code=400, detail=str(e))


@router.get("", response_model=list[VolumeSummary])
def list_volumes(session: Session = Depends(default_db.get_session)):
    svc = VolumeService(session)
    return svc.list_volumes()


@router.get("/{name}", response_model=VolumeManifest)
def get_volume(name: str, session: Session = Depends(default_db.get_session)):
    svc = VolumeService(session)
    try:
        return svc.get_volume(name)
    except CBoxError as e:
        raise HTTPException(status_code=404, detail=str(e))


@router.delete("/{name}")
def delete_volume(name: str, session: Session = Depends(default_db.get_session)):
    svc = VolumeService(session)
    success = svc.remove_volume(name)
    if not success:
        raise HTTPException(status_code=404, detail=f"Volume {name} not found")
    return {"status": "deleted", "name": name}
