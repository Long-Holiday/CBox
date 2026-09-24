package config

import (
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Engine   EngineConfig   `yaml:"engine"`
	Runtime  RuntimeConfig  `yaml:"runtime"`
	Sync     SyncConfig     `yaml:"sync"`
	SSH      SSHConfig      `yaml:"ssh"`
	Rsync    RsyncConfig    `yaml:"rsync"`
	Provider ProviderConfig `yaml:"provider"`
	Recovery RecoveryConfig `yaml:"recovery"`
	Logging  LoggingConfig  `yaml:"logging"`
}

type EngineConfig struct {
	DataDir    string `yaml:"data_dir"`
	SocketPath string `yaml:"socket_path"`
}

type RuntimeConfig struct {
	IdleTimeoutStr string        `yaml:"idle_timeout"`
	IdleTimeout    time.Duration `yaml:"-"`
}

type SyncConfig struct {
	IntervalStr string        `yaml:"interval"`
	Interval    time.Duration `yaml:"-"`
	Transfer    string        `yaml:"transfer"`
}

type SSHConfig struct {
	Binary            string        `yaml:"binary"`
	ControlPersistStr string        `yaml:"control_persist"`
	ControlPersist    time.Duration `yaml:"-"`
}

type RsyncConfig struct {
	Binary string `yaml:"binary"`
}

type ProviderConfig struct {
	ColabBinary string `yaml:"colab_binary"`
}

type RecoveryConfig struct {
	Enabled     bool `yaml:"enabled"`
	MaxAttempts int  `yaml:"max_attempts"`
}

type LoggingConfig struct {
	Level string `yaml:"level"`
}

func DefaultConfig() *Config {
	return &Config{
		Engine: EngineConfig{
			DataDir: "",
		},
		Runtime: RuntimeConfig{
			IdleTimeoutStr: "30m",
			IdleTimeout:    30 * time.Minute,
		},
		Sync: SyncConfig{
			IntervalStr: "10m",
			Interval:    10 * time.Minute,
			Transfer:    "auto",
		},
		SSH: SSHConfig{
			Binary:            "ssh",
			ControlPersistStr: "10m",
			ControlPersist:    10 * time.Minute,
		},
		Rsync: RsyncConfig{
			Binary: "rsync",
		},
		Provider: ProviderConfig{
			ColabBinary: "colab",
		},
		Recovery: RecoveryConfig{
			Enabled:     true,
			MaxAttempts: 3,
		},
		Logging: LoggingConfig{
			Level: "info",
		},
	}
}

func LoadConfig(configPath string) (*Config, error) {
	cfg := DefaultConfig()

	if configPath == "" {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			configPath = filepath.Join(homeDir, ".config", "cbox", "config.yaml")
		}
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			_ = cfg.ParseDurations()
			return cfg, nil
		}
		return nil, err
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	if err := cfg.ParseDurations(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) ParseDurations() error {
	if c.Runtime.IdleTimeoutStr != "" {
		d, err := time.ParseDuration(c.Runtime.IdleTimeoutStr)
		if err == nil {
			c.Runtime.IdleTimeout = d
		}
	} else if c.Runtime.IdleTimeout == 0 {
		c.Runtime.IdleTimeout = 30 * time.Minute
	}

	if c.Sync.IntervalStr != "" {
		d, err := time.ParseDuration(c.Sync.IntervalStr)
		if err == nil {
			c.Sync.Interval = d
		}
	} else if c.Sync.Interval == 0 {
		c.Sync.Interval = 10 * time.Minute
	}

	if c.SSH.ControlPersistStr != "" {
		d, err := time.ParseDuration(c.SSH.ControlPersistStr)
		if err == nil {
			c.SSH.ControlPersist = d
		}
	} else if c.SSH.ControlPersist == 0 {
		c.SSH.ControlPersist = 10 * time.Minute
	}

	return nil
}
