package provider

import (
	"context"

	"cbox/internal/runtime"
	"cbox/internal/transport"
)

type Provider interface {
	Name() string

	CreateRuntime(ctx context.Context, req runtime.RuntimeRequest) (*runtime.RuntimeHandle, error)

	GetRuntime(ctx context.Context, rt *runtime.Runtime) (*runtime.RuntimeStatus, error)

	StopRuntime(ctx context.Context, rt *runtime.Runtime) error

	ListRuntimes(ctx context.Context) ([]runtime.RuntimeStatus, error)

	OpenTransport(ctx context.Context, rt *runtime.Runtime) (transport.Transport, error)
}
