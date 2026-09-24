package container

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"cbox/internal/transport"
)

type Runner struct {
	logger *slog.Logger
}

func NewRunner(logger *slog.Logger) *Runner {
	if logger == nil {
		logger = slog.Default()
	}
	return &Runner{logger: logger}
}

func (r *Runner) SetupContainer(ctx context.Context, trans transport.Transport, c *Container, cmdToRun []string) error {
	containerDir := fmt.Sprintf("/content/.cbox/containers/%s", c.ID)
	mkdirCmd := fmt.Sprintf("mkdir -p %s", containerDir)
	_, err := trans.Exec(ctx, []string{"bash", "-c", mkdirCmd}, transport.ExecOptions{})
	if err != nil {
		return fmt.Errorf("create container dir: %w", err)
	}

	// 1. Write config.json
	cfgBytes, _ := json.MarshalIndent(c, "", "  ")
	writeCfg := fmt.Sprintf("cat << 'EOF' > %s/config.json\n%s\nEOF", containerDir, string(cfgBytes))
	_, _ = trans.Exec(ctx, []string{"bash", "-c", writeCfg}, transport.ExecOptions{})

	// 2. Write workdir
	workDir := c.WorkDir
	if workDir == "" {
		workDir = "/workspace"
	}
	writeWd := fmt.Sprintf("echo %q > %s/workdir", workDir, containerDir)
	_, _ = trans.Exec(ctx, []string{"bash", "-c", writeWd}, transport.ExecOptions{})

	// 3. Write env.sh
	var envLines []string
	envLines = append(envLines, "#!/usr/bin/env bash")
	for k, v := range c.Env {
		envLines = append(envLines, fmt.Sprintf("export %s=%q", k, v))
	}
	writeEnv := fmt.Sprintf("cat << 'EOF' > %s/env.sh\n%s\nEOF\nchmod +x %s/env.sh",
		containerDir, strings.Join(envLines, "\n"), containerDir)
	_, _ = trans.Exec(ctx, []string{"bash", "-c", writeEnv}, transport.ExecOptions{})

	// 4. Write cmd.sh
	execCmd := cmdToRun
	if len(execCmd) == 0 {
		execCmd = c.Command
	}
	var cmdLines []string
	cmdLines = append(cmdLines, "#!/usr/bin/env bash")
	cmdLines = append(cmdLines, "set -e")
	cmdLines = append(cmdLines, strings.Join(execCmd, " "))

	writeCmd := fmt.Sprintf("cat << 'EOF' > %s/cmd.sh\n%s\nEOF\nchmod +x %s/cmd.sh",
		containerDir, strings.Join(cmdLines, "\n"), containerDir)
	_, err = trans.Exec(ctx, []string{"bash", "-c", writeCmd}, transport.ExecOptions{})
	if err != nil {
		return fmt.Errorf("write cmd.sh: %w", err)
	}

	return nil
}

func (r *Runner) Launch(ctx context.Context, trans transport.Transport, c *Container) error {
	sessionName := fmt.Sprintf("cbox-%s", c.ID)
	launchCmd := fmt.Sprintf("tmux new-session -d -s %s '/content/.cbox/bin/entrypoint.sh %s'", sessionName, c.ID)
	r.logger.Info("launching container in remote tmux", "session", sessionName, "container_id", c.ID)

	res, err := trans.Exec(ctx, []string{"bash", "-c", launchCmd}, transport.ExecOptions{})
	if err != nil || res.ExitCode != 0 {
		// Fallback: if tmux is not present, run directly in background with nohup
		r.logger.Warn("tmux session launch failed, trying nohup fallback", "err", err, "stderr", string(res.Stderr))
		nohupCmd := fmt.Sprintf("nohup /content/.cbox/bin/entrypoint.sh %s >/dev/null 2>&1 &", c.ID)
		_, errFallback := trans.Exec(ctx, []string{"bash", "-c", nohupCmd}, transport.ExecOptions{})
		if errFallback != nil {
			return fmt.Errorf("launch container process: %w", errFallback)
		}
	}

	return nil
}

func (r *Runner) CheckStatus(ctx context.Context, trans transport.Transport, c *Container) (isRunning bool, exitCode *int, err error) {
	sessionName := fmt.Sprintf("cbox-%s", c.ID)
	tmuxCheck := fmt.Sprintf("tmux has-session -t %s 2>/dev/null", sessionName)
	res, err := trans.Exec(ctx, []string{"bash", "-c", tmuxCheck}, transport.ExecOptions{})

	if err == nil && res.ExitCode == 0 {
		return true, nil, nil
	}

	// Session is not running in tmux. Check state file or exit_code
	exitFile := fmt.Sprintf("/content/.cbox/containers/%s/exit_code", c.ID)
	readExit := fmt.Sprintf("cat %s 2>/dev/null", exitFile)
	exitRes, err := trans.Exec(ctx, []string{"bash", "-c", readExit}, transport.ExecOptions{})
	if err == nil && len(exitRes.Stdout) > 0 {
		codeStr := strings.TrimSpace(string(exitRes.Stdout))
		if code, errConv := strconv.Atoi(codeStr); errConv == nil {
			return false, &code, nil
		}
	}

	// Also check pid file
	pidFile := fmt.Sprintf("/content/.cbox/containers/%s/pid", c.ID)
	pidCheck := fmt.Sprintf("test -f %s && kill -0 $(cat %s 2>/dev/null) 2>/dev/null", pidFile, pidFile)
	pidRes, err := trans.Exec(ctx, []string{"bash", "-c", pidCheck}, transport.ExecOptions{})
	if err == nil && pidRes.ExitCode == 0 {
		return true, nil, nil
	}

	return false, nil, nil
}

func (r *Runner) ReadLogs(ctx context.Context, trans transport.Transport, c *Container) (stdout, stderr string, err error) {
	containerDir := fmt.Sprintf("/content/.cbox/containers/%s", c.ID)
	readCmd := fmt.Sprintf("cat %s/stdout.log 2>/dev/null; echo '---CBOX-DELIM---'; cat %s/stderr.log 2>/dev/null", containerDir, containerDir)

	res, err := trans.Exec(ctx, []string{"bash", "-c", readCmd}, transport.ExecOptions{})
	if err != nil {
		return "", "", err
	}

	parts := strings.Split(string(res.Stdout), "---CBOX-DELIM---\n")
	if len(parts) >= 2 {
		stdout = parts[0]
		stderr = parts[1]
	} else {
		stdout = string(res.Stdout)
	}

	return stdout, stderr, nil
}

func (r *Runner) Stop(ctx context.Context, trans transport.Transport, c *Container) error {
	sessionName := fmt.Sprintf("cbox-%s", c.ID)
	containerDir := fmt.Sprintf("/content/.cbox/containers/%s", c.ID)

	killCmd := fmt.Sprintf(`
	if tmux has-session -t %s 2>/dev/null; then
		tmux kill-session -t %s 2>/dev/null || true
	fi
	if [ -f %s/pid ]; then
		PID=$(cat %s/pid 2>/dev/null)
		kill -15 "$PID" 2>/dev/null || true
		sleep 1
		kill -9 "$PID" 2>/dev/null || true
	fi
	echo 130 > %s/exit_code
	echo stopped > %s/state
	`, sessionName, sessionName, containerDir, containerDir, containerDir, containerDir)

	_, _ = trans.Exec(ctx, []string{"bash", "-c", killCmd}, transport.ExecOptions{})
	return nil
}
