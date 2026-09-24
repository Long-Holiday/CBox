package compose

import (
	"path/filepath"
	"testing"
)

func TestParseComposeYaml(t *testing.T) {
	composePath := filepath.Join("..", "..", "examples", "cbox-compose.yaml")
	cfg, err := ParseComposeFile(composePath)
	if err != nil {
		t.Fatalf("failed to parse compose file: %v", err)
	}

	if cfg.Version != "1" {
		t.Fatalf("expected version 1, got %s", cfg.Version)
	}

	if len(cfg.Services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(cfg.Services))
	}

	fullSvc, ok := cfg.Services["full"]
	if !ok {
		t.Fatal("missing 'full' service")
	}

	if fullSvc.Image != "wwtp:mmseg" {
		t.Fatalf("expected image wwtp:mmseg, got %s", fullSvc.Image)
	}

	gpus := ExtractGPUPreferences(fullSvc.GPU)
	if len(gpus) != 2 || gpus[0] != "L4" || gpus[1] != "T4" {
		t.Fatalf("unexpected GPU preferences: %v", gpus)
	}

	vols := ExtractVolumeSpecs(fullSvc.Volumes)
	if len(vols) != 2 {
		t.Fatalf("expected 2 volumes, got %v", vols)
	}
}

func TestParseComposeLocalYaml(t *testing.T) {
	composePath := filepath.Join("..", "..", "examples", "cbox-compose.local.yaml")
	cfg, err := ParseComposeFile(composePath)
	if err != nil {
		t.Fatalf("failed to parse local compose file: %v", err)
	}

	if len(cfg.Services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(cfg.Services))
	}
}
