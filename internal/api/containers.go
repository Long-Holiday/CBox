package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"

	"cbox/internal/container"
	cboxErr "cbox/internal/errors"
	"cbox/internal/runtime"
	"cbox/internal/transport"
	pkgApi "cbox/pkg/api"
)

type ContainerHandler struct {
	service        *container.Service
	runtimeService *runtime.Service
	sshDir         string
}

func NewContainerHandler(service *container.Service, runtimeService *runtime.Service, sshDir string) *ContainerHandler {
	return &ContainerHandler{
		service:        service,
		runtimeService: runtimeService,
		sshDir:         sshDir,
	}
}

func toContainerResponse(c *container.Container) pkgApi.ContainerResponse {
	var mounts []pkgApi.MountInfo
	for _, m := range c.Mounts {
		mounts = append(mounts, pkgApi.MountInfo{
			VolumeID: m.VolumeID,
			Source:   m.Source,
			Target:   m.Target,
			Mode:     string(m.Mode),
		})
	}

	gpu := ""
	if len(c.Resource.GPUPreference) > 0 {
		gpu = c.Resource.GPUPreference[0]
	}

	return pkgApi.ContainerResponse{
		ID:            c.ID,
		Name:          c.Name,
		ImageID:       c.ImageID,
		ImageName:     c.ImageName,
		State:         string(c.State),
		DesiredState:  string(c.DesiredState),
		Command:       c.Command,
		Env:           c.Env,
		WorkDir:       c.WorkDir,
		GPU:           gpu,
		Mounts:        mounts,
		RuntimeID:     c.RuntimeID,
		RestartPolicy: string(c.RestartPolicy),
		ResumeCommand: c.ResumeCommand,
		ExitCode:      c.ExitCode,
		CreatedAt:     c.CreatedAt,
		StartedAt:     c.StartedAt,
		FinishedAt:    c.FinishedAt,
	}
}

func (h *ContainerHandler) CreateContainer(w http.ResponseWriter, r *http.Request) {
	var req pkgApi.ContainerCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, err)
		return
	}

	c, err := h.service.CreateContainer(r.Context(), req)
	if err != nil {
		if errors.Is(err, cboxErr.ErrAlreadyExists) {
			WriteError(w, http.StatusConflict, err)
			return
		}
		WriteError(w, http.StatusInternalServerError, err)
		return
	}

	WriteJSON(w, http.StatusCreated, toContainerResponse(c))
}

func (h *ContainerHandler) ListContainers(w http.ResponseWriter, r *http.Request) {
	containers, err := h.service.ListContainers(r.Context())
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err)
		return
	}

	var resp []pkgApi.ContainerResponse
	for _, c := range containers {
		resp = append(resp, toContainerResponse(c))
	}

	WriteJSON(w, http.StatusOK, resp)
}

func (h *ContainerHandler) GetContainer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	c, err := h.service.GetContainer(r.Context(), id)
	if err != nil {
		if errors.Is(err, cboxErr.ErrNotFound) {
			WriteError(w, http.StatusNotFound, err)
			return
		}
		WriteError(w, http.StatusInternalServerError, err)
		return
	}

	WriteJSON(w, http.StatusOK, toContainerResponse(c))
}

func (h *ContainerHandler) StartContainer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req pkgApi.ContainerStartRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	c, err := h.service.StartContainer(r.Context(), id, req.Detach)
	if err != nil {
		if errors.Is(err, cboxErr.ErrNotFound) {
			WriteError(w, http.StatusNotFound, err)
			return
		}
		WriteError(w, http.StatusInternalServerError, err)
		return
	}

	WriteJSON(w, http.StatusOK, toContainerResponse(c))
}

func (h *ContainerHandler) StopContainer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req pkgApi.ContainerStopRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	c, err := h.service.StopContainer(r.Context(), id, req.TimeoutSeconds)
	if err != nil {
		if errors.Is(err, cboxErr.ErrNotFound) {
			WriteError(w, http.StatusNotFound, err)
			return
		}
		WriteError(w, http.StatusInternalServerError, err)
		return
	}

	WriteJSON(w, http.StatusOK, toContainerResponse(c))
}

func (h *ContainerHandler) RestartContainer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req pkgApi.ContainerStopRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	c, err := h.service.RestartContainer(r.Context(), id, req.TimeoutSeconds)
	if err != nil {
		if errors.Is(err, cboxErr.ErrNotFound) {
			WriteError(w, http.StatusNotFound, err)
			return
		}
		WriteError(w, http.StatusInternalServerError, err)
		return
	}

	WriteJSON(w, http.StatusOK, toContainerResponse(c))
}

func (h *ContainerHandler) RemoveContainer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	force := r.URL.Query().Get("force") == "true"

	err := h.service.RemoveContainer(r.Context(), id, force)
	if err != nil {
		if errors.Is(err, cboxErr.ErrNotFound) {
			WriteError(w, http.StatusNotFound, err)
			return
		}
		if errors.Is(err, cboxErr.ErrConflict) {
			WriteError(w, http.StatusConflict, err)
			return
		}
		WriteError(w, http.StatusInternalServerError, err)
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "removed"})
}

func (h *ContainerHandler) GetLogs(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	stdout, stderr, err := h.service.GetLogs(r.Context(), id)
	if err != nil {
		if errors.Is(err, cboxErr.ErrNotFound) {
			WriteError(w, http.StatusNotFound, err)
			return
		}
		WriteError(w, http.StatusInternalServerError, err)
		return
	}

	WriteJSON(w, http.StatusOK, pkgApi.LogsResponse{
		ContainerID: id,
		Stdout:      stdout,
		Stderr:      stderr,
	})
}

func (h *ContainerHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	stats, err := h.service.GetStats(r.Context(), id)
	if err != nil {
		if errors.Is(err, cboxErr.ErrNotFound) {
			WriteError(w, http.StatusNotFound, err)
			return
		}
		WriteError(w, http.StatusInternalServerError, err)
		return
	}

	WriteJSON(w, http.StatusOK, stats)
}

func (h *ContainerHandler) Exec(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req pkgApi.ExecRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, err)
		return
	}

	opts := transport.ExecOptions{
		Pty: req.Tty,
	}

	res, err := h.service.Exec(r.Context(), id, req.Command, opts)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err)
		return
	}

	WriteJSON(w, http.StatusOK, pkgApi.ExecResponse{
		ExitCode: res.ExitCode,
		Stdout:   string(res.Stdout),
		Stderr:   string(res.Stderr),
	})
}

func (h *ContainerHandler) GetTransportInfo(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	c, err := h.service.GetContainer(r.Context(), id)
	if err != nil {
		WriteError(w, http.StatusNotFound, err)
		return
	}

	if c.RuntimeID == "" {
		WriteError(w, http.StatusBadRequest, errors.New("container has no assigned runtime"))
		return
	}

	sshConfig := filepath.Join(h.sshDir, c.RuntimeID+".conf")
	WriteJSON(w, http.StatusOK, pkgApi.TransportInfoResponse{
		Host:       c.RuntimeID,
		ConfigFile: sshConfig,
		User:       "root",
		RuntimeID:  c.RuntimeID,
	})
}
