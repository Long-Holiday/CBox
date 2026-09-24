package colab

import (
	"context"
	"errors"
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
}

type mockRunner struct {
	lastCmd  string
	lastArgs []string
}

func (m *mockRunner) Run(ctx context.Context, name string, args []string, opts CommandOptions) (*CommandResult, error) {
	m.lastCmd = name
	m.lastArgs = args
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

	if mr.lastCmd != "colab" || mr.lastArgs[0] != "new" || mr.lastArgs[1] != "-s" || mr.lastArgs[2] != "my-sess" {
		t.Fatalf("unexpected command call: %s %v", mr.lastCmd, mr.lastArgs)
	}
}
