package colab

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	cboxErr "cbox/internal/errors"
	"cbox/internal/runtime"
	"cbox/internal/transport"
	"github.com/google/uuid"
)

type ColabProvider struct {
	binary   string
	profiles *ProfileStore
	runner   CommandRunner
	sshDir   string
	keysDir  string
	logger   *slog.Logger
}

func NewColabProvider(binary string, profiles *ProfileStore, runner CommandRunner, sshDir, keysDir string, logger *slog.Logger) *ColabProvider {
	if binary == "" {
		binary = "colab"
	}
	if runner == nil {
		runner = &OSCommandRunner{}
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &ColabProvider{
		binary:   binary,
		profiles: profiles,
		runner:   runner,
		sshDir:   sshDir,
		keysDir:  keysDir,
		logger:   logger,
	}
}

func (p *ColabProvider) Name() string {
	return "colab"
}

func (p *ColabProvider) getCLI(profileName string) (*ColabCLI, *Profile, error) {
	profile, err := p.profiles.EnsureProfile(profileName)
	if err != nil {
		return nil, nil, fmt.Errorf("ensure profile %s: %w", profileName, err)
	}
	cli := NewColabCLI(p.binary, p.runner, profile)
	return cli, profile, nil
}

func (p *ColabProvider) ensureWorkerKey() (string, error) {
	keyPath := filepath.Join(p.keysDir, "worker")
	if _, err := os.Stat(keyPath); err == nil {
		return keyPath, nil
	}

	if err := os.MkdirAll(p.keysDir, 0700); err != nil {
		return "", err
	}

	// Generate ed25519 key if not present
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := p.runner
	_, err := cmd.Run(ctx, "ssh-keygen", []string{"-t", "ed25519", "-f", keyPath, "-N", "", "-C", "cbox-worker"}, CommandOptions{})
	if err != nil {
		// Fallback to dummy key if ssh-keygen isn't available
		_ = os.WriteFile(keyPath, []byte("dummy-key"), 0600)
	}
	return keyPath, nil
}

func (p *ColabProvider) CreateRuntime(ctx context.Context, req runtime.RuntimeRequest) (*runtime.RuntimeHandle, error) {
	cli, profile, err := p.getCLI(req.Profile)
	if err != nil {
		return nil, err
	}

	session := req.Session
	if session == "" {
		session = "cbox-" + uuid.New().String()[:8]
	}

	p.logger.Info("creating colab runtime", "session", session, "gpu", req.GPU, "profile", profile.Name)

	res, err := cli.New(ctx, session, req.GPU, req.HighMemory)
	if err != nil {
		return nil, &cboxErr.ProviderError{
			Provider: "colab",
			Op:       "create_runtime",
			Err:      err,
		}
	}
	if res.ExitCode != 0 {
		return nil, ParseCLIError(p.binary, res)
	}

	keyFile, err := p.ensureWorkerKey()
	if err != nil {
		p.logger.Warn("could not ensure worker key", "err", err)
	}

	// Generate SSH config file
	host := session
	sshConfigFile := filepath.Join(p.sshDir, session+".conf")
	proxyCmd := fmt.Sprintf("%s ssh --proxy-mode -s %s -i %s", p.binary, session, keyFile)
	if profile.ConfigFile != "" {
		proxyCmd = fmt.Sprintf("%s --config %s ssh --proxy-mode -s %s -i %s", p.binary, profile.ConfigFile, session, keyFile)
	}

	controlDir := filepath.Join(filepath.Dir(p.sshDir), "run", "ssh")
	_ = os.MkdirAll(controlDir, 0700)
	_ = os.MkdirAll(p.sshDir, 0700)

	cfgContent := transport.GenerateSSHConfig(host, "root", keyFile, proxyCmd, controlDir)
	if err := os.WriteFile(sshConfigFile, []byte(cfgContent), 0600); err != nil {
		return nil, fmt.Errorf("write ssh config: %w", err)
	}

	handle := &runtime.RuntimeHandle{
		ID:            session,
		Session:       session,
		Host:          host,
		SSHConfigFile: sshConfigFile,
		User:          "root",
	}

	return handle, nil
}

func (p *ColabProvider) GetRuntime(ctx context.Context, rt *runtime.Runtime) (*runtime.RuntimeStatus, error) {
	cli, _, err := p.getCLI(rt.Profile)
	if err != nil {
		return nil, err
	}

	res, err := cli.Status(ctx, rt.Session)
	if err != nil {
		return &runtime.RuntimeStatus{
			ID:       rt.ID,
			State:    runtime.StateLost,
			Alive:    false,
			LastSeen: time.Now().UTC(),
		}, nil
	}

	if res.ExitCode != 0 {
		return &runtime.RuntimeStatus{
			ID:       rt.ID,
			State:    runtime.StateLost,
			Alive:    false,
			LastSeen: time.Now().UTC(),
		}, nil
	}

	info, err := ParseStatus(res.Stdout)
	if err != nil {
		return &runtime.RuntimeStatus{
			ID:       rt.ID,
			State:    runtime.StateReady,
			Alive:    true,
			LastSeen: time.Now().UTC(),
		}, nil
	}

	state := runtime.StateReady
	if !info.Alive || info.State == "stopped" {
		state = runtime.StateStopped
	}

	return &runtime.RuntimeStatus{
		ID:       rt.ID,
		State:    state,
		GPU:      info.GPU,
		Alive:    info.Alive,
		LastSeen: time.Now().UTC(),
	}, nil
}

func (p *ColabProvider) StopRuntime(ctx context.Context, rt *runtime.Runtime) error {
	cli, _, err := p.getCLI(rt.Profile)
	if err != nil {
		return err
	}

	p.logger.Info("stopping colab runtime", "session", rt.Session)
	res, err := cli.Stop(ctx, rt.Session)
	if err != nil {
		return &cboxErr.ProviderError{
			Provider: "colab",
			Op:       "stop_runtime",
			Err:      err,
		}
	}
	if res.ExitCode != 0 {
		return ParseCLIError(p.binary, res)
	}

	// Clean up ssh config if present
	sshConfigFile := filepath.Join(p.sshDir, rt.Session+".conf")
	_ = os.Remove(sshConfigFile)

	return nil
}

func (p *ColabProvider) ListRuntimes(ctx context.Context) ([]runtime.RuntimeStatus, error) {
	cli, _, err := p.getCLI("default")
	if err != nil {
		return nil, err
	}

	res, err := cli.Sessions(ctx)
	if err != nil {
		return nil, &cboxErr.ProviderError{
			Provider: "colab",
			Op:       "list_runtimes",
			Err:      err,
		}
	}
	if res.ExitCode != 0 {
		return nil, ParseCLIError(p.binary, res)
	}

	sessions, err := ParseSessions(res.Stdout)
	if err != nil {
		return nil, err
	}

	var list []runtime.RuntimeStatus
	for _, s := range sessions {
		st := runtime.StateReady
		if s.State == "stopped" {
			st = runtime.StateStopped
		}
		list = append(list, runtime.RuntimeStatus{
			ID:       s.Session,
			State:    st,
			GPU:      s.GPU,
			Alive:    st == runtime.StateReady,
			LastSeen: time.Now().UTC(),
		})
	}

	return list, nil
}

func (p *ColabProvider) OpenTransport(ctx context.Context, rt *runtime.Runtime) (transport.Transport, error) {
	sshConfigFile := filepath.Join(p.sshDir, rt.Session+".conf")
	host := rt.Session
	return transport.NewSSHTransport(host, sshConfigFile, p.logger), nil
}
