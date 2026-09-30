package api

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"cbox/internal/client"
	"cbox/internal/compose"
	"cbox/internal/config"
	"cbox/internal/container"
	cboxContext "cbox/internal/context"
	"cbox/internal/image"
	"cbox/internal/provider"
	"cbox/internal/runtime"
	"cbox/internal/scheduler"
	"cbox/internal/store"
	"cbox/internal/volume"
	pkgApi "cbox/pkg/api"
)

func TestFullE2ELifecycle(t *testing.T) {
	tempDir := t.TempDir()
	socketPath := filepath.Join(tempDir, "cbox.sock")
	dbPath := filepath.Join(tempDir, "cbox.db")
	sshDir := filepath.Join(tempDir, "ssh")

	paths, _ := config.ResolvePaths(tempDir)
	_ = paths.EnsureDirs()

	db, err := store.OpenDB(dbPath)
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	defer db.Close()

	containerRepo := store.NewContainerRepository(db)
	imageRepo := store.NewImageRepository(db)
	volumeRepo := store.NewVolumeRepository(db)
	runtimeRepo := store.NewRuntimeRepository(db)
	contextRepo := store.NewContextRepository(db)
	eventRepo := store.NewEventRepository(db)

	mockProv := provider.NewMockProvider("mock")
	mockProv.UseLocalExec = true
	mockProv.LocalBaseDir = tempDir

	rtService := runtime.NewService(runtimeRepo, 30*time.Minute, nil)
	rtService.RegisterProvider(mockProv)

	sched := scheduler.NewScheduler(rtService, nil)
	volService := volume.NewService(volumeRepo, 5*time.Minute, nil)
	imgService := image.NewService(imageRepo, nil)
	ctxService := cboxContext.NewService(contextRepo, nil)

	contService := container.NewService(
		containerRepo,
		rtService,
		sched,
		volService,
		imgService,
		eventRepo,
		paths,
		nil,
	)

	compService := compose.NewService(contService, nil)

	containerH := NewContainerHandler(contService, rtService, sshDir)
	imageH := NewImageHandler(imgService)
	volumeH := NewVolumeHandler(volService)
	contextH := NewContextHandler(ctxService)
	composeH := NewComposeHandler(compService)
	systemH := NewSystemHandler(eventRepo, "0.1.0")

	server := NewServer(
		ServerConfig{SocketPath: socketPath},
		containerH,
		imageH,
		volumeH,
		contextH,
		composeH,
		systemH,
		nil,
	)

	if err := server.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer func() { _ = server.Stop(context.Background()) }()

	// Wait for socket to become ready
	time.Sleep(100 * time.Millisecond)

	cli := client.NewClient(socketPath)
	ctx := context.Background()

	// 1. Ping & Version
	if err := cli.Ping(ctx); err != nil {
		t.Fatalf("ping failed: %v", err)
	}

	// Stopping a failed start must cancel the reconciler's desired-running state.
	failed, err := cli.CreateContainer(ctx, pkgApi.ContainerCreateRequest{Name: "failed-start", Image: "test:latest"})
	if err != nil {
		t.Fatal(err)
	}
	stored, err := containerRepo.Get(ctx, failed.ID)
	if err != nil {
		t.Fatal(err)
	}
	stored.State = container.StateFailed
	stored.DesiredState = container.DesiredRunning
	if err := containerRepo.Update(ctx, stored); err != nil {
		t.Fatal(err)
	}
	if _, err := cli.StopContainer(ctx, failed.ID, 5); err != nil {
		t.Fatal(err)
	}
	stored, err = containerRepo.Get(ctx, failed.ID)
	if err != nil || stored.DesiredState != container.DesiredStopped {
		t.Fatalf("failed start still requests recovery: %+v, %v", stored, err)
	}
	if err := cli.RemoveContainer(ctx, failed.ID, false); err != nil {
		t.Fatal(err)
	}

	ver, err := cli.Version(ctx)
	if err != nil || ver.Version != "0.1.0" {
		t.Fatalf("version failed: %v, %+v", err, ver)
	}

	// 2. Volume lifecycle
	vol, err := cli.CreateVolume(ctx, "test-vol", tempDir, "ro")
	if err != nil {
		t.Fatalf("create volume failed: %v", err)
	}
	if vol.Name != "test-vol" {
		t.Fatalf("expected volume test-vol, got %s", vol.Name)
	}

	volumes, err := cli.ListVolumes(ctx)
	if err != nil || len(volumes) != 1 {
		t.Fatalf("list volumes failed: %v, len=%d", err, len(volumes))
	}

	// 3. Container lifecycle
	req := pkgApi.ContainerCreateRequest{
		Name:          "test-worker",
		Image:         "test:latest",
		GPUPreference: []string{"T4"},
		Command:       []string{"echo", "hello cbox"},
		Env:           map[string]string{"CBOX_TEST_VALUE": "literal $HOME 'quoted'"},
	}

	c, err := cli.CreateContainer(ctx, req)
	if err != nil {
		t.Fatalf("create container failed: %v", err)
	}
	if c.Name != "test-worker" {
		t.Fatalf("expected name test-worker, got %s", c.Name)
	}

	// Start container
	started, err := cli.StartContainer(ctx, c.ID, true)
	if err != nil {
		t.Fatalf("start container failed: %v", err)
	}
	if started.State != "running" {
		t.Fatalf("expected state running, got %s", started.State)
	}

	// List containers
	containers, err := cli.ListContainers(ctx)
	if err != nil || len(containers) != 1 {
		t.Fatalf("list containers failed: %v", err)
	}

	// Exec in container
	execRes, err := cli.Exec(ctx, c.ID, []string{"echo", "cbox-exec-test"}, false)
	if err != nil {
		t.Fatalf("exec in container failed: %v", err)
	}
	if execRes.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", execRes.ExitCode)
	}
	envRes, err := cli.Exec(ctx, c.ID, []string{"printenv", "CBOX_TEST_VALUE"}, false)
	if err != nil || envRes.ExitCode != 0 || envRes.Stdout != "literal $HOME 'quoted'\n" {
		t.Fatalf("exec did not inherit container environment: %+v, %v", envRes, err)
	}

	// Stop container
	stopped, err := cli.StopContainer(ctx, c.ID, 5)
	if err != nil {
		t.Fatalf("stop container failed: %v", err)
	}
	if stopped.State != "stopped" {
		t.Fatalf("expected state stopped, got %s", stopped.State)
	}

	// Remove container
	if err := cli.RemoveContainer(ctx, c.ID, false); err != nil {
		t.Fatalf("remove container failed: %v", err)
	}

	// Verify events were recorded
	events, err := cli.GetEvents(ctx, 10, "", "")
	if err != nil || len(events) == 0 {
		t.Fatalf("expected recorded events, got %v", events)
	}
}
