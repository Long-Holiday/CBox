package api

import (
	"encoding/json"
	"errors"
	"net/http"

	cboxErr "cbox/internal/errors"
	"cbox/internal/image"
	pkgApi "cbox/pkg/api"
)

type ImageHandler struct {
	service *image.Service
}

func NewImageHandler(service *image.Service) *ImageHandler {
	return &ImageHandler{service: service}
}

func toImageResponse(img *image.Image) pkgApi.ImageResponse {
	return pkgApi.ImageResponse{
		ID:        img.ID,
		Tags:      img.Tags,
		CreatedAt: img.CreatedAt,
		Base:      img.Manifest.Base,
		Apt:       img.Manifest.Apt,
		Pip:       img.Manifest.Pip,
	}
}

func (h *ImageHandler) BuildImage(w http.ResponseWriter, r *http.Request) {
	var req pkgApi.ImageBuildRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, err)
		return
	}

	img, err := h.service.Build(r.Context(), req.ContextDir, req.Cboxfile, req.Tag)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err)
		return
	}

	WriteJSON(w, http.StatusCreated, toImageResponse(img))
}

func (h *ImageHandler) ListImages(w http.ResponseWriter, r *http.Request) {
	images, err := h.service.List(r.Context())
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err)
		return
	}

	var resp []pkgApi.ImageResponse
	for _, img := range images {
		resp = append(resp, toImageResponse(img))
	}

	WriteJSON(w, http.StatusOK, resp)
}

func (h *ImageHandler) GetImage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	img, err := h.service.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, cboxErr.ErrNotFound) {
			WriteError(w, http.StatusNotFound, err)
			return
		}
		WriteError(w, http.StatusInternalServerError, err)
		return
	}

	WriteJSON(w, http.StatusOK, toImageResponse(img))
}

func (h *ImageHandler) DeleteImage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	err := h.service.Delete(r.Context(), id)
	if err != nil {
		if errors.Is(err, cboxErr.ErrNotFound) {
			WriteError(w, http.StatusNotFound, err)
			return
		}
		WriteError(w, http.StatusInternalServerError, err)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
