package api

import (
	"encoding/json"
	"errors"
	"net/http"

	cboxContext "cbox/internal/context"
	cboxErr "cbox/internal/errors"
	pkgApi "cbox/pkg/api"
)

type ContextHandler struct {
	service *cboxContext.Service
}

func NewContextHandler(service *cboxContext.Service) *ContextHandler {
	return &ContextHandler{service: service}
}

func toContextResponse(c *cboxContext.Context) pkgApi.ContextResponse {
	return pkgApi.ContextResponse{
		Name:         c.Name,
		Provider:     c.Provider,
		Profile:      c.Profile,
		AutoSchedule: c.AutoSchedule,
		IsCurrent:    c.IsCurrent,
	}
}

func (h *ContextHandler) CreateContext(w http.ResponseWriter, r *http.Request) {
	var req pkgApi.ContextCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, err)
		return
	}

	c, err := h.service.Create(r.Context(), req.Name, req.Provider, req.Profile, req.AutoSchedule)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err)
		return
	}

	WriteJSON(w, http.StatusCreated, toContextResponse(c))
}

func (h *ContextHandler) ListContexts(w http.ResponseWriter, r *http.Request) {
	list, err := h.service.List(r.Context())
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err)
		return
	}

	var resp []pkgApi.ContextResponse
	for _, c := range list {
		resp = append(resp, toContextResponse(c))
	}

	WriteJSON(w, http.StatusOK, resp)
}

func (h *ContextHandler) GetContext(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	c, err := h.service.Get(r.Context(), name)
	if err != nil {
		if errors.Is(err, cboxErr.ErrNotFound) {
			WriteError(w, http.StatusNotFound, err)
			return
		}
		WriteError(w, http.StatusInternalServerError, err)
		return
	}

	WriteJSON(w, http.StatusOK, toContextResponse(c))
}

func (h *ContextHandler) UseContext(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	err := h.service.Use(r.Context(), name)
	if err != nil {
		if errors.Is(err, cboxErr.ErrNotFound) {
			WriteError(w, http.StatusNotFound, err)
			return
		}
		WriteError(w, http.StatusInternalServerError, err)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok", "current": name})
}

func (h *ContextHandler) DeleteContext(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	err := h.service.Delete(r.Context(), name)
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
