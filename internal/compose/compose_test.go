package compose

import (
	"os"
	"path/filepath"
	"strings"
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

func TestGoogleDriveComposeVolumes(t *testing.T) {
	specs, err := ParseVolumeSpecs([]any{
		map[string]any{"type": "google-drive", "source": "MyDrive/datasets", "target": "/data"},
		map[string]any{"type": "google-drive", "source": "MyDrive/runs", "target": "/output", "mode": "rw"},
		"./local:/workspace:ro",
	})
	if err != nil || len(specs) != 3 || specs[0] != "gdrive://MyDrive/datasets:/data:ro" || specs[1] != "gdrive://MyDrive/runs:/output:rw" {
		t.Fatalf("unexpected volumes: %v, %v", specs, err)
	}
	for _, entry := range []any{
		map[string]any{"type": "s3", "source": "bucket", "target": "/data"},
		map[string]any{"type": "google-drive", "target": "/data"},
		map[string]any{"type": "google-drive", "source": "MyDrive/data", "target": "/data", "mode": "output"},
		42,
	} {
		if _, err := ParseVolumeSpecs([]any{entry}); err == nil {
			t.Errorf("accepted invalid volume %v", entry)
		}
	}
}

func TestComposeRejectsInvalidCloudMount(t *testing.T) {
	file := filepath.Join(t.TempDir(), "cbox-compose.yaml")
	if err := os.WriteFile(file, []byte("services:\n  train:\n    volumes:\n      - type: google-drive\n        source: MyDrive/data\n        target: /content/drive\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseComposeFile(file); err == nil || !strings.Contains(err.Error(), "service train") {
		t.Fatalf("expected service validation error, got %v", err)
	}
}
