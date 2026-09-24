package transport

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

type MockTransport struct {
	mu           sync.Mutex
	CommandsRun  [][]string
	FilesCopied  map[string]string
	ExecHandler  func(ctx context.Context, command []string, opts ExecOptions) (*ExecResult, error)
	UseLocalExec bool
	LocalBaseDir string
}

func NewMockTransport() *MockTransport {
	return &MockTransport{
		FilesCopied: make(map[string]string),
	}
}

func (m *MockTransport) Exec(ctx context.Context, command []string, opts ExecOptions) (*ExecResult, error) {
	m.mu.Lock()
	m.CommandsRun = append(m.CommandsRun, command)
	handler := m.ExecHandler
	useLocal := m.UseLocalExec
	baseDir := m.LocalBaseDir
	m.mu.Unlock()

	if handler != nil {
		return handler(ctx, command, opts)
	}

	if useLocal && len(command) > 0 {
		// Run locally for integration testing
		cmdStr := strings.Join(command, " ")
		cmd := exec.CommandContext(ctx, "bash", "-c", cmdStr)
		if opts.WorkDir != "" {
			if _, err := os.Stat(opts.WorkDir); err == nil {
				cmd.Dir = opts.WorkDir
			} else if baseDir != "" {
				subDir := filepath.Join(baseDir, strings.TrimPrefix(opts.WorkDir, "/"))
				_ = os.MkdirAll(subDir, 0755)
				cmd.Dir = subDir
			}
		}
		var envList []string
		for k, v := range opts.Env {
			envList = append(envList, k+"="+v)
		}
		if len(envList) > 0 {
			cmd.Env = append(os.Environ(), envList...)
		}
		var stdoutBuf, stderrBuf strings.Builder
		if opts.Stdout != nil {
			cmd.Stdout = io.MultiWriter(&stdoutBuf, opts.Stdout)
		} else {
			cmd.Stdout = &stdoutBuf
		}
		if opts.Stderr != nil {
			cmd.Stderr = io.MultiWriter(&stderrBuf, opts.Stderr)
		} else {
			cmd.Stderr = &stderrBuf
		}

		err := cmd.Run()
		exitCode := 0
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				exitCode = 1
			}
		}
		return &ExecResult{
			ExitCode: exitCode,
			Stdout:   []byte(stdoutBuf.String()),
			Stderr:   []byte(stderrBuf.String()),
		}, nil
	}

	// Default dummy success
	return &ExecResult{
		ExitCode: 0,
		Stdout:   []byte("ok"),
		Stderr:   nil,
	}, nil
}

func (m *MockTransport) CopyTo(ctx context.Context, src, dst string) error {
	m.mu.Lock()
	m.FilesCopied[src] = dst
	useLocal := m.UseLocalExec
	baseDir := m.LocalBaseDir
	m.mu.Unlock()

	if useLocal && baseDir != "" {
		targetDst := filepath.Join(baseDir, dst)
		_ = os.MkdirAll(filepath.Dir(targetDst), 0755)
		data, err := os.ReadFile(src)
		if err == nil {
			_ = os.WriteFile(targetDst, data, 0644)
		}
	}
	return nil
}

func (m *MockTransport) CopyFrom(ctx context.Context, src, dst string) error {
	m.mu.Lock()
	m.FilesCopied[src] = dst
	m.mu.Unlock()
	return nil
}

func (m *MockTransport) SyncTo(ctx context.Context, src, dst string, opts SyncOptions) error {
	m.mu.Lock()
	m.FilesCopied[src] = dst
	m.mu.Unlock()
	return nil
}

func (m *MockTransport) SyncFrom(ctx context.Context, src, dst string, opts SyncOptions) error {
	m.mu.Lock()
	m.FilesCopied[src] = dst
	m.mu.Unlock()
	return nil
}
