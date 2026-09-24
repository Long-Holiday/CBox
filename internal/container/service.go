package container

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"cbox/internal/config"
	cboxErr "cbox/internal/errors"
	"cbox/internal/image"
	"cbox/internal/runtime"
	"cbox/internal/transport"
	"cbox/internal/volume"
	"cbox/internal/worker"
	pkgApi "cbox/pkg/api"
	"github.com/google/uuid"
)

type Scheduler interface {
	AcquireRuntime(ctx context.Context, c *Container) (*runtime.Runtime, error)
}

type Repository interface {
	Create(ctx context.Context, c *Container) error
	Get(ctx context.Context, idOrName string) (*Container, error)
	Update(ctx context.Context, c *Container) error
	UpdateState(ctx context.Context, id string, state ContainerState) error
	UpdateRuntime(ctx context.Context, id string, runtimeID string) error
	UpdateExitCode(ctx context.Context, id string, exitCode int) error
	List(ctx context.Context) ([]*Container, error)
	Delete(ctx context.Context, id string) error
}

type EventRecorder interface {
	Record(ctx context.Context, objType, objID, eventType string, payload any) error
}

type Service struct {
	mu             sync.RWMutex
	repo           Repository
	runtimeService *runtime.Service
	scheduler      Scheduler
	volumeService  *volume.Service
	imageService   *image.Service
	materializer   *image.Materializer
	runner         *Runner
	eventRecorder  EventRecorder
	paths          *config.Paths
	logger         *slog.Logger
	cancelFuncs    map[string]context.CancelFunc
}

func NewService(
	repo Repository,
	runtimeService *runtime.Service,
	scheduler Scheduler,
	volumeService *volume.Service,
	imageService *image.Service,
	eventRecorder EventRecorder,
	paths *config.Paths,
	logger *slog.Logger,
) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		repo:           repo,
		runtimeService: runtimeService,
		scheduler:      scheduler,
		volumeService:  volumeService,
		imageService:   imageService,
		materializer:   image.NewMaterializer(logger),
		runner:         NewRunner(logger),
		eventRecorder:  eventRecorder,
		paths:          paths,
		logger:         logger,
		cancelFuncs:    make(map[string]context.CancelFunc),
	}
}

func (s *Service) CreateContainer(ctx context.Context, req pkgApi.ContainerCreateRequest) (*Container, error) {
	if req.Name == "" {
		req.Name = "cbox-" + uuid.New().String()[:8]
	}

	// Check if already exists
	if existing, _ := s.repo.Get(ctx, req.Name); existing != nil {
		return nil, fmt.Errorf("%w: container %s already exists", cboxErr.ErrAlreadyExists, req.Name)
	}

	restartPol := RestartPolicy(req.RestartPolicy)
	if restartPol == "" {
		restartPol = RestartNo
	}

	var mounts []Mount
	for _, vSpec := range req.Volumes {
		mnt, err := volume.ParseMountSpec(vSpec)
		if err != nil {
			return nil, err
		}
		mounts = append(mounts, *mnt)
	}

	imageID := req.Image
	imageName := req.Image
	if s.imageService != nil {
		if img, err := s.imageService.Get(ctx, req.Image); err == nil {
			imageID = img.ID
		}
	}

	workDir := req.WorkDir
	if workDir == "" {
		workDir = "/workspace"
	}

	c := &Container{
		ID:        uuid.New().String()[:12],
		Name:      req.Name,
		ImageID:   imageID,
		ImageName: imageName,
		Command:   req.Command,
		Env:       req.Env,
		WorkDir:   workDir,
		Resource: ResourceSpec{
			GPUPreference: req.GPUPreference,
			HighMemory:    req.HighMemory,
		},
		Mounts:        mounts,
		State:         StateCreated,
		DesiredState:  DesiredStopped,
		RestartPolicy: restartPol,
		ResumeCommand: req.ResumeCommand,
		Secrets:       req.Secrets,
		CreatedAt:     time.Now().UTC(),
	}

	if err := s.repo.Create(ctx, c); err != nil {
		return nil, fmt.Errorf("persist container: %w", err)
	}

	if s.eventRecorder != nil {
		_ = s.eventRecorder.Record(ctx, "container", c.ID, "container.created", map[string]any{"name": c.Name, "image": c.ImageName})
	}

	s.logger.Info("container created", "id", c.ID, "name", c.Name)
	return c, nil
}

func (s *Service) StartContainer(ctx context.Context, idOrName string, detach bool) (*Container, error) {
	c, err := s.repo.Get(ctx, idOrName)
	if err != nil {
		return nil, err
	}

	if c.State == StateRunning {
		return c, nil
	}

	c.DesiredState = DesiredRunning
	if err := s.repo.Update(ctx, c); err != nil {
		return nil, err
	}

	// 1. Transition to Provisioning
	if err := Transition(c, StateProvisioning); err != nil {
		return nil, err
	}
	_ = s.repo.UpdateState(ctx, c.ID, StateProvisioning)

	// 2. Acquire Runtime via Scheduler
	s.logger.Info("acquiring runtime for container", "container_id", c.ID, "gpu", c.Resource.GPUPreference)
	rt, err := s.scheduler.AcquireRuntime(ctx, c)
	if err != nil {
		_ = Transition(c, StateFailed)
		_ = s.repo.UpdateState(ctx, c.ID, StateFailed)
		return nil, fmt.Errorf("acquire runtime: %w", err)
	}

	c.RuntimeID = rt.ID
	_ = s.repo.UpdateRuntime(ctx, c.ID, rt.ID)

	// 3. Open Transport
	trans, err := s.runtimeService.OpenTransport(ctx, rt)
	if err != nil {
		_ = Transition(c, StateFailed)
		_ = s.repo.UpdateState(ctx, c.ID, StateFailed)
		return nil, fmt.Errorf("open transport: %w", err)
	}

	// 4. Transition to Preparing
	_ = Transition(c, StatePreparing)
	_ = s.repo.UpdateState(ctx, c.ID, StatePreparing)

	// 5. Bootstrap remote worker if needed
	if err := s.ensureWorkerBootstrap(ctx, trans); err != nil {
		s.logger.Warn("worker bootstrap warning", "err", err)
	}

	// 6. Materialize image if specified
	if c.ImageID != "" && s.imageService != nil {
		img, err := s.imageService.Get(ctx, c.ImageID)
		if err == nil {
			if err := s.materializer.Materialize(ctx, trans, img, "."); err != nil {
				_ = Transition(c, StateFailed)
				_ = s.repo.UpdateState(ctx, c.ID, StateFailed)
				return nil, fmt.Errorf("materialize image %s: %w", c.ImageID, err)
			}
			rt.Cache.Images[c.ImageID] = true
			_ = s.runtimeService.UpdateCache(ctx, rt.ID, rt.Cache)
		}
	}

	// 7. Prepare and sync volume mounts
	if len(c.Mounts) > 0 && s.volumeService != nil {
		if err := s.volumeService.PrepareMounts(ctx, trans, c.Mounts, rt.Cache.Volumes); err != nil {
			_ = Transition(c, StateFailed)
			_ = s.repo.UpdateState(ctx, c.ID, StateFailed)
			return nil, fmt.Errorf("prepare mounts: %w", err)
		}
		_ = s.runtimeService.UpdateCache(ctx, rt.ID, rt.Cache)
	}

	// 8. Transition to Starting
	_ = Transition(c, StateStarting)
	_ = s.repo.UpdateState(ctx, c.ID, StateStarting)

	// 9. Setup and launch container process
	if err := s.runner.SetupContainer(ctx, trans, c, c.Command); err != nil {
		_ = Transition(c, StateFailed)
		_ = s.repo.UpdateState(ctx, c.ID, StateFailed)
		return nil, fmt.Errorf("setup container fs: %w", err)
	}

	if err := s.runner.Launch(ctx, trans, c); err != nil {
		_ = Transition(c, StateFailed)
		_ = s.repo.UpdateState(ctx, c.ID, StateFailed)
		return nil, fmt.Errorf("launch container: %w", err)
	}

	// 10. Transition to Running
	_ = Transition(c, StateRunning)
	_ = s.repo.Update(ctx, c)

	if s.eventRecorder != nil {
		_ = s.eventRecorder.Record(ctx, "container", c.ID, "container.started", map[string]any{"runtime_id": rt.ID})
	}

	// 11. Start background goroutines: output sync and process monitor
	containerCtx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.cancelFuncs[c.ID] = cancel
	s.mu.Unlock()

	if s.volumeService != nil {
		s.volumeService.SyncManager().StartOutputSync(containerCtx, trans, c.Mounts)
	}

	go s.monitorContainer(containerCtx, c, rt, trans)

	s.logger.Info("container started successfully", "id", c.ID, "name", c.Name)
	return c, nil
}

func (s *Service) monitorContainer(ctx context.Context, c *Container, rt *runtime.Runtime, trans transport.Transport) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			isRunning, exitCode, err := s.runner.CheckStatus(ctx, trans, c)
			if err != nil {
				s.logger.Warn("error checking container status", "container_id", c.ID, "err", err)
			}

			if !isRunning {
				s.logger.Info("container process exited", "container_id", c.ID, "exit_code", exitCode)

				// Final output volume sync
				if s.volumeService != nil {
					_ = s.volumeService.SyncManager().FinalSyncOutputs(context.Background(), trans, c.Mounts)
				}

				s.mu.Lock()
				delete(s.cancelFuncs, c.ID)
				s.mu.Unlock()

				finalState := StateExited
				if exitCode != nil && *exitCode != 0 {
					finalState = StateFailed
				}

				c.ExitCode = exitCode
				_ = Transition(c, finalState)
				c.DesiredState = DesiredStopped
				_ = s.repo.Update(context.Background(), c)

				if s.eventRecorder != nil {
					_ = s.eventRecorder.Record(context.Background(), "container", c.ID, "container.exited", map[string]any{"exit_code": exitCode})
				}

				// Mark runtime as idle for reuse
				s.runtimeService.Pool().MarkIdle(rt.ID)
				return
			}
		}
	}
}

func (s *Service) StopContainer(ctx context.Context, idOrName string, timeoutSeconds int) (*Container, error) {
	c, err := s.repo.Get(ctx, idOrName)
	if err != nil {
		return nil, err
	}

	if c.State != StateRunning && c.State != StateStarting && c.State != StatePreparing {
		return c, nil
	}

	c.DesiredState = DesiredStopped
	_ = Transition(c, StateStopping)
	_ = s.repo.UpdateState(ctx, c.ID, StateStopping)

	s.mu.Lock()
	if cancel, ok := s.cancelFuncs[c.ID]; ok {
		cancel()
		delete(s.cancelFuncs, c.ID)
	}
	s.mu.Unlock()

	// Stop remote process
	if c.RuntimeID != "" {
		if rt, err := s.runtimeService.GetRuntime(ctx, c.RuntimeID); err == nil {
			if trans, err := s.runtimeService.OpenTransport(ctx, rt); err == nil {
				_ = s.runner.Stop(ctx, trans, c)
				if s.volumeService != nil {
					_ = s.volumeService.SyncManager().FinalSyncOutputs(ctx, trans, c.Mounts)
				}
			}
			s.runtimeService.Pool().MarkIdle(rt.ID)
		}
	}

	_ = Transition(c, StateStopped)
	_ = s.repo.Update(ctx, c)

	if s.eventRecorder != nil {
		_ = s.eventRecorder.Record(ctx, "container", c.ID, "container.stopped", nil)
	}

	s.logger.Info("container stopped", "id", c.ID, "name", c.Name)
	return c, nil
}

func (s *Service) RestartContainer(ctx context.Context, idOrName string, timeoutSeconds int) (*Container, error) {
	_, err := s.StopContainer(ctx, idOrName, timeoutSeconds)
	if err != nil {
		return nil, err
	}
	return s.StartContainer(ctx, idOrName, true)
}

func (s *Service) RemoveContainer(ctx context.Context, idOrName string, force bool) error {
	c, err := s.repo.Get(ctx, idOrName)
	if err != nil {
		return err
	}

	if c.State == StateRunning {
		if !force {
			return fmt.Errorf("%w: cannot remove running container %s, stop it first or use --force", cboxErr.ErrConflict, c.Name)
		}
		_, _ = s.StopContainer(ctx, c.ID, 5)
	}

	if err := s.repo.Delete(ctx, c.ID); err != nil {
		return fmt.Errorf("delete container: %w", err)
	}

	if s.eventRecorder != nil {
		_ = s.eventRecorder.Record(ctx, "container", c.ID, "container.removed", nil)
	}

	s.logger.Info("container removed", "id", c.ID, "name", c.Name)
	return nil
}

func (s *Service) GetContainer(ctx context.Context, idOrName string) (*Container, error) {
	return s.repo.Get(ctx, idOrName)
}

func (s *Service) ListContainers(ctx context.Context) ([]*Container, error) {
	return s.repo.List(ctx)
}

func (s *Service) GetLogs(ctx context.Context, idOrName string) (stdout, stderr string, err error) {
	c, err := s.repo.Get(ctx, idOrName)
	if err != nil {
		return "", "", err
	}

	if c.RuntimeID == "" {
		return "", "", fmt.Errorf("container has no assigned runtime")
	}

	rt, err := s.runtimeService.GetRuntime(ctx, c.RuntimeID)
	if err != nil {
		return "", "", err
	}

	trans, err := s.runtimeService.OpenTransport(ctx, rt)
	if err != nil {
		return "", "", err
	}

	return s.runner.ReadLogs(ctx, trans, c)
}

func (s *Service) GetStats(ctx context.Context, idOrName string) (*pkgApi.StatsResponse, error) {
	c, err := s.repo.Get(ctx, idOrName)
	if err != nil {
		return nil, err
	}

	if c.RuntimeID == "" {
		return nil, fmt.Errorf("container %s has no active runtime", c.Name)
	}

	rt, err := s.runtimeService.GetRuntime(ctx, c.RuntimeID)
	if err != nil {
		return nil, err
	}

	trans, err := s.runtimeService.OpenTransport(ctx, rt)
	if err != nil {
		return nil, err
	}

	// Read stats directly from remote worker
	cmd := `nvidia-smi --query-gpu=name,memory.used,memory.total,utilization.gpu --format=csv,noheader,nounits 2>/dev/null || echo "N/A,0,0,0"`
	res, err := trans.Exec(ctx, []string{"bash", "-c", cmd}, transport.ExecOptions{})
	if err != nil {
		return nil, err
	}

	stats := &pkgApi.StatsResponse{
		ContainerID: c.ID,
		Timestamp:   time.Now().UTC(),
	}

	line := strings.TrimSpace(string(res.Stdout))
	parts := strings.Split(line, ",")
	if len(parts) >= 4 {
		stats.GPUName = strings.TrimSpace(parts[0])
	}

	return stats, nil
}

func (s *Service) Exec(ctx context.Context, idOrName string, command []string, opts transport.ExecOptions) (*transport.ExecResult, error) {
	c, err := s.repo.Get(ctx, idOrName)
	if err != nil {
		return nil, err
	}

	if c.State != StateRunning {
		return nil, fmt.Errorf("%w: container %s is not running (state=%s)", cboxErr.ErrInvalidState, c.Name, c.State)
	}

	rt, err := s.runtimeService.GetRuntime(ctx, c.RuntimeID)
	if err != nil {
		return nil, err
	}

	trans, err := s.runtimeService.OpenTransport(ctx, rt)
	if err != nil {
		return nil, err
	}

	if opts.WorkDir == "" {
		opts.WorkDir = c.WorkDir
	}

	return trans.Exec(ctx, command, opts)
}

func (s *Service) ensureWorkerBootstrap(ctx context.Context, trans transport.Transport) error {
	// Check if bootstrap was already done
	res, err := trans.Exec(ctx, []string{"test", "-f", "/content/.cbox/worker.json"}, transport.ExecOptions{})
	if err == nil && res.ExitCode == 0 {
		return nil
	}

	s.logger.Info("bootstrapping remote worker environment")

	// Ensure remote directories
	initDirs := "mkdir -p /content/.cbox/bin /content/.cbox/containers /content/.cbox/images /content/.cbox/volumes /content/.cbox/cache"
	_, _ = trans.Exec(ctx, []string{"bash", "-c", initDirs}, transport.ExecOptions{})

	// Copy worker scripts (using embedded scripts if not found on disk)
	for _, script := range worker.ListScripts() {
		scriptData, err := worker.GetScript(script)
		if err == nil && len(scriptData) > 0 {
			tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("cbox-%s", script))
			if err := os.WriteFile(tmpFile, scriptData, 0755); err == nil {
				_ = trans.CopyTo(ctx, tmpFile, fmt.Sprintf("/content/.cbox/bin/%s", script))
				_ = os.Remove(tmpFile)
			}
			_, _ = trans.Exec(ctx, []string{"chmod", "+x", fmt.Sprintf("/content/.cbox/bin/%s", script)}, transport.ExecOptions{})
		}
	}

	// Mark worker ready
	markReady := `echo '{"status": "ready"}' > /content/.cbox/worker.json`
	_, _ = trans.Exec(ctx, []string{"bash", "-c", markReady}, transport.ExecOptions{})

	return nil
}

func (s *Service) Reconcile(ctx context.Context) error {
	containers, err := s.repo.List(ctx)
	if err != nil {
		return err
	}

	for _, c := range containers {
		if c.DesiredState == DesiredRunning && c.State != StateRunning && c.State != StateStarting && c.State != StatePreparing && c.State != StateProvisioning {
			// Container should be running but is not!
			s.logger.Info("reconciler: recovering container to desired state running", "container_id", c.ID, "name", c.Name)
			go func(cont *Container) {
				_, err := s.StartContainer(context.Background(), cont.ID, true)
				if err != nil {
					s.logger.Error("reconciler failed to restart container", "container_id", cont.ID, "err", err)
				}
			}(c)
		}
	}
	return nil
}

func (s *Service) StartReconciler(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_ = s.Reconcile(ctx)
			}
		}
	}()
}
