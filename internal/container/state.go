package container

import (
	"fmt"
	"time"

	cboxErr "cbox/internal/errors"
)

var validTransitions = map[ContainerState][]ContainerState{
	StateCreated: {
		StateProvisioning,
		StateStopped,
		StateFailed,
	},
	StateProvisioning: {
		StatePreparing,
		StateInterrupted,
		StateFailed,
		StateStopped,
	},
	StatePreparing: {
		StateStarting,
		StateInterrupted,
		StateFailed,
		StateStopped,
	},
	StateStarting: {
		StateRunning,
		StateInterrupted,
		StateFailed,
		StateStopped,
	},
	StateRunning: {
		StateExited,
		StateStopping,
		StateInterrupted,
		StateFailed,
	},
	StateStopping: {
		StateStopped,
		StateFailed,
	},
	StateStopped: {
		StateProvisioning,
		StateCreated,
	},
	StateInterrupted: {
		StateRecovering,
		StateStopped,
		StateFailed,
	},
	StateRecovering: {
		StateProvisioning,
		StateStopped,
		StateFailed,
	},
	StateExited: {
		StateProvisioning,
		StateCreated,
	},
	StateFailed: {
		StateProvisioning,
		StateCreated,
	},
}

func CanTransition(from, to ContainerState) bool {
	allowed, ok := validTransitions[from]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
}

func Transition(c *Container, to ContainerState) error {
	if c.State == to {
		return nil
	}

	if !CanTransition(c.State, to) {
		return fmt.Errorf("%w: invalid container state transition from %s to %s", cboxErr.ErrInvalidState, c.State, to)
	}

	now := time.Now().UTC()
	c.State = to

	if to == StateRunning && c.StartedAt == nil {
		c.StartedAt = &now
	}

	if to == StateExited || to == StateStopped || to == StateFailed {
		c.FinishedAt = &now
	}

	return nil
}
