package api

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
)

type ServerConfig struct {
	SocketPath string
	TCPAddr    string
}

type Server struct {
	httpServer *http.Server
	listener   net.Listener
	socketPath string
	tcpAddr    string
	logger     *slog.Logger
}

func NewServer(
	cfg ServerConfig,
	containerH *ContainerHandler,
	imageH *ImageHandler,
	volumeH *VolumeHandler,
	contextH *ContextHandler,
	composeH *ComposeHandler,
	systemH *SystemHandler,
	logger *slog.Logger,
) *Server {
	if logger == nil {
		logger = slog.Default()
	}

	mux := http.NewServeMux()

	// Container routes
	mux.HandleFunc("POST /v1/containers", containerH.CreateContainer)
	mux.HandleFunc("GET /v1/containers", containerH.ListContainers)
	mux.HandleFunc("GET /v1/containers/{id}", containerH.GetContainer)
	mux.HandleFunc("POST /v1/containers/{id}/start", containerH.StartContainer)
	mux.HandleFunc("POST /v1/containers/{id}/stop", containerH.StopContainer)
	mux.HandleFunc("POST /v1/containers/{id}/restart", containerH.RestartContainer)
	mux.HandleFunc("DELETE /v1/containers/{id}", containerH.RemoveContainer)
	mux.HandleFunc("GET /v1/containers/{id}/logs", containerH.GetLogs)
	mux.HandleFunc("GET /v1/containers/{id}/stats", containerH.GetStats)
	mux.HandleFunc("POST /v1/containers/{id}/exec", containerH.Exec)
	mux.HandleFunc("GET /v1/containers/{id}/transport", containerH.GetTransportInfo)

	// Image routes
	mux.HandleFunc("POST /v1/images/build", imageH.BuildImage)
	mux.HandleFunc("GET /v1/images", imageH.ListImages)
	mux.HandleFunc("GET /v1/images/{id}", imageH.GetImage)
	mux.HandleFunc("DELETE /v1/images/{id}", imageH.DeleteImage)

	// Volume routes
	mux.HandleFunc("POST /v1/volumes", volumeH.CreateVolume)
	mux.HandleFunc("GET /v1/volumes", volumeH.ListVolumes)
	mux.HandleFunc("GET /v1/volumes/{name}", volumeH.GetVolume)
	mux.HandleFunc("DELETE /v1/volumes/{name}", volumeH.DeleteVolume)

	// Context routes
	mux.HandleFunc("POST /v1/contexts", contextH.CreateContext)
	mux.HandleFunc("GET /v1/contexts", contextH.ListContexts)
	mux.HandleFunc("GET /v1/contexts/{name}", contextH.GetContext)
	mux.HandleFunc("POST /v1/contexts/{name}/use", contextH.UseContext)
	mux.HandleFunc("DELETE /v1/contexts/{name}", contextH.DeleteContext)

	// Compose routes
	mux.HandleFunc("POST /v1/compose/up", composeH.Up)
	mux.HandleFunc("POST /v1/compose/down", composeH.Down)
	mux.HandleFunc("GET /v1/compose/ps", composeH.Ps)
	mux.HandleFunc("GET /v1/compose/logs", composeH.Logs)

	// System routes
	mux.HandleFunc("GET /v1/ping", systemH.Ping)
	mux.HandleFunc("GET /v1/version", systemH.Version)
	mux.HandleFunc("GET /v1/events", systemH.Events)

	handler := LoggingMiddleware(logger)(mux)

	return &Server{
		httpServer: &http.Server{
			Handler: handler,
		},
		socketPath: cfg.SocketPath,
		tcpAddr:    cfg.TCPAddr,
		logger:     logger,
	}
}

func (s *Server) Start() error {
	if s.tcpAddr != "" {
		l, err := net.Listen("tcp", s.tcpAddr)
		if err != nil {
			return fmt.Errorf("listen tcp %s: %w", s.tcpAddr, err)
		}
		s.listener = l
		s.logger.Info("server listening on TCP", "addr", s.tcpAddr)
	} else {
		if err := os.MkdirAll(filepath.Dir(s.socketPath), 0755); err != nil {
			return fmt.Errorf("create socket dir: %w", err)
		}
		_ = os.Remove(s.socketPath)

		l, err := net.Listen("unix", s.socketPath)
		if err != nil {
			return fmt.Errorf("listen unix socket %s: %w", s.socketPath, err)
		}
		_ = os.Chmod(s.socketPath, 0660)
		s.listener = l
		s.logger.Info("server listening on Unix socket", "socket", s.socketPath)
	}

	go func() {
		if err := s.httpServer.Serve(s.listener); err != nil && err != http.ErrServerClosed {
			s.logger.Error("server error", "err", err)
		}
	}()

	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	s.logger.Info("shutting down HTTP server")
	err := s.httpServer.Shutdown(ctx)
	if s.socketPath != "" {
		_ = os.Remove(s.socketPath)
	}
	return err
}
