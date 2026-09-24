package container

import (
	"errors"
	"testing"

	cboxErr "cbox/internal/errors"
)

func TestStateTransitions(t *testing.T) {
	c := &Container{
		ID:    "test-1",
		State: StateCreated,
	}

	// Created -> Provisioning: ok
	if err := Transition(c, StateProvisioning); err != nil {
		t.Fatalf("expected transition to succeed: %v", err)
	}
	if c.State != StateProvisioning {
		t.Fatalf("expected state provisioning, got %s", c.State)
	}

	// Provisioning -> Preparing: ok
	if err := Transition(c, StatePreparing); err != nil {
		t.Fatalf("expected transition to succeed: %v", err)
	}

	// Preparing -> Starting: ok
	if err := Transition(c, StateStarting); err != nil {
		t.Fatalf("expected transition to succeed: %v", err)
	}

	// Starting -> Running: ok
	if err := Transition(c, StateRunning); err != nil {
		t.Fatalf("expected transition to succeed: %v", err)
	}
	if c.StartedAt == nil {
		t.Fatal("expected StartedAt to be set")
	}

	// Running -> Exited: ok
	if err := Transition(c, StateExited); err != nil {
		t.Fatalf("expected transition to succeed: %v", err)
	}
	if c.FinishedAt == nil {
		t.Fatal("expected FinishedAt to be set")
	}

	// Exited -> Running: invalid (must go through provisioning/created)
	if err := Transition(c, StateRunning); !errors.Is(err, cboxErr.ErrInvalidState) {
		t.Fatalf("expected ErrInvalidState, got %v", err)
	}
}

func TestRecoveryStateTransitions(t *testing.T) {
	c := &Container{
		ID:    "test-rec",
		State: StateRunning,
	}

	// Running -> Interrupted: ok
	if err := Transition(c, StateInterrupted); err != nil {
		t.Fatalf("expected transition to interrupted: %v", err)
	}

	// Interrupted -> Recovering: ok
	if err := Transition(c, StateRecovering); err != nil {
		t.Fatalf("expected transition to recovering: %v", err)
	}

	// Recovering -> Provisioning: ok
	if err := Transition(c, StateProvisioning); err != nil {
		t.Fatalf("expected transition to provisioning: %v", err)
	}
}
