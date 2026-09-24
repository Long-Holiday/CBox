package container

import (
	"time"

	"cbox/internal/volume"
)

type ContainerState string

const (
	StateCreated      ContainerState = "created"
	StateProvisioning ContainerState = "provisioning"
	StatePreparing    ContainerState = "preparing"
	StateStarting     ContainerState = "starting"
	StateRunning      ContainerState = "running"
	StateExited       ContainerState = "exited"
	StateStopping     ContainerState = "stopping"
	StateStopped      ContainerState = "stopped"
	StateInterrupted  ContainerState = "interrupted"
	StateRecovering   ContainerState = "recovering"
	StateFailed       ContainerState = "failed"
)

type DesiredState string

const (
	DesiredRunning DesiredState = "running"
	DesiredStopped DesiredState = "stopped"
)

type RestartPolicy string

const (
	RestartNo            RestartPolicy = "no"
	RestartOnFailure     RestartPolicy = "on-failure"
	RestartUnlessStopped RestartPolicy = "unless-stopped"
	RestartAlways        RestartPolicy = "always"
)

type ResourceSpec struct {
	GPUPreference []string `json:"gpu_preference"`
	HighMemory    bool     `json:"high_memory"`
	Profile       string   `json:"profile,omitempty"`
}

type Mount = volume.Mount

type Container struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	ImageID       string            `json:"image_id"`
	ImageName     string            `json:"image_name,omitempty"`
	Command       []string          `json:"command"`
	Env           map[string]string `json:"env,omitempty"`
	WorkDir       string            `json:"workdir"`
	Resource      ResourceSpec      `json:"resource"`
	Mounts        []Mount           `json:"mounts"`
	State         ContainerState    `json:"state"`
	DesiredState  DesiredState      `json:"desired_state"`
	RuntimeID     string            `json:"runtime_id,omitempty"`
	RestartPolicy RestartPolicy     `json:"restart_policy"`
	ResumeCommand []string          `json:"resume_command,omitempty"`
	Secrets       []string          `json:"secrets,omitempty"`
	ExitCode      *int              `json:"exit_code,omitempty"`
	CreatedAt     time.Time         `json:"created_at"`
	StartedAt     *time.Time        `json:"started_at,omitempty"`
	FinishedAt    *time.Time        `json:"finished_at,omitempty"`
}
