package runtime

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	cboxErr "cbox/internal/errors"
	"cbox/internal/transport"
)

type Provider interface {
	Name() string
	CreateRuntime(ctx context.Context, req RuntimeRequest) (*RuntimeHandle, error)
	GetRuntime(ctx context.Context, rt *Runtime) (*RuntimeStatus, error)
	StopRuntime(ctx context.Context, rt *Runtime) error
	ListRuntimes(ctx context.Context) ([]RuntimeStatus, error)
	OpenTransport(ctx context.Context, rt *Runtime) (transport.Transport, error)
}

type Repository interface {
	Create(ctx context.Context, rt *Runtime) error
	Get(ctx context.Context, id string) (*Runtime, error)
	UpdateState(ctx context.Context, id string, state RuntimeState) error
	UpdateCache(ctx context.Context, id string, cache RuntimeCache) error
	UpdateLastSeen(ctx context.Context, id string) error
	List(ctx context.Context) ([]*Runtime, error)
	Delete(ctx context.Context, id string) error
}

type Service struct {
	mu        sync.RWMutex
	providers map[string]Provider
	repo      Repository
	pool      *Pool
	logger    *slog.Logger
}

func NewService(repo Repository, idleTimeout time.Duration, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}

	s := &Service{
		providers: make(map[string]Provider),
		repo:      repo,
		logger:    logger,
	}

	s.pool = NewPool(idleTimeout, s.StopRuntime, logger)
	return s
}

func (s *Service) RegisterProvider(p Provider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.providers[p.Name()] = p
	s.logger.Info("registered runtime provider", "provider", p.Name())
}

func (s *Service) GetProvider(name string) (Provider, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if name == "" {
		name = "colab"
	}

	p, ok := s.providers[name]
	if !ok {
		// If only 1 provider exists, fallback to it (useful for testing mock)
		if len(s.providers) == 1 {
			for _, singleP := range s.providers {
				return singleP, nil
			}
		}
		return nil, fmt.Errorf("%w: provider %s not found", cboxErr.ErrNotFound, name)
	}
	return p, nil
}

func (s *Service) Pool() *Pool {
	return s.pool
}

func (s *Service) CreateRuntime(ctx context.Context, providerName string, req RuntimeRequest) (*Runtime, error) {
	p, err := s.GetProvider(providerName)
	if err != nil {
		return nil, err
	}

	handle, err := p.CreateRuntime(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("provider create runtime: %w", err)
	}

	now := time.Now().UTC()
	rt := &Runtime{
		ID:           handle.ID,
		Provider:     p.Name(),
		Session:      handle.Session,
		Profile:      req.Profile,
		RequestedGPU: req.GPU,
		ActualGPU:    req.GPU,
		State:        StateReady,
		CreatedAt:    now,
		LastSeen:     now,
		Cache: RuntimeCache{
			Images:  make(map[string]bool),
			Volumes: make(map[string]string),
		},
	}

	if s.repo != nil {
		if err := s.repo.Create(ctx, rt); err != nil {
			s.logger.Error("failed to persist runtime", "id", rt.ID, "err", err)
		}
	}

	s.pool.Track(rt)
	s.logger.Info("runtime created and ready", "runtime_id", rt.ID, "gpu", rt.ActualGPU)
	return rt, nil
}

func (s *Service) GetRuntime(ctx context.Context, id string) (*Runtime, error) {
	if rt := s.pool.Get(id); rt != nil {
		return rt, nil
	}

	if s.repo != nil {
		rt, err := s.repo.Get(ctx, id)
		if err == nil {
			s.pool.Track(rt)
			return rt, nil
		}
	}

	return nil, fmt.Errorf("%w: runtime %s not found", cboxErr.ErrNotFound, id)
}

func (s *Service) StopRuntime(ctx context.Context, rt *Runtime) error {
	p, err := s.GetProvider(rt.Provider)
	if err != nil {
		return err
	}

	rt.State = StateStopping
	if s.repo != nil {
		_ = s.repo.UpdateState(ctx, rt.ID, StateStopping)
	}

	if err := p.StopRuntime(ctx, rt); err != nil {
		s.logger.Error("error stopping runtime on provider", "runtime_id", rt.ID, "err", err)
	}

	rt.State = StateStopped
	if s.repo != nil {
		_ = s.repo.UpdateState(ctx, rt.ID, StateStopped)
	}
	s.pool.Untrack(rt.ID)
	s.logger.Info("runtime stopped", "runtime_id", rt.ID)
	return nil
}

func (s *Service) OpenTransport(ctx context.Context, rt *Runtime) (transport.Transport, error) {
	p, err := s.GetProvider(rt.Provider)
	if err != nil {
		return nil, err
	}
	return p.OpenTransport(ctx, rt)
}

func (s *Service) ListRuntimes(ctx context.Context) ([]*Runtime, error) {
	if s.repo != nil {
		return s.repo.List(ctx)
	}
	return s.pool.List(), nil
}

func (s *Service) UpdateCache(ctx context.Context, id string, cache RuntimeCache) error {
	rt, err := s.GetRuntime(ctx, id)
	if err != nil {
		return err
	}
	rt.Cache = cache
	if s.repo != nil {
		return s.repo.UpdateCache(ctx, id, cache)
	}
	return nil
}

// EnsureGoogleDrive uses the provider's notebook authentication channel, which
// is unavailable to an ordinary SSH Python process.
func (s *Service) EnsureGoogleDrive(ctx context.Context, rt *Runtime) error {
	p, err := s.GetProvider(rt.Provider)
	if err != nil {
		return err
	}
	driveProvider, ok := p.(interface {
		EnsureGoogleDrive(context.Context, *Runtime) error
	})
	if !ok {
		return fmt.Errorf("provider %s does not support Google Drive mounts", p.Name())
	}
	return driveProvider.EnsureGoogleDrive(ctx, rt)
}
