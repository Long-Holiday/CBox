package api

import (
	"encoding/json"
	"net/http"

	"cbox/internal/compose"
	pkgApi "cbox/pkg/api"
)

type ComposeHandler struct {
	service *compose.Service
}

func NewComposeHandler(service *compose.Service) *ComposeHandler {
	return &ComposeHandler{service: service}
}

type ComposeRequest struct {
	File   string `json:"file"`
	Detach bool   `json:"detach"`
}

func (h *ComposeHandler) Up(w http.ResponseWriter, r *http.Request) {
	var req ComposeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, err)
		return
	}

	if req.File == "" {
		req.File = "cbox-compose.yaml"
	}

	started, err := h.service.Up(r.Context(), req.File, req.Detach)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err)
		return
	}

	var resp []pkgApi.ContainerResponse
	for _, c := range started {
		resp = append(resp, toContainerResponse(c))
	}

	WriteJSON(w, http.StatusOK, resp)
}

func (h *ComposeHandler) Down(w http.ResponseWriter, r *http.Request) {
	var req ComposeRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.File == "" {
		req.File = "cbox-compose.yaml"
	}

	if err := h.service.Down(r.Context(), req.File); err != nil {
		WriteError(w, http.StatusInternalServerError, err)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "stopped and removed"})
}

func (h *ComposeHandler) Ps(w http.ResponseWriter, r *http.Request) {
	file := r.URL.Query().Get("file")
	if file == "" {
		file = "cbox-compose.yaml"
	}

	list, err := h.service.Ps(r.Context(), file)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err)
		return
	}

	var resp []pkgApi.ContainerResponse
	for _, c := range list {
		resp = append(resp, toContainerResponse(c))
	}

	WriteJSON(w, http.StatusOK, resp)
}

func (h *ComposeHandler) Logs(w http.ResponseWriter, r *http.Request) {
	file := r.URL.Query().Get("file")
	if file == "" {
		file = "cbox-compose.yaml"
	}

	logs, err := h.service.Logs(r.Context(), file)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err)
		return
	}

	WriteJSON(w, http.StatusOK, logs)
}
