package volume

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseMountSpec(t *testing.T) {
	// ro
	m1, err := ParseMountSpec("/data/WWTP:/data:ro")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m1.Target != "/data" || m1.Mode != ModeReadOnly {
		t.Fatalf("expected /data and ro, got %s and %s", m1.Target, m1.Mode)
	}

	// output
	m2, err := ParseMountSpec("./runs/full:/output:output")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m2.Target != "/output" || m2.Mode != ModeOutput {
		t.Fatalf("expected /output and output, got %s and %s", m2.Target, m2.Mode)
	}

	// default mode should be ro
	m3, err := ParseMountSpec("/var/log:/log")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m3.Mode != ModeReadOnly {
		t.Fatalf("expected default mode ro, got %s", m3.Mode)
	}

	// invalid spec
	_, err = ParseMountSpec("invalid")
	if err == nil {
		t.Fatal("expected error for invalid spec")
	}
}

func TestComputeSourceHash(t *testing.T) {
	tempDir := t.TempDir()
	f1 := filepath.Join(tempDir, "file1.txt")
	_ = os.WriteFile(f1, []byte("hello world"), 0644)

	h1, err := ComputeSourceHash(tempDir)
	if err != nil {
		t.Fatalf("failed to compute hash: %v", err)
	}
	if h1 == "" {
		t.Fatal("expected non-empty hash")
	}

	// Same directory should give same hash
	h2, err := ComputeSourceHash(tempDir)
	if err != nil || h1 != h2 {
		t.Fatalf("expected same hash, got %s and %s", h1, h2)
	}
}
