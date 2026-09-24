package compose

type VolumeSpec struct {
	Source string `yaml:"source"`
	Target string `yaml:"target"`
	Mode   string `yaml:"mode"`
}

type GPUSpec struct {
	Preference []string `yaml:"preference"`
}

type ServiceSpec struct {
	Image   string            `yaml:"image"`
	GPU     any               `yaml:"gpu"` // can be []string, string, or GPUSpec
	Volumes []any             `yaml:"volumes"`
	Command []string          `yaml:"command"`
	Env     map[string]string `yaml:"env"`
	Restart string            `yaml:"restart"`
	WorkDir string            `yaml:"workdir"`
}

type ComposeConfig struct {
	Version  string                 `yaml:"version"`
	Services map[string]ServiceSpec `yaml:"services"`
}
