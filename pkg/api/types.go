package api

import "time"

type ContainerCreateRequest struct {
	Name          string            `json:"name"`
	Image         string            `json:"image"`
	GPUPreference []string          `json:"gpu_preference"`
	HighMemory    bool              `json:"high_memory"`
	Command       []string          `json:"command"`
	Env           map[string]string `json:"env"`
	WorkDir       string            `json:"workdir"`
	Volumes       []string          `json:"volumes"`
	RestartPolicy string            `json:"restart_policy"`
	ResumeCommand []string          `json:"resume_command"`
	Secrets       []string          `json:"secrets"`
}

type ContainerStartRequest struct {
	Detach bool `json:"detach"`
}

type ContainerStopRequest struct {
	TimeoutSeconds int `json:"timeout_seconds"`
}

type MountInfo struct {
	VolumeID string `json:"volume_id"`
	Source   string `json:"source"`
	Target   string `json:"target"`
	Mode     string `json:"mode"`
}

type ContainerResponse struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	ImageID       string            `json:"image_id"`
	ImageName     string            `json:"image_name"`
	State         string            `json:"state"`
	DesiredState  string            `json:"desired_state"`
	Command       []string          `json:"command"`
	Env           map[string]string `json:"env,omitempty"`
	WorkDir       string            `json:"workdir"`
	GPU           string            `json:"gpu"`
	Mounts        []MountInfo       `json:"mounts"`
	RuntimeID     string            `json:"runtime_id,omitempty"`
	RestartPolicy string            `json:"restart_policy"`
	ResumeCommand []string          `json:"resume_command,omitempty"`
	ExitCode      *int              `json:"exit_code,omitempty"`
	CreatedAt     time.Time         `json:"created_at"`
	StartedAt     *time.Time        `json:"started_at,omitempty"`
	FinishedAt    *time.Time        `json:"finished_at,omitempty"`
}

type ContainerSummary struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Image     string    `json:"image"`
	GPU       string    `json:"gpu"`
	State     string    `json:"state"`
	CreatedAt time.Time `json:"created_at"`
}

type ImageBuildRequest struct {
	ContextDir string `json:"context_dir"`
	Cboxfile   string `json:"cboxfile"`
	Tag        string `json:"tag"`
}

type ImageResponse struct {
	ID        string    `json:"id"`
	Tags      []string  `json:"tags"`
	CreatedAt time.Time `json:"created_at"`
	Base      string    `json:"base"`
	Apt       []string  `json:"apt,omitempty"`
	Pip       []string  `json:"pip,omitempty"`
}

type VolumeCreateRequest struct {
	Name   string `json:"name"`
	Source string `json:"source"`
	Mode   string `json:"mode"`
}

type VolumeResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Source    string    `json:"source"`
	Mode      string    `json:"mode"`
	CreatedAt time.Time `json:"created_at"`
}

type ContextCreateRequest struct {
	Name         string `json:"name"`
	Provider     string `json:"provider"`
	Profile      string `json:"profile"`
	AutoSchedule bool   `json:"auto_schedule"`
}

type ContextResponse struct {
	Name         string `json:"name"`
	Provider     string `json:"provider"`
	Profile      string `json:"profile"`
	AutoSchedule bool   `json:"auto_schedule"`
	IsCurrent    bool   `json:"is_current"`
}

type StatsResponse struct {
	ContainerID      string    `json:"container_id"`
	GPUName          string    `json:"gpu_name"`
	GPUMemoryUsedMB  int       `json:"gpu_memory_used_mb"`
	GPUMemoryTotalMB int       `json:"gpu_memory_total_mb"`
	GPUUtilization   float64   `json:"gpu_utilization"`
	MemoryUsedMB     int64     `json:"memory_used_mb"`
	MemoryTotalMB    int64     `json:"memory_total_mb"`
	Timestamp        time.Time `json:"timestamp"`
}

type LogsResponse struct {
	ContainerID string `json:"container_id"`
	Stdout      string `json:"stdout"`
	Stderr      string `json:"stderr"`
}

type ExecRequest struct {
	Command     []string `json:"command"`
	Interactive bool     `json:"interactive"`
	Tty         bool     `json:"tty"`
}

type ExecResponse struct {
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
}

type TransportInfoResponse struct {
	Host       string `json:"host"`
	ConfigFile string `json:"config_file"`
	User       string `json:"user"`
	RuntimeID  string `json:"runtime_id"`
}

type EventResponse struct {
	ID         string    `json:"id"`
	ObjectType string    `json:"object_type"`
	ObjectID   string    `json:"object_id"`
	Type       string    `json:"type"`
	Timestamp  time.Time `json:"timestamp"`
	Payload    string    `json:"payload"`
}

type VersionResponse struct {
	Version   string `json:"version"`
	GoVersion string `json:"go_version"`
	OS        string `json:"os"`
	Arch      string `json:"arch"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}
