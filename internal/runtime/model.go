package runtime

import "time"

type RuntimeState string

const (
	StateProvisioning RuntimeState = "provisioning"
	StateConnecting   RuntimeState = "connecting"
	StateReady        RuntimeState = "ready"
	StateBusy         RuntimeState = "busy"
	StateIdle         RuntimeState = "idle"
	StateStopping     RuntimeState = "stopping"
	StateStopped      RuntimeState = "stopped"
	StateLost         RuntimeState = "lost"
)

type RuntimeCache struct {
	Images  map[string]bool   `json:"images"`
	Volumes map[string]string `json:"volumes"`
}

type Runtime struct {
	ID           string       `json:"id"`
	Provider     string       `json:"provider"`
	Session      string       `json:"session"`
	Profile      string       `json:"profile"`
	RequestedGPU string       `json:"requested_gpu"`
	ActualGPU    string       `json:"actual_gpu"`
	State        RuntimeState `json:"state"`
	CreatedAt    time.Time    `json:"created_at"`
	LastSeen     time.Time    `json:"last_seen"`
	Cache        RuntimeCache `json:"cache"`
}

type RuntimeRequest struct {
	Session    string `json:"session"`
	GPU        string `json:"gpu"`
	HighMemory bool   `json:"high_memory"`
	Profile    string `json:"profile"`
}

type RuntimeHandle struct {
	ID            string `json:"id"`
	Session       string `json:"session"`
	Host          string `json:"host"`
	SSHConfigFile string `json:"ssh_config_file"`
	User          string `json:"user"`
}

type RuntimeStatus struct {
	ID       string       `json:"id"`
	State    RuntimeState `json:"state"`
	GPU      string       `json:"gpu"`
	Alive    bool         `json:"alive"`
	LastSeen time.Time    `json:"last_seen"`
}
