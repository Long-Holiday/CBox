package runtime

import (
	"context"
	"log/slog"
	"time"
)

type Monitor struct {
	service  *Service
	interval time.Duration
	logger   *slog.Logger
}

func NewMonitor(service *Service, interval time.Duration, logger *slog.Logger) *Monitor {
	if interval <= 0 {
		interval = 15 * time.Second
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Monitor{
		service:  service,
		interval: interval,
		logger:   logger,
	}
}

func (m *Monitor) Start(ctx context.Context) {
	ticker := time.NewTicker(m.interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.pollRuntimes(ctx)
			}
		}
	}()
}

func (m *Monitor) pollRuntimes(ctx context.Context) {
	runtimes := m.service.Pool().List()
	for _, rt := range runtimes {
		if rt.State == StateStopped || rt.State == StateLost {
			continue
		}

		p, err := m.service.GetProvider(rt.Provider)
		if err != nil {
			continue
		}

		status, err := p.GetRuntime(ctx, rt)
		if err != nil || !status.Alive {
			m.logger.Warn("runtime lost or unreachable", "runtime_id", rt.ID, "err", err)
			rt.State = StateLost
			if m.service.repo != nil {
				_ = m.service.repo.UpdateState(ctx, rt.ID, StateLost)
			}
			continue
		}

		rt.LastSeen = time.Now().UTC()
		if m.service.repo != nil {
			_ = m.service.repo.UpdateLastSeen(ctx, rt.ID)
		}
	}
}
