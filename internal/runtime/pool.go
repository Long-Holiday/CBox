package runtime

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

type Pool struct {
	mu          sync.RWMutex
	idleTimeout time.Duration
	stopFunc    func(ctx context.Context, rt *Runtime) error
	logger      *slog.Logger
	runtimes    map[string]*Runtime
}

func NewPool(idleTimeout time.Duration, stopFunc func(ctx context.Context, rt *Runtime) error, logger *slog.Logger) *Pool {
	if idleTimeout <= 0 {
		idleTimeout = 30 * time.Minute
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Pool{
		idleTimeout: idleTimeout,
		stopFunc:    stopFunc,
		logger:      logger,
		runtimes:    make(map[string]*Runtime),
	}
}

func (p *Pool) Track(rt *Runtime) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.runtimes[rt.ID] = rt
}

func (p *Pool) Untrack(id string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.runtimes, id)
}

func (p *Pool) Get(id string) *Runtime {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.runtimes[id]
}

func (p *Pool) List() []*Runtime {
	p.mu.RLock()
	defer p.mu.RUnlock()
	var list []*Runtime
	for _, rt := range p.runtimes {
		list = append(list, rt)
	}
	return list
}

func (p *Pool) MarkIdle(id string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if rt, ok := p.runtimes[id]; ok {
		rt.State = StateIdle
		rt.LastSeen = time.Now().UTC()
	}
}

func (p *Pool) MarkBusy(id string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if rt, ok := p.runtimes[id]; ok {
		rt.State = StateBusy
		rt.LastSeen = time.Now().UTC()
	}
}

func (p *Pool) StartReaper(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = time.Minute
	}
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				p.reapExpired(ctx)
			}
		}
	}()
}

func (p *Pool) reapExpired(ctx context.Context) {
	p.mu.Lock()
	now := time.Now().UTC()
	var toStop []*Runtime

	for _, rt := range p.runtimes {
		if rt.State == StateIdle && now.Sub(rt.LastSeen) > p.idleTimeout {
			toStop = append(toStop, rt)
		}
	}
	p.mu.Unlock()

	for _, rt := range toStop {
		p.logger.Info("reaping idle runtime due to timeout", "runtime_id", rt.ID, "last_seen", rt.LastSeen)
		if p.stopFunc != nil {
			if err := p.stopFunc(ctx, rt); err != nil {
				p.logger.Error("failed to stop expired runtime", "runtime_id", rt.ID, "err", err)
			}
		}
	}
}
