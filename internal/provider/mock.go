package provider

import (
	"context"
	"fmt"
	"sync"
	"time"

	cboxErr "cbox/internal/errors"
	"cbox/internal/runtime"
	"cbox/internal/transport"
	"github.com/google/uuid"
)

type MockProvider struct {
	mu           sync.Mutex
	name         string
	runtimes     map[string]*runtime.RuntimeStatus
	transports   map[string]transport.Transport
	FailCreate   bool
	FailGPU      string
	MockTrans    *transport.MockTransport
	UseLocalExec bool
	LocalBaseDir string
}

func NewMockProvider(name string) *MockProvider {
	if name == "" {
		name = "mock"
	}
	return &MockProvider{
		name:       name,
		runtimes:   make(map[string]*runtime.RuntimeStatus),
		transports: make(map[string]transport.Transport),
	}
}

func (p *MockProvider) Name() string {
	return p.name
}

func (p *MockProvider) CreateRuntime(ctx context.Context, req runtime.RuntimeRequest) (*runtime.RuntimeHandle, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.FailCreate {
		return nil, &cboxErr.AllocationError{
			GPU:    req.GPU,
			Reason: "simulated allocation failure",
		}
	}

	if p.FailGPU != "" && req.GPU == p.FailGPU {
		return nil, &cboxErr.AllocationError{
			GPU:    req.GPU,
			Reason: fmt.Sprintf("gpu %s not available", req.GPU),
		}
	}

	id := "mock-" + uuid.New().String()[:8]
	session := req.Session
	if session == "" {
		session = id
	}

	status := &runtime.RuntimeStatus{
		ID:       id,
		State:    runtime.StateReady,
		GPU:      req.GPU,
		Alive:    true,
		LastSeen: time.Now().UTC(),
	}
	p.runtimes[id] = status

	// Assign transport
	var trans transport.Transport
	if p.MockTrans != nil {
		trans = p.MockTrans
	} else {
		mockT := transport.NewMockTransport()
		mockT.UseLocalExec = p.UseLocalExec
		mockT.LocalBaseDir = p.LocalBaseDir
		trans = mockT
	}
	p.transports[id] = trans

	return &runtime.RuntimeHandle{
		ID:            id,
		Session:       session,
		Host:          "localhost",
		SSHConfigFile: "",
		User:          "root",
	}, nil
}

func (p *MockProvider) GetRuntime(ctx context.Context, rt *runtime.Runtime) (*runtime.RuntimeStatus, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	status, ok := p.runtimes[rt.ID]
	if !ok {
		return &runtime.RuntimeStatus{
			ID:       rt.ID,
			State:    runtime.StateLost,
			Alive:    false,
			LastSeen: time.Now().UTC(),
		}, nil
	}
	return status, nil
}

func (p *MockProvider) StopRuntime(ctx context.Context, rt *runtime.Runtime) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if status, ok := p.runtimes[rt.ID]; ok {
		status.State = runtime.StateStopped
		status.Alive = false
	}
	return nil
}

func (p *MockProvider) ListRuntimes(ctx context.Context) ([]runtime.RuntimeStatus, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	var list []runtime.RuntimeStatus
	for _, s := range p.runtimes {
		list = append(list, *s)
	}
	return list, nil
}

func (p *MockProvider) OpenTransport(ctx context.Context, rt *runtime.Runtime) (transport.Transport, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if trans, ok := p.transports[rt.ID]; ok {
		return trans, nil
	}

	mockT := transport.NewMockTransport()
	mockT.UseLocalExec = p.UseLocalExec
	mockT.LocalBaseDir = p.LocalBaseDir
	p.transports[rt.ID] = mockT
	return mockT, nil
}
