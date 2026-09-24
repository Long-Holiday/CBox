package volume

import "time"

type VolumeMode string

const (
	ModeReadOnly  VolumeMode = "ro"
	ModeReadWrite VolumeMode = "rw"
	ModeOutput    VolumeMode = "output"
	ModeCache     VolumeMode = "cache"
)

type Volume struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	Source       string     `json:"source"`
	Mode         VolumeMode `json:"mode"`
	Immutable    bool       `json:"immutable"`
	ManifestHash string     `json:"manifest_hash,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

type Mount struct {
	VolumeID string     `json:"volume_id"`
	Source   string     `json:"source"`
	Target   string     `json:"target"`
	Mode     VolumeMode `json:"mode"`
}
