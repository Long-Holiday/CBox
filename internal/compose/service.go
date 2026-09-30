package compose

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"cbox/internal/container"
	pkgApi "cbox/pkg/api"
)

type Service struct {
	containerService *container.Service
	logger           *slog.Logger
}

func NewService(containerService *container.Service, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		containerService: containerService,
		logger:           logger,
	}
}

func (s *Service) resolveProjectName(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "cbox"
	}
	dir := filepath.Dir(abs)
	return filepath.Base(dir)
}

func (s *Service) Up(ctx context.Context, composeFile string, detach bool) ([]*container.Container, error) {
	cfg, err := ParseComposeFile(composeFile)
	if err != nil {
		return nil, err
	}

	projectName := s.resolveProjectName(composeFile)
	var started []*container.Container

	for svcName, svc := range cfg.Services {
		containerName := fmt.Sprintf("%s-%s", projectName, svcName)
		gpuPref := ExtractGPUPreferences(svc.GPU)
		volSpecs, err := ParseVolumeSpecs(svc.Volumes)
		if err != nil {
			return nil, fmt.Errorf("service %s: %w", svcName, err)
		}

		s.logger.Info("compose: bringing up service", "service", svcName, "container", containerName)

		req := pkgApi.ContainerCreateRequest{
			Name:          containerName,
			Image:         svc.Image,
			GPUPreference: gpuPref,
			Command:       svc.Command,
			Env:           svc.Env,
			WorkDir:       svc.WorkDir,
			Volumes:       volSpecs,
			RestartPolicy: svc.Restart,
		}

		c, err := s.containerService.CreateContainer(ctx, req)
		if err != nil {
			// If already exists, attempt to use existing
			existing, getErr := s.containerService.GetContainer(ctx, containerName)
			if getErr == nil {
				c = existing
			} else {
				return nil, fmt.Errorf("create container for service %s: %w", svcName, err)
			}
		}

		startedCont, err := s.containerService.StartContainer(ctx, c.ID, detach)
		if err != nil {
			s.logger.Error("failed to start compose service", "service", svcName, "err", err)
			return nil, fmt.Errorf("start service %s: %w", svcName, err)
		}
		started = append(started, startedCont)
	}

	return started, nil
}

func (s *Service) Down(ctx context.Context, composeFile string) error {
	cfg, err := ParseComposeFile(composeFile)
	if err != nil {
		return err
	}

	projectName := s.resolveProjectName(composeFile)

	for svcName := range cfg.Services {
		containerName := fmt.Sprintf("%s-%s", projectName, svcName)
		s.logger.Info("compose: stopping and removing service", "service", svcName, "container", containerName)

		_, _ = s.containerService.StopContainer(ctx, containerName, 5)
		_ = s.containerService.RemoveContainer(ctx, containerName, true)
	}

	return nil
}

func (s *Service) Ps(ctx context.Context, composeFile string) ([]*container.Container, error) {
	cfg, err := ParseComposeFile(composeFile)
	if err != nil {
		return nil, err
	}

	projectName := s.resolveProjectName(composeFile)
	var list []*container.Container

	for svcName := range cfg.Services {
		containerName := fmt.Sprintf("%s-%s", projectName, svcName)
		if c, err := s.containerService.GetContainer(ctx, containerName); err == nil {
			list = append(list, c)
		}
	}

	return list, nil
}

func (s *Service) Logs(ctx context.Context, composeFile string) (map[string]string, error) {
	cfg, err := ParseComposeFile(composeFile)
	if err != nil {
		return nil, err
	}

	projectName := s.resolveProjectName(composeFile)
	logsMap := make(map[string]string)

	for svcName := range cfg.Services {
		containerName := fmt.Sprintf("%s-%s", projectName, svcName)
		stdout, stderr, err := s.containerService.GetLogs(ctx, containerName)
		if err == nil {
			logContent := stdout
			if stderr != "" {
				logContent += "\n[stderr]\n" + stderr
			}
			logsMap[svcName] = strings.TrimSpace(logContent)
		}
	}

	return logsMap, nil
}
