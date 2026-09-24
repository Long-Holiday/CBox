package transport

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	cboxErr "cbox/internal/errors"
)

type SSHTransport struct {
	Host       string
	ConfigFile string
	SSHBinary  string
	Logger     *slog.Logger
}

func NewSSHTransport(host, configFile string, logger *slog.Logger) *SSHTransport {
	if logger == nil {
		logger = slog.Default()
	}
	return &SSHTransport{
		Host:       host,
		ConfigFile: configFile,
		SSHBinary:  "ssh",
		Logger:     logger,
	}
}

func (t *SSHTransport) Exec(ctx context.Context, command []string, opts ExecOptions) (*ExecResult, error) {
	var args []string
	if t.ConfigFile != "" {
		args = append(args, "-F", t.ConfigFile)
	}

	if opts.Pty {
		args = append(args, "-tt")
	}

	args = append(args, t.Host, "--")

	// Construct shell command if Env or WorkDir specified
	remoteCmd := strings.Join(command, " ")
	if opts.WorkDir != "" || len(opts.Env) > 0 {
		var prefix []string
		for k, v := range opts.Env {
			prefix = append(prefix, fmt.Sprintf("export %s=%q;", k, v))
		}
		if opts.WorkDir != "" {
			prefix = append(prefix, fmt.Sprintf("cd %q;", opts.WorkDir))
		}
		remoteCmd = strings.Join(prefix, " ") + " " + remoteCmd
	}

	args = append(args, remoteCmd)

	cmd := exec.CommandContext(ctx, t.SSHBinary, args...)

	var stdoutBuf, stderrBuf bytes.Buffer
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

	if opts.Stdin != nil {
		cmd.Stdin = opts.Stdin
	}

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, &cboxErr.TransportError{
				Host:    t.Host,
				Op:      "exec",
				Message: "failed to start ssh process",
				Err:     err,
			}
		}
	}

	return &ExecResult{
		ExitCode: exitCode,
		Stdout:   stdoutBuf.Bytes(),
		Stderr:   stderrBuf.Bytes(),
	}, nil
}

func (t *SSHTransport) CopyTo(ctx context.Context, src, dst string) error {
	var args []string
	if t.ConfigFile != "" {
		args = append(args, "-F", t.ConfigFile)
	}
	args = append(args, "-r", src, fmt.Sprintf("%s:%s", t.Host, dst))

	cmd := exec.CommandContext(ctx, "scp", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return &cboxErr.TransportError{
			Host:    t.Host,
			Op:      "copy_to",
			Message: string(out),
			Err:     err,
		}
	}
	return nil
}

func (t *SSHTransport) CopyFrom(ctx context.Context, src, dst string) error {
	var args []string
	if t.ConfigFile != "" {
		args = append(args, "-F", t.ConfigFile)
	}
	args = append(args, "-r", fmt.Sprintf("%s:%s", t.Host, src), dst)

	cmd := exec.CommandContext(ctx, "scp", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return &cboxErr.TransportError{
			Host:    t.Host,
			Op:      "copy_from",
			Message: string(out),
			Err:     err,
		}
	}
	return nil
}

func (t *SSHTransport) SyncTo(ctx context.Context, src, dst string, opts SyncOptions) error {
	var sshOpt string
	if t.ConfigFile != "" {
		sshOpt = fmt.Sprintf("ssh -F %s", t.ConfigFile)
	} else {
		sshOpt = "ssh"
	}

	args := []string{"-az", "-e", sshOpt}
	for _, excl := range opts.Exclude {
		args = append(args, "--exclude", excl)
	}
	if opts.Delete {
		args = append(args, "--delete")
	}

	// Ensure trailing slash for directory content sync
	srcArg := src
	if info, err := os.Stat(src); err == nil && info.IsDir() && !strings.HasSuffix(srcArg, "/") {
		srcArg += "/"
	}

	args = append(args, srcArg, fmt.Sprintf("%s:%s", t.Host, dst))

	cmd := exec.CommandContext(ctx, "rsync", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return &cboxErr.TransportError{
			Host:    t.Host,
			Op:      "sync_to",
			Message: string(out),
			Err:     err,
		}
	}
	return nil
}

func (t *SSHTransport) SyncFrom(ctx context.Context, src, dst string, opts SyncOptions) error {
	var sshOpt string
	if t.ConfigFile != "" {
		sshOpt = fmt.Sprintf("ssh -F %s", t.ConfigFile)
	} else {
		sshOpt = "ssh"
	}

	args := []string{"-az", "-e", sshOpt}
	for _, excl := range opts.Exclude {
		args = append(args, "--exclude", excl)
	}
	if opts.Delete {
		args = append(args, "--delete")
	}

	srcArg := src
	if !strings.HasSuffix(srcArg, "/") {
		srcArg += "/"
	}

	args = append(args, fmt.Sprintf("%s:%s", t.Host, srcArg), dst)

	cmd := exec.CommandContext(ctx, "rsync", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return &cboxErr.TransportError{
			Host:    t.Host,
			Op:      "sync_from",
			Message: string(out),
			Err:     err,
		}
	}
	return nil
}

func GenerateSSHConfig(host, user, keyFile, proxyCommand, controlDir string) string {
	controlPath := filepath.Join(controlDir, "%C")
	return fmt.Sprintf(`Host %s
    User %s
    IdentityFile %s
    ProxyCommand %s
    ControlMaster auto
    ControlPersist 600
    ControlPath %s
    StrictHostKeyChecking no
    UserKnownHostsFile /dev/null
    LogLevel ERROR
`, host, user, keyFile, proxyCommand, controlPath)
}
