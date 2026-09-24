package image

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestImageBuildAndHash(t *testing.T) {
	tempDir := t.TempDir()
	cboxFile := filepath.Join(tempDir, "Cboxfile")
	cboxContent := `
FROM colab/python:3
APT git
PIP torch
CMD ["bash"]
`
	if err := os.WriteFile(cboxFile, []byte(cboxContent), 0644); err != nil {
		t.Fatal(err)
	}

	svc := NewService(nil, nil)
	img1, err := svc.Build(context.Background(), tempDir, cboxFile, "test:v1")
	if err != nil {
		t.Fatalf("failed to build image: %v", err)
	}

	if img1.ID == "" {
		t.Fatal("expected non-empty image ID")
	}

	// Rebuild same should produce same ID
	img2, err := svc.Build(context.Background(), tempDir, cboxFile, "test:v2")
	if err != nil {
		t.Fatalf("failed to build image 2: %v", err)
	}

	if img1.ID != img2.ID {
		t.Fatalf("expected identical IDs for same Cboxfile, got %s and %s", img1.ID, img2.ID)
	}

	// Modify content should produce different ID
	cboxFile2 := filepath.Join(tempDir, "Cboxfile2")
	_ = os.WriteFile(cboxFile2, []byte(cboxContent+"\nAPT rsync\n"), 0644)

	img3, err := svc.Build(context.Background(), tempDir, cboxFile2, "test:v3")
	if err != nil {
		t.Fatalf("failed to build image 3: %v", err)
	}

	if img1.ID == img3.ID {
		t.Fatal("expected different IDs for modified Cboxfile")
	}
}
