package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Runtime.IdleTimeout != 30*time.Minute {
		t.Fatalf("expected 30m idle timeout, got %v", cfg.Runtime.IdleTimeout)
	}
	if cfg.Sync.Interval != 10*time.Minute {
		t.Fatalf("expected 10m sync interval, got %v", cfg.Sync.Interval)
	}
}

func TestLoadConfigCustomYAML(t *testing.T) {
	tempDir := t.TempDir()
	cfgFile := filepath.Join(tempDir, "config.yaml")

	yamlContent := `
engine:
  data_dir: /custom/cbox
runtime:
  idle_timeout: 45m
sync:
  interval: 15m
provider:
  colab_binary: /usr/local/bin/colab
recovery:
  enabled: true
  max_attempts: 5
`
	if err := os.WriteFile(cfgFile, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(cfgFile)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Engine.DataDir != "/custom/cbox" {
		t.Fatalf("expected /custom/cbox, got %s", cfg.Engine.DataDir)
	}
	if cfg.Runtime.IdleTimeout != 45*time.Minute {
		t.Fatalf("expected 45m, got %v", cfg.Runtime.IdleTimeout)
	}
	if cfg.Sync.Interval != 15*time.Minute {
		t.Fatalf("expected 15m, got %v", cfg.Sync.Interval)
	}
	if cfg.Recovery.MaxAttempts != 5 {
		t.Fatalf("expected 5 max attempts, got %d", cfg.Recovery.MaxAttempts)
	}
}

func TestResolvePaths(t *testing.T) {
	paths, err := ResolvePaths("/tmp/custom-cbox")
	if err != nil {
		t.Fatalf("failed to resolve paths: %v", err)
	}

	if paths.DataDir != "/tmp/custom-cbox" {
		t.Fatalf("expected /tmp/custom-cbox, got %s", paths.DataDir)
	}
	if paths.DBPath != "/tmp/custom-cbox/cbox.db" {
		t.Fatalf("expected DBPath in data dir, got %s", paths.DBPath)
	}
}
