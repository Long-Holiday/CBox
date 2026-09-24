package api

import (
	"encoding/json"
	"errors"
	"net/http"

	cboxErr "cbox/internal/errors"
	"cbox/internal/volume"
	pkgApi "cbox/pkg/api"
)

type VolumeHandler struct {
	service *volume.Service
}

func NewVolumeHandler(service *volume.Service) *VolumeHandler {
	return &VolumeHandler{service: service}
}

func toVolumeResponse(v *volume.Volume) pkgApi.VolumeResponse {
	return pkgApi.VolumeResponse{
		ID:        v.ID,
		Name:      v.Name,
		Source:    v.Source,
		Mode:      string(v.Mode),
		CreatedAt: v.CreatedAt,
	}
}

func (h *VolumeHandler) CreateVolume(w http.ResponseWriter, r *http.Request) {
	var req pkgApi.VolumeCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, err)
		return
	}

	mode := volume.VolumeMode(req.Mode)
	if mode == "" {
		mode = volume.ModeReadOnly
	}

	v, err := h.service.CreateVolume(r.Context(), req.Name, req.Source, mode)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err)
		return
	}

	WriteJSON(w, http.StatusCreated, toVolumeResponse(v))
}

func (h *VolumeHandler) ListVolumes(w http.ResponseWriter, r *http.Request) {
	volumes, err := h.service.ListVolumes(r.Context())
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err)
		return
	}

	var resp []pkgApi.VolumeResponse
	for _, v := range volumes {
		resp = append(resp, toVolumeResponse(v))
	}

	WriteJSON(w, http.StatusOK, resp)
}

func (h *VolumeHandler) GetVolume(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	v, err := h.service.GetVolume(r.Context(), name)
	if err != nil {
		if errors.Is(err, cboxErr.ErrNotFound) {
			WriteError(w, http.StatusNotFound, err)
			return
		}
		WriteError(w, http.StatusInternalServerError, err)
		return
	}

	WriteJSON(w, http.StatusOK, toVolumeResponse(v))
}

func (h *VolumeHandler) DeleteVolume(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	err := h.service.DeleteVolume(r.Context(), name)
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
