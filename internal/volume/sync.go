package volume

import (
	"context"
	"log/slog"
	"time"

	"cbox/internal/transport"
)

type SyncManager struct {
	interval time.Duration
	logger   *slog.Logger
}

func NewSyncManager(interval time.Duration, logger *slog.Logger) *SyncManager {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &SyncManager{
		interval: interval,
		logger:   logger,
	}
}

func (m *SyncManager) StartOutputSync(ctx context.Context, trans transport.Transport, mounts []Mount) {
	var outputMounts []Mount
	for _, mnt := range mounts {
		if mnt.Mode == ModeOutput {
			outputMounts = append(outputMounts, mnt)
		}
	}

	if len(outputMounts) == 0 {
		return
	}

	ticker := time.NewTicker(m.interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.syncOutputs(ctx, trans, outputMounts)
			}
		}
	}()
}

func (m *SyncManager) syncOutputs(ctx context.Context, trans transport.Transport, mounts []Mount) {
	for _, mnt := range mounts {
		if mnt.Mode != ModeOutput {
			continue
		}
		m.logger.Debug("syncing output volume", "target", mnt.Target, "source", mnt.Source)
		err := trans.SyncFrom(ctx, mnt.Target, mnt.Source, transport.SyncOptions{})
		if err != nil {
			m.logger.Warn("periodic sync output failed", "target", mnt.Target, "source", mnt.Source, "err", err)
		}
	}
}

func (m *SyncManager) FinalSyncOutputs(ctx context.Context, trans transport.Transport, mounts []Mount) error {
	for _, mnt := range mounts {
		if mnt.Mode != ModeOutput {
			continue
		}
		m.logger.Info("running final output sync", "target", mnt.Target, "source", mnt.Source)
		if err := trans.SyncFrom(ctx, mnt.Target, mnt.Source, transport.SyncOptions{}); err != nil {
			return err
		}
	}
	return nil
}
