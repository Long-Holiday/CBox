"""Image management API endpoints."""

from typing import Optional
from fastapi import APIRouter, Depends, HTTPException
from pydantic import BaseModel
from sqlalchemy.orm import Session

from cbox.domain.image import ImageManifest, ImageSummary
from cbox.services.image_service import ImageService
from cbox.storage.database import default_db
from cbox.utils.errors import CBoxError

router = APIRouter(prefix="/images", tags=["images"])


class BuildImageRequest(BaseModel):
    context_dir: str
    tag: Optional[str] = None
    cboxfile_name: str = "Cboxfile"


@router.post("/build", response_model=ImageManifest)
def build_image(req: BuildImageRequest, session: Session = Depends(default_db.get_session)):
    svc = ImageService(session)
    try:
        return svc.build_image(req.context_dir, tag=req.tag, cboxfile_name=req.cboxfile_name)
    except CBoxError as e:
        raise HTTPException(status_code=400, detail=str(e))


@router.get("", response_model=list[ImageSummary])
def list_images(session: Session = Depends(default_db.get_session)):
    svc = ImageService(session)
    return svc.list_images()


@router.get("/{identifier}", response_model=ImageManifest)
def get_image(identifier: str, session: Session = Depends(default_db.get_session)):
    svc = ImageService(session)
    try:
        return svc.get_image(identifier)
    except CBoxError as e:
        raise HTTPException(status_code=404, detail=str(e))


@router.delete("/{identifier}")
def delete_image(identifier: str, session: Session = Depends(default_db.get_session)):
    svc = ImageService(session)
    success = svc.remove_image(identifier)
    if not success:
        raise HTTPException(status_code=404, detail=f"Image {identifier} not found")
    return {"status": "deleted", "id": identifier}
