package context

import (
	"context"
	"fmt"
	"log/slog"

	cboxErr "cbox/internal/errors"
)

type Repository interface {
	Create(ctx context.Context, c *Context) error
	Get(ctx context.Context, name string) (*Context, error)
	GetCurrent(ctx context.Context) (*Context, error)
	SetCurrent(ctx context.Context, name string) error
	List(ctx context.Context) ([]*Context, error)
	Delete(ctx context.Context, name string) error
}

type Service struct {
	repo   Repository
	logger *slog.Logger
}

func NewService(repo Repository, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		repo:   repo,
		logger: logger,
	}
}

func (s *Service) Create(ctx context.Context, name, provider, profile string, autoSchedule bool) (*Context, error) {
	if name == "" {
		return nil, fmt.Errorf("context name cannot be empty")
	}
	if provider == "" {
		provider = "colab"
	}
	if profile == "" {
		profile = "default"
	}

	c := &Context{
		Name:         name,
		Provider:     provider,
		Profile:      profile,
		AutoSchedule: autoSchedule,
		IsCurrent:    false,
	}

	if s.repo != nil {
		if err := s.repo.Create(ctx, c); err != nil {
			return nil, err
		}
	}

	return c, nil
}

func (s *Service) Get(ctx context.Context, name string) (*Context, error) {
	if s.repo != nil {
		return s.repo.Get(ctx, name)
	}
	return nil, cboxErr.ErrNotFound
}

func (s *Service) GetCurrent(ctx context.Context) (*Context, error) {
	if s.repo != nil {
		c, err := s.repo.GetCurrent(ctx)
		if err == nil {
			return c, nil
		}
	}
	// Default fallback
	return &Context{
		Name:         "default",
		Provider:     "colab",
		Profile:      "default",
		AutoSchedule: true,
		IsCurrent:    true,
	}, nil
}

func (s *Service) Use(ctx context.Context, name string) error {
	if s.repo != nil {
		return s.repo.SetCurrent(ctx, name)
	}
	return nil
}

func (s *Service) List(ctx context.Context) ([]*Context, error) {
	if s.repo != nil {
		return s.repo.List(ctx)
	}
	return nil, nil
}

func (s *Service) Delete(ctx context.Context, name string) error {
	if s.repo != nil {
		return s.repo.Delete(ctx, name)
	}
	return nil
}
