package colab

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type CommandOptions struct {
	Env map[string]string
	Dir string
}

type CommandResult struct {
	ExitCode int
	Stdout   []byte
	Stderr   []byte
}

type CommandRunner interface {
	Run(ctx context.Context, name string, args []string, opts CommandOptions) (*CommandResult, error)
}

type OSCommandRunner struct{}

func (r *OSCommandRunner) Run(ctx context.Context, name string, args []string, opts CommandOptions) (*CommandResult, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if opts.Dir != "" {
		cmd.Dir = opts.Dir
	}

	envList := os.Environ()
	for k, v := range opts.Env {
		envList = append(envList, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = envList

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, err
		}
	}

	return &CommandResult{
		ExitCode: exitCode,
		Stdout:   stdoutBuf.Bytes(),
		Stderr:   stderrBuf.Bytes(),
	}, nil
}

type ColabCLI struct {
	Binary  string
	Runner  CommandRunner
	Profile *Profile
}

func NewColabCLI(binary string, runner CommandRunner, profile *Profile) *ColabCLI {
	if binary == "" {
		binary = "colab"
	}
	if runner == nil {
		runner = &OSCommandRunner{}
	}
	return &ColabCLI{
		Binary:  binary,
		Runner:  runner,
		Profile: profile,
	}
}

func (c *ColabCLI) buildOptions() CommandOptions {
	opts := CommandOptions{
		Env: make(map[string]string),
	}
	if c.Profile != nil && c.Profile.HomeDir != "" {
		opts.Env["HOME"] = c.Profile.HomeDir
	}
	return opts
}

// Global options must precede the subcommand in Colab's Typer CLI.
func (c *ColabCLI) buildArgs(args ...string) []string {
	var global []string
	if c.Profile != nil {
		if c.Profile.ConfigFile != "" {
			global = append(global, "--config", c.Profile.ConfigFile)
		}
		if c.Profile.OAuthFile != "" {
			global = append(global, "--client-oauth-config", c.Profile.OAuthFile)
		}
	}
	return append(global, args...)
}

// ProxyCommand uses the same state and authentication settings as other calls.
func (c *ColabCLI) ProxyCommand(session, keyFile string) string {
	args := []string{c.Binary}
	if c.Profile != nil && c.Profile.HomeDir != "" {
		args = append([]string{"env", "HOME=" + c.Profile.HomeDir}, args...)
	}
	args = append(args, c.buildArgs("ssh", "--proxy-mode", "-s", session, "--identity", keyFile)...)
	for i, arg := range args {
		args[i] = "'" + strings.ReplaceAll(arg, "'", "'\"'\"'") + "'"
	}
	return strings.Join(args, " ")
}

func (c *ColabCLI) New(ctx context.Context, session, gpu string, highMem bool) (*CommandResult, error) {
	args := []string{"new", "-s", session}
	if gpu != "" {
		args = append(args, "--gpu", gpu)
	}
	if highMem {
		args = append(args, "--high-mem")
	}

	return c.Runner.Run(ctx, c.Binary, c.buildArgs(args...), c.buildOptions())
}

func (c *ColabCLI) Status(ctx context.Context, session string) (*CommandResult, error) {
	args := []string{"status"}
	if session != "" {
		args = append(args, "-s", session)
	}
	return c.Runner.Run(ctx, c.Binary, c.buildArgs(args...), c.buildOptions())
}

func (c *ColabCLI) Sessions(ctx context.Context) (*CommandResult, error) {
	args := []string{"sessions"}
	return c.Runner.Run(ctx, c.Binary, c.buildArgs(args...), c.buildOptions())
}

func (c *ColabCLI) Stop(ctx context.Context, session string) (*CommandResult, error) {
	args := []string{"stop", "-s", session}
	return c.Runner.Run(ctx, c.Binary, c.buildArgs(args...), c.buildOptions())
}

func (c *ColabCLI) DriveMount(ctx context.Context, session string) (*CommandResult, error) {
	return c.Runner.Run(ctx, c.Binary, c.buildArgs("drivemount", "-s", session, "/content/drive"), c.buildOptions())
}
