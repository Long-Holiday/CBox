package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"cbox/internal/api"
	"cbox/internal/client"
	"cbox/internal/compose"
	"cbox/internal/config"
	"cbox/internal/container"
	cboxContext "cbox/internal/context"
	"cbox/internal/image"
	"cbox/internal/provider/colab"
	"cbox/internal/runtime"
	"cbox/internal/scheduler"
	"cbox/internal/store"
	"cbox/internal/volume"
)

var Version = "0.1.0"

type Options struct {
	ConfigPath string
	SocketPath string
	TCPAddr    string
	Debug      bool
}

func Run(ctx context.Context, opts Options) error {
	logLevel := slog.LevelInfo
	if opts.Debug {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: logLevel,
	}))
	slog.SetDefault(logger)

	logger.Info("starting cboxd engine daemon", "version", Version)

	cfg, err := config.LoadConfig(opts.ConfigPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	paths, err := config.ResolvePaths(cfg.Engine.DataDir)
	if err != nil {
		return fmt.Errorf("resolve paths: %w", err)
	}

	if err := paths.EnsureDirs(); err != nil {
		return fmt.Errorf("ensure dirs: %w", err)
	}

	if opts.SocketPath != "" {
		paths.SocketPath = opts.SocketPath
	} else if cfg.Engine.SocketPath != "" {
		paths.SocketPath = cfg.Engine.SocketPath
	}

	// 1. Open Database
	db, err := store.OpenDB(paths.DBPath)
	if err != nil {
		return fmt.Errorf("open db %s: %w", paths.DBPath, err)
	}
	defer db.Close()
	logger.Info("database initialized", "path", paths.DBPath)

	// 2. Repositories
	containerRepo := store.NewContainerRepository(db)
	imageRepo := store.NewImageRepository(db)
	volumeRepo := store.NewVolumeRepository(db)
	runtimeRepo := store.NewRuntimeRepository(db)
	contextRepo := store.NewContextRepository(db)
	eventRepo := store.NewEventRepository(db)

	// 3. Providers
	profileStore := colab.NewProfileStore(paths.ProfilesDir)
	colabProvider := colab.NewColabProvider(
		cfg.Provider.ColabBinary,
		profileStore,
		nil,
		paths.SSHDir,
		paths.KeysDir,
		logger,
	)

	// 4. Services
	runtimeService := runtime.NewService(runtimeRepo, cfg.Runtime.IdleTimeout, logger)
	runtimeService.RegisterProvider(colabProvider)

	sched := scheduler.NewScheduler(runtimeService, logger)
	volumeService := volume.NewService(volumeRepo, cfg.Sync.Interval, logger)
	imageService := image.NewService(imageRepo, logger)
	contextService := cboxContext.NewService(contextRepo, logger)

	containerService := container.NewService(
		containerRepo,
		runtimeService,
		sched,
		volumeService,
		imageService,
		eventRepo,
		paths,
		logger,
	)

	composeService := compose.NewService(containerService, logger)

	// Graceful shutdown context
	rootCtx, rootCancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer rootCancel()

	// 5. Initial state reconciliation
	if err := containerService.Reconcile(rootCtx); err != nil {
		logger.Warn("initial reconciliation warning", "err", err)
	}

	// 6. Background tasks
	runtimeService.Pool().StartReaper(rootCtx, time.Minute)
	containerService.StartReconciler(rootCtx, 30*time.Second)

	monitor := runtime.NewMonitor(runtimeService, 15*time.Second, logger)
	monitor.Start(rootCtx)

	// 7. HTTP API Handlers & Server
	containerH := api.NewContainerHandler(containerService, runtimeService, paths.SSHDir)
	imageH := api.NewImageHandler(imageService)
	volumeH := api.NewVolumeHandler(volumeService)
	contextH := api.NewContextHandler(contextService)
	composeH := api.NewComposeHandler(composeService)
	systemH := api.NewSystemHandler(eventRepo, Version)

	serverCfg := api.ServerConfig{
		SocketPath: paths.SocketPath,
		TCPAddr:    opts.TCPAddr,
	}

	server := api.NewServer(
		serverCfg,
		containerH,
		imageH,
		volumeH,
		contextH,
		composeH,
		systemH,
		logger,
	)

	if err := server.Start(); err != nil {
		return fmt.Errorf("start server: %w", err)
	}

	// Write PID file
	if err := os.WriteFile(paths.PIDPath, []byte(fmt.Sprintf("%d\n", os.Getpid())), 0644); err != nil {
		logger.Warn("failed to write pid file", "path", paths.PIDPath, "err", err)
	}
	defer func() {
		_ = os.Remove(paths.PIDPath)
		_ = os.Remove(paths.SocketPath)
	}()

	logger.Info("cboxd is running and ready for requests", "socket", paths.SocketPath, "pid", os.Getpid())

	// Wait for shutdown signal
	<-rootCtx.Done()
	logger.Info("shutting down cboxd...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Stop(shutdownCtx); err != nil {
		logger.Error("server shutdown error", "err", err)
	}

	logger.Info("cboxd stopped cleanly")
	return nil
}

type StatusInfo struct {
	Running    bool   `json:"running"`
	PID        int    `json:"pid,omitempty"`
	SocketPath string `json:"socket_path"`
	Version    string `json:"version,omitempty"`
	Error      string `json:"error,omitempty"`
}

func getPathsAndSocket(opts Options) (*config.Paths, string, error) {
	cfg, err := config.LoadConfig(opts.ConfigPath)
	if err != nil {
		return nil, "", fmt.Errorf("load config: %w", err)
	}
	paths, err := config.ResolvePaths(cfg.Engine.DataDir)
	if err != nil {
		return nil, "", fmt.Errorf("resolve paths: %w", err)
	}
	socketPath := paths.SocketPath
	if opts.SocketPath != "" {
		socketPath = opts.SocketPath
	} else if cfg.Engine.SocketPath != "" {
		socketPath = cfg.Engine.SocketPath
	}
	return paths, socketPath, nil
}

// Status checks whether cboxd is running
func Status(opts Options) (*StatusInfo, error) {
	paths, socketPath, err := getPathsAndSocket(opts)
	if err != nil {
		return nil, err
	}

	info := &StatusInfo{
		Running:    false,
		SocketPath: socketPath,
	}

	// 1. Check PID file
	pidData, err := os.ReadFile(paths.PIDPath)
	var pid int
	if err == nil {
		pid, _ = strconv.Atoi(strings.TrimSpace(string(pidData)))
	}

	// 2. Check if process is alive
	if pid > 0 {
		if err := syscall.Kill(pid, 0); err == nil {
			info.PID = pid
			info.Running = true
		}
	}

	// 3. Ping daemon via socket client
	cli := client.NewClient(socketPath)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	v, err := cli.Version(ctx)
	if err == nil && v != nil {
		info.Running = true
		info.Version = v.Version
		return info, nil
	}

	if info.Running {
		info.Error = fmt.Sprintf("process PID %d is alive but socket ping failed: %v", info.PID, err)
	}

	return info, nil
}

// StartBackground launches the daemon as an independent background process
func StartBackground(opts Options) (int, error) {
	paths, socketPath, err := getPathsAndSocket(opts)
	if err != nil {
		return 0, err
	}

	status, _ := Status(opts)
	if status != nil && status.Running && status.Error == "" {
		return status.PID, nil // already running cleanly
	}

	if err := paths.EnsureDirs(); err != nil {
		return 0, fmt.Errorf("ensure dirs: %w", err)
	}

	execPath, err := os.Executable()
	if err != nil {
		execPath = "cbox"
	}

	args := []string{"daemon", "run"}
	if opts.ConfigPath != "" {
		args = append(args, "--config", opts.ConfigPath)
	}
	if opts.SocketPath != "" {
		args = append(args, "--socket", opts.SocketPath)
	}
	if opts.TCPAddr != "" {
		args = append(args, "--tcp", opts.TCPAddr)
	}
	if opts.Debug {
		args = append(args, "--debug")
	}

	logFile, err := os.OpenFile(paths.LogFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return 0, fmt.Errorf("open log file %s: %w", paths.LogFilePath, err)
	}
	defer logFile.Close()

	cmd := exec.Command(execPath, args...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("start background process: %w", err)
	}

	pid := cmd.Process.Pid

	// Wait up to 5 seconds for the socket to become ready
	deadline := time.Now().Add(5 * time.Second)
	var ready bool
	for time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
		conn, err := net.DialTimeout("unix", socketPath, 100*time.Millisecond)
		if err == nil {
			conn.Close()
			ready = true
			break
		}
	}

	if !ready {
		// Child might have exited or failed
		_ = cmd.Process.Kill()
		return 0, fmt.Errorf("cbox daemon failed to start within 5s (see log at %s)", paths.LogFilePath)
	}

	return pid, nil
}

// Stop terminates the background daemon
func Stop(opts Options) error {
	paths, _, err := getPathsAndSocket(opts)
	if err != nil {
		return err
	}

	pidData, err := os.ReadFile(paths.PIDPath)
	if err != nil {
		// Clean leftover socket if present
		_ = os.Remove(paths.SocketPath)
		return nil
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(pidData)))
	if err != nil || pid <= 0 {
		_ = os.Remove(paths.PIDPath)
		_ = os.Remove(paths.SocketPath)
		return nil
	}

	// Try graceful SIGTERM
	_ = syscall.Kill(pid, syscall.SIGTERM)

	// Wait up to 5 seconds for process termination
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if err := syscall.Kill(pid, 0); err != nil {
			// Process is gone
			_ = os.Remove(paths.PIDPath)
			_ = os.Remove(paths.SocketPath)
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Force kill with SIGKILL
	_ = syscall.Kill(pid, syscall.SIGKILL)
	_ = os.Remove(paths.PIDPath)
	_ = os.Remove(paths.SocketPath)
	return nil
}

// Restart stops and restarts the background daemon
func Restart(opts Options) (int, error) {
	_ = Stop(opts)
	time.Sleep(300 * time.Millisecond)
	return StartBackground(opts)
}

// EnsureRunning guarantees the daemon is running, auto-starting it in background if needed
func EnsureRunning(opts Options) error {
	status, _ := Status(opts)
	if status != nil && status.Running && status.Error == "" {
		return nil
	}
	_, err := StartBackground(opts)
	return err
}
