package errors

import (
	"errors"
	"fmt"
)

var (
	// Sentinel errors
	ErrNotFound       = errors.New("not found")
	ErrAlreadyExists  = errors.New("already exists")
	ErrInvalidState   = errors.New("invalid state")
	ErrConflict       = errors.New("conflict")
	ErrUnauthorized   = errors.New("unauthorized")
	ErrTimeout        = errors.New("timed out")
	ErrNotImplemented = errors.New("not implemented")
)

type AllocationError struct {
	GPU    string
	Reason string
	Err    error
}

func (e *AllocationError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("gpu allocation failed for %s (%s): %v", e.GPU, e.Reason, e.Err)
	}
	return fmt.Sprintf("gpu allocation failed for %s: %s", e.GPU, e.Reason)
}

func (e *AllocationError) Unwrap() error {
	return e.Err
}

type ProviderError struct {
	Provider string
	Op       string
	Err      error
}

func (e *ProviderError) Error() string {
	return fmt.Sprintf("provider %s failed during %s: %v", e.Provider, e.Op, e.Err)
}

func (e *ProviderError) Unwrap() error {
	return e.Err
}

type AuthenticationError struct {
	Provider string
	Profile  string
	Message  string
	Err      error
}

func (e *AuthenticationError) Error() string {
	return fmt.Sprintf("authentication failed for provider %s (profile %s): %s", e.Provider, e.Profile, e.Message)
}

func (e *AuthenticationError) Unwrap() error {
	return e.Err
}

type TransportError struct {
	Host    string
	Op      string
	Message string
	Err     error
}

func (e *TransportError) Error() string {
	return fmt.Sprintf("transport error on %s during %s: %s: %v", e.Host, e.Op, e.Message, e.Err)
}

func (e *TransportError) Unwrap() error {
	return e.Err
}

type ContainerError struct {
	ContainerID string
	Op          string
	Err         error
}

func (e *ContainerError) Error() string {
	return fmt.Sprintf("container %s %s error: %v", e.ContainerID, e.Op, e.Err)
}

func (e *ContainerError) Unwrap() error {
	return e.Err
}

type ImageError struct {
	Image string
	Op    string
	Err   error
}

func (e *ImageError) Error() string {
	return fmt.Sprintf("image %s %s error: %v", e.Image, e.Op, e.Err)
}

func (e *ImageError) Unwrap() error {
	return e.Err
}

type VolumeError struct {
	Volume string
	Op     string
	Err    error
}

func (e *VolumeError) Error() string {
	return fmt.Sprintf("volume %s %s error: %v", e.Volume, e.Op, e.Err)
}

func (e *VolumeError) Unwrap() error {
	return e.Err
}

type RecoveryError struct {
	ContainerID string
	Attempt     int
	Err         error
}

func (e *RecoveryError) Error() string {
	return fmt.Sprintf("recovery failed for container %s (attempt %d): %v", e.ContainerID, e.Attempt, e.Err)
}

func (e *RecoveryError) Unwrap() error {
	return e.Err
}

type CommandError struct {
	Command  string
	ExitCode int
	Stdout   string
	Stderr   string
}

func (e *CommandError) Error() string {
	return fmt.Sprintf("command %q failed with exit code %d: stderr=%s", e.Command, e.ExitCode, e.Stderr)
}
