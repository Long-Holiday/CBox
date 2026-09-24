package image

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	cboxErr "cbox/internal/errors"
)

type Repository interface {
	Create(ctx context.Context, img *Image) error
	Get(ctx context.Context, idOrTag string) (*Image, error)
	AddTag(ctx context.Context, imageID string, tag string) error
	List(ctx context.Context) ([]*Image, error)
	Delete(ctx context.Context, idOrTag string) error
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

func (s *Service) Build(ctx context.Context, contextDir, cboxfilePath, tag string) (*Image, error) {
	if contextDir == "" {
		contextDir = "."
	}
	absContext, err := filepath.Abs(contextDir)
	if err == nil {
		contextDir = absContext
	}

	if cboxfilePath == "" {
		cboxfilePath = filepath.Join(contextDir, "Cboxfile")
	}

	manifest, err := ParseCboxfileFile(cboxfilePath)
	if err != nil {
		return nil, fmt.Errorf("parse cboxfile %s: %w", cboxfilePath, err)
	}

	// Compute image hash
	h := sha256.New()
	manifestBytes, _ := json.Marshal(manifest)
	h.Write(manifestBytes)

	// Hash contents of referenced COPY files if they exist in contextDir
	for _, copySpec := range manifest.Copies {
		srcPath := filepath.Join(contextDir, copySpec.Source)
		if data, err := os.ReadFile(srcPath); err == nil {
			h.Write([]byte(copySpec.Source))
			h.Write(data)
		}
	}

	imageID := hex.EncodeToString(h.Sum(nil))[:12]

	var tags []string
	if tag != "" {
		tags = append(tags, tag)
	}

	img := &Image{
		ID:        imageID,
		Tags:      tags,
		Manifest:  *manifest,
		CreatedAt: time.Now().UTC(),
	}

	if s.repo != nil {
		if err := s.repo.Create(ctx, img); err != nil {
			return nil, fmt.Errorf("save image to repo: %w", err)
		}
	}

	s.logger.Info("built image", "id", img.ID, "tag", tag)
	return img, nil
}

func (s *Service) Get(ctx context.Context, idOrTag string) (*Image, error) {
	if s.repo != nil {
		return s.repo.Get(ctx, idOrTag)
	}
	return nil, cboxErr.ErrNotFound
}

func (s *Service) List(ctx context.Context) ([]*Image, error) {
	if s.repo != nil {
		return s.repo.List(ctx)
	}
	return nil, nil
}

func (s *Service) Delete(ctx context.Context, idOrTag string) error {
	if s.repo != nil {
		return s.repo.Delete(ctx, idOrTag)
	}
	return nil
}
