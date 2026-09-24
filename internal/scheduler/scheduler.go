package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"cbox/internal/container"
	cboxErr "cbox/internal/errors"
	"cbox/internal/runtime"
)

type Scheduler interface {
	AcquireRuntime(ctx context.Context, c *container.Container) (*runtime.Runtime, error)
}

type DefaultScheduler struct {
	mu             sync.Mutex
	runtimeService *runtime.Service
	logger         *slog.Logger
}

func NewScheduler(runtimeService *runtime.Service, logger *slog.Logger) *DefaultScheduler {
	if logger == nil {
		logger = slog.Default()
	}
	return &DefaultScheduler{
		runtimeService: runtimeService,
		logger:         logger,
	}
}

func (s *DefaultScheduler) AcquireRuntime(ctx context.Context, c *container.Container) (*runtime.Runtime, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 1. Try to find best scored idle runtime in pool
	poolRuntimes := s.runtimeService.Pool().List()
	var bestRuntime *runtime.Runtime
	bestScore := -1

	for _, rt := range poolRuntimes {
		sc := ScoreRuntime(rt, c)
		if sc > bestScore {
			bestScore = sc
			bestRuntime = rt
		}
	}

	if bestRuntime != nil && bestScore >= 100 {
		s.logger.Info("reusing existing idle runtime", "runtime_id", bestRuntime.ID, "score", bestScore)
		s.runtimeService.Pool().MarkBusy(bestRuntime.ID)
		return bestRuntime, nil
	}

	// 2. Allocate new runtime by trying GPU preferences in order
	gpuChoices := c.Resource.GPUPreference
	if len(gpuChoices) == 0 {
		gpuChoices = []string{""} // default GPU
	}

	var lastErr error
	for _, gpu := range gpuChoices {
		s.logger.Info("attempting to provision new runtime", "gpu", gpu, "profile", c.Resource.Profile)
		req := runtime.RuntimeRequest{
			GPU:        gpu,
			HighMemory: c.Resource.HighMemory,
			Profile:    c.Resource.Profile,
		}

		rt, err := s.runtimeService.CreateRuntime(ctx, "", req)
		if err == nil {
			s.runtimeService.Pool().MarkBusy(rt.ID)
			return rt, nil
		}
		s.logger.Warn("failed to provision runtime with gpu preference", "gpu", gpu, "err", err)
		lastErr = err
	}

	return nil, &cboxErr.AllocationError{
		GPU:    fmt.Sprintf("%v", gpuChoices),
		Reason: "exhausted all gpu preferences",
		Err:    lastErr,
	}
}
