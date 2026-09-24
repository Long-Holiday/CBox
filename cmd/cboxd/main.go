package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cbox/internal/api"
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

var (
	Version = "0.1.0"
)

func main() {
	configPath := flag.String("config", "", "path to cbox config file")
	socketPath := flag.String("socket", "", "override unix socket path")
	tcpAddr := flag.String("tcp", "", "optional TCP listen address (e.g. 127.0.0.1:8080)")
	debug := flag.Bool("debug", false, "enable debug logging")
	flag.Parse()

	logLevel := slog.LevelInfo
	if *debug {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: logLevel,
	}))
	slog.SetDefault(logger)

	logger.Info("starting cboxd engine daemon", "version", Version)

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		logger.Error("failed to load configuration", "err", err)
		os.Exit(1)
	}

	paths, err := config.ResolvePaths(cfg.Engine.DataDir)
	if err != nil {
		logger.Error("failed to resolve cbox paths", "err", err)
		os.Exit(1)
	}

	if err := paths.EnsureDirs(); err != nil {
		logger.Error("failed to initialize directories", "err", err)
		os.Exit(1)
	}

	if *socketPath != "" {
		paths.SocketPath = *socketPath
	} else if cfg.Engine.SocketPath != "" {
		paths.SocketPath = cfg.Engine.SocketPath
	}

	// 1. Open Database
	db, err := store.OpenDB(paths.DBPath)
	if err != nil {
		logger.Error("failed to open database", "path", paths.DBPath, "err", err)
		os.Exit(1)
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
	rootCtx, rootCancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
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
		TCPAddr:    *tcpAddr,
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
		logger.Error("failed to start server", "err", err)
		os.Exit(1)
	}

	logger.Info("cboxd is running and ready for requests", "socket", paths.SocketPath)

	// Wait for shutdown signal
	<-rootCtx.Done()
	logger.Info("shutting down cboxd...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Stop(shutdownCtx); err != nil {
		logger.Error("server shutdown error", "err", err)
	}

	logger.Info("cboxd stopped cleanly")
}
