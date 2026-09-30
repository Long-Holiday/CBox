package colab

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	cboxErr "cbox/internal/errors"
)

func TestParseSessions(t *testing.T) {
	output := []byte(`
SESSION             GPU      STATE
cbox-abc12345       T4       RUNNING
cbox-def67890       L4       STOPPED
`)

	sessions, err := ParseSessions(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}
	if sessions[0].Session != "cbox-abc12345" || sessions[0].GPU != "T4" {
		t.Fatalf("unexpected session 0: %+v", sessions[0])
	}
}

func TestParseStatus(t *testing.T) {
	output := []byte(`
Session: cbox-test
GPU: L4
Status: RUNNING
`)
	status, err := ParseStatus(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if status.Session != "cbox-test" || status.GPU != "L4" || !status.Alive {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func TestParseCLIError(t *testing.T) {
	// Allocation error
	res1 := &CommandResult{
		ExitCode: 1,
		Stderr:   []byte("Error: GPU not available in region"),
	}
	err1 := ParseCLIError("colab", res1)
	var allocErr *cboxErr.AllocationError
	if !errors.As(err1, &allocErr) {
		t.Fatalf("expected AllocationError, got %T: %v", err1, err1)
	}

	// Auth error
	res2 := &CommandResult{
		ExitCode: 1,
		Stderr:   []byte("Please login to google colab"),
	}
	err2 := ParseCLIError("colab", res2)
	var authErr *cboxErr.AuthenticationError
	if !errors.As(err2, &authErr) {
		t.Fatalf("expected AuthenticationError, got %T: %v", err2, err2)
	}
	for _, message := range []string{"[colab] Allocation refused (precondition failed).", "[colab] Backend rejected accelerator 'T4'."} {
		err := ParseCLIError("colab", &CommandResult{ExitCode: 1, Stderr: []byte(message)})
		if !errors.As(err, &allocErr) {
			t.Fatalf("expected allocation error: %v", err)
		}
	}
	networkErr := ParseCLIError("colab", &CommandResult{ExitCode: 1, Stderr: []byte("SSLError: HTTPSConnectionPool /tun/m/assign?authuser=0 UNEXPECTED_EOF_WHILE_READING")})
	var commandErr *cboxErr.CommandError
	if !errors.As(networkErr, &commandErr) {
		t.Fatalf("network error was misclassified: %v", networkErr)
	}
}

type mockRunner struct {
	lastCmd  string
	lastArgs []string
	lastOpts CommandOptions
}

func (m *mockRunner) Run(ctx context.Context, name string, args []string, opts CommandOptions) (*CommandResult, error) {
	m.lastCmd = name
	m.lastArgs = args
	m.lastOpts = opts
	return &CommandResult{
		ExitCode: 0,
		Stdout:   []byte("ok"),
	}, nil
}

func TestColabCLICommands(t *testing.T) {
	mr := &mockRunner{}
	profile := &Profile{
		Name:    "test-prof",
		HomeDir: "/tmp/test-home",
	}
	cli := NewColabCLI("colab", mr, profile)

	_, err := cli.New(context.Background(), "my-sess", "L4", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"new", "-s", "my-sess", "--gpu", "L4", "--high-mem"}
	if mr.lastCmd != "colab" || !reflect.DeepEqual(mr.lastArgs, want) {
		t.Fatalf("unexpected command call: %s %v", mr.lastCmd, mr.lastArgs)
	}
}

func TestColabProfileCommands(t *testing.T) {
	profile := &Profile{HomeDir: "/tmp/profile home", ConfigFile: "/tmp/profile/config.json", OAuthFile: "/tmp/profile/oauth.json"}
	mr := &mockRunner{}
	cli := NewColabCLI("/opt/bin/colab", mr, profile)
	prefix := []string{"--config", profile.ConfigFile, "--client-oauth-config", profile.OAuthFile}
	for _, tc := range []struct {
		args []string
		call func() (*CommandResult, error)
	}{
		{[]string{"new", "-s", "my-sess"}, func() (*CommandResult, error) { return cli.New(context.Background(), "my-sess", "", false) }},
		{[]string{"status", "-s", "my-sess"}, func() (*CommandResult, error) { return cli.Status(context.Background(), "my-sess") }},
		{[]string{"status"}, func() (*CommandResult, error) { return cli.Status(context.Background(), "") }},
		{[]string{"sessions"}, func() (*CommandResult, error) { return cli.Sessions(context.Background()) }},
		{[]string{"stop", "-s", "my-sess"}, func() (*CommandResult, error) { return cli.Stop(context.Background(), "my-sess") }},
	} {
		if _, err := tc.call(); err != nil {
			t.Fatal(err)
		}
		want := append(append([]string{}, prefix...), tc.args...)
		if !reflect.DeepEqual(mr.lastArgs, want) || mr.lastOpts.Env["HOME"] != profile.HomeDir {
			t.Fatalf("unexpected invocation: args=%v opts=%+v", mr.lastArgs, mr.lastOpts)
		}
	}
	wantProxy := "'env' 'HOME=/tmp/profile home' '/opt/bin/colab' '--config' '/tmp/profile/config.json' '--client-oauth-config' '/tmp/profile/oauth.json' 'ssh' '--proxy-mode' '-s' 'my-sess' '--identity' '/tmp/worker key'"
	if got := cli.ProxyCommand("my-sess", "/tmp/worker key"); got != wantProxy {
		t.Fatalf("unexpected proxy command: %s", got)
	}
	if got := cli.ProxyCommand("user's-session", "/tmp/worker"); !strings.Contains(got, "'user'\"'\"'s-session'") {
		t.Fatalf("session argument was not quoted: %s", got)
	}
}

func TestProfiles(t *testing.T) {
	store := NewProfileStore(t.TempDir())
	for _, name := range []string{"", "default", "other"} {
		p, err := store.EnsureProfile(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Dir(p.ConfigFile)); err != nil {
			t.Fatal(err)
		}
		if name == "other" {
			if _, err := os.Stat(p.HomeDir); err != nil {
				t.Fatal(err)
			}
		} else if p.HomeDir != "" || p.OAuthFile != "" {
			t.Fatalf("default profile should use the user's Colab authentication: %+v", p)
		}
	}
}

func TestParseColabOutput(t *testing.T) {
	output := []byte("[colab] Pruned 1 stale local session(s).\n[cbox-test] endpoint-1 | Hardware: L4 | Shape: STANDARD | Variant: GPU\n[?] endpoint-2 | Hardware: CPU | Shape: STANDARD | Variant: DEFAULT\n")
	sessions, err := ParseSessions(output)
	if err != nil || len(sessions) != 2 || sessions[0].Session != "cbox-test" || sessions[0].GPU != "L4" || sessions[1].Session != "endpoint-2" || sessions[1].GPU != "" {
		t.Fatalf("unexpected sessions: %+v, %v", sessions, err)
	}
	sessions, err = ParseSessions([]byte("[colab] No active sessions found on server.\n"))
	if err != nil || len(sessions) != 0 {
		t.Fatalf("expected no sessions: %+v, %v", sessions, err)
	}
	for _, state := range []string{"IDLE", "BUSY (train.py)", "STOPPED"} {
		status, err := ParseStatus([]byte("[cbox-test] endpoint | Hardware: T4 | Shape: STANDARD | Variant: GPU | Status: " + state + "\n  Last Execution: train.py at yesterday\n"))
		if err != nil || status.Session != "cbox-test" || status.GPU != "T4" || status.Alive != (state != "STOPPED") {
			t.Fatalf("unexpected status: %+v, %v", status, err)
		}
	}
	for _, output := range []string{"[colab] Session 'cbox-test' not found.", "[colab] No active sessions.", "Status: STOPPED"} {
		status, err := ParseStatus([]byte(output))
		if err != nil || status.Alive {
			t.Fatalf("expected stopped status: %+v, %v", status, err)
		}
	}
}
