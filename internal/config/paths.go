package config

import (
	"os"
	"path/filepath"
)

type Paths struct {
	DataDir     string
	ConfigDir   string
	RuntimeDir  string
	DBPath      string
	SocketPath  string
	KeysDir     string
	ProfilesDir string
	SSHDir      string
	LogsDir     string
	ImagesDir   string
	VolumesDir  string
}

func ResolvePaths(customDataDir string) (*Paths, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "/tmp"
	}

	dataDir := customDataDir
	if dataDir == "" {
		if xdgData := os.Getenv("XDG_DATA_HOME"); xdgData != "" {
			dataDir = filepath.Join(xdgData, "cbox")
		} else {
			dataDir = filepath.Join(homeDir, ".local", "share", "cbox")
		}
	} else if len(dataDir) >= 2 && dataDir[:2] == "~/" {
		dataDir = filepath.Join(homeDir, dataDir[2:])
	}

	var configDir string
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		configDir = filepath.Join(xdgConfig, "cbox")
	} else {
		configDir = filepath.Join(homeDir, ".config", "cbox")
	}

	var runtimeDir string
	if xdgRuntime := os.Getenv("XDG_RUNTIME_DIR"); xdgRuntime != "" {
		runtimeDir = filepath.Join(xdgRuntime, "cbox")
	} else {
		runtimeDir = filepath.Join(dataDir, "run")
	}

	p := &Paths{
		DataDir:     dataDir,
		ConfigDir:   configDir,
		RuntimeDir:  runtimeDir,
		DBPath:      filepath.Join(dataDir, "cbox.db"),
		SocketPath:  filepath.Join(runtimeDir, "cbox.sock"),
		KeysDir:     filepath.Join(dataDir, "keys"),
		ProfilesDir: filepath.Join(dataDir, "profiles"),
		SSHDir:      filepath.Join(dataDir, "ssh"),
		LogsDir:     filepath.Join(dataDir, "logs"),
		ImagesDir:   filepath.Join(dataDir, "images"),
		VolumesDir:  filepath.Join(dataDir, "volumes"),
	}

	return p, nil
}

func (p *Paths) EnsureDirs() error {
	dirs := []string{
		p.DataDir,
		p.ConfigDir,
		p.RuntimeDir,
		p.KeysDir,
		p.ProfilesDir,
		p.SSHDir,
		p.LogsDir,
		p.ImagesDir,
		p.VolumesDir,
	}

	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return err
		}
	}

	// Ensure keys directory has strict permissions
	_ = os.Chmod(p.KeysDir, 0700)
	_ = os.Chmod(p.SSHDir, 0700)

	return nil
}
