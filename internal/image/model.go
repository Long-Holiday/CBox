package image

import "time"

type InstructionType string

const (
	InstructionFrom    InstructionType = "FROM"
	InstructionApt     InstructionType = "APT"
	InstructionPip     InstructionType = "PIP"
	InstructionEnv     InstructionType = "ENV"
	InstructionWorkdir InstructionType = "WORKDIR"
	InstructionCopy    InstructionType = "COPY"
	InstructionRun     InstructionType = "RUN"
	InstructionCmd     InstructionType = "CMD"
)

type Instruction interface {
	Type() InstructionType
}

type CopySpec struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

type ImageManifest struct {
	Base    string            `json:"base"`
	Apt     []string          `json:"apt,omitempty"`
	Pip     []string          `json:"pip,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	WorkDir string            `json:"workdir,omitempty"`
	Copies  []CopySpec        `json:"copies,omitempty"`
	Runs    []string          `json:"runs,omitempty"`
	Command []string          `json:"command,omitempty"`
}

type Image struct {
	ID        string        `json:"id"`
	Tags      []string      `json:"tags"`
	Manifest  ImageManifest `json:"manifest"`
	CreatedAt time.Time     `json:"created_at"`
}
