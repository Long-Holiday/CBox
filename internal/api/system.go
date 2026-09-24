package api

import (
	"net/http"
	"runtime"
	"strconv"

	"cbox/internal/store"
	pkgApi "cbox/pkg/api"
)

type SystemHandler struct {
	eventRepo *store.EventRepository
	version   string
}

func NewSystemHandler(eventRepo *store.EventRepository, version string) *SystemHandler {
	if version == "" {
		version = "0.1.0"
	}
	return &SystemHandler{
		eventRepo: eventRepo,
		version:   version,
	}
}

func (h *SystemHandler) Ping(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *SystemHandler) Version(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, pkgApi.VersionResponse{
		Version:   h.version,
		GoVersion: runtime.Version(),
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
	})
}

func (h *SystemHandler) Events(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
		limit = l
	}

	objType := r.URL.Query().Get("object_type")
	objID := r.URL.Query().Get("object_id")

	var events []*store.Event
	var err error

	if objType != "" && objID != "" && h.eventRepo != nil {
		events, err = h.eventRepo.ListByObject(r.Context(), objType, objID)
	} else if h.eventRepo != nil {
		events, err = h.eventRepo.List(r.Context(), limit)
	}

	if err != nil {
		WriteError(w, http.StatusInternalServerError, err)
		return
	}

	var resp []pkgApi.EventResponse
	for _, e := range events {
		resp = append(resp, pkgApi.EventResponse{
			ID:         e.ID,
			ObjectType: e.ObjectType,
			ObjectID:   e.ObjectID,
			Type:       e.Type,
			Timestamp:  e.Timestamp,
			Payload:    e.Payload,
		})
	}

	WriteJSON(w, http.StatusOK, resp)
}
