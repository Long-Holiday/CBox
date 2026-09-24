package transport

import (
	"context"
	"io"
)

type ExecOptions struct {
	Env     map[string]string
	WorkDir string
	Pty     bool
	Stdin   io.Reader
	Stdout  io.Writer
	Stderr  io.Writer
}

type ExecResult struct {
	ExitCode int
	Stdout   []byte
	Stderr   []byte
}

type SyncOptions struct {
	Exclude []string
	Delete  bool
}

type Transport interface {
	Exec(ctx context.Context, command []string, opts ExecOptions) (*ExecResult, error)
	CopyTo(ctx context.Context, src string, dst string) error
	CopyFrom(ctx context.Context, src string, dst string) error
	SyncTo(ctx context.Context, src string, dst string, opts SyncOptions) error
	SyncFrom(ctx context.Context, src string, dst string, opts SyncOptions) error
}
