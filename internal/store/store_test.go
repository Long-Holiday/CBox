package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"cbox/internal/container"
	cboxContext "cbox/internal/context"
	"cbox/internal/image"
	"cbox/internal/runtime"
	"cbox/internal/volume"
)

func setupTestDB(t *testing.T) *DB {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	db, err := OpenDB(dbPath)
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestContainerRepository(t *testing.T) {
	db := setupTestDB(t)
	repo := NewContainerRepository(db)
	ctx := context.Background()

	c := &container.Container{
		ID:        "c-001",
		Name:      "test-container",
		ImageID:   "img-123",
		State:     container.StateCreated,
		Command:   []string{"python", "train.py"},
		Env:       map[string]string{"LR": "0.01"},
		WorkDir:   "/workspace",
		CreatedAt: time.Now().UTC(),
		Mounts: []volume.Mount{
			{VolumeID: "v-1", Source: "/src", Target: "/dst", Mode: volume.ModeReadOnly},
		},
	}

	// Create
	if err := repo.Create(ctx, c); err != nil {
		t.Fatalf("failed to create container: %v", err)
	}

	// Get by ID
	fetched, err := repo.Get(ctx, "c-001")
	if err != nil {
		t.Fatalf("failed to get container: %v", err)
	}
	if fetched.Name != "test-container" || len(fetched.Mounts) != 1 {
		t.Fatalf("unexpected fetched container: %+v", fetched)
	}

	// Get by Name
	fetchedByName, err := repo.Get(ctx, "test-container")
	if err != nil || fetchedByName.ID != "c-001" {
		t.Fatalf("failed to get by name: %v", err)
	}

	// Update state
	if err := repo.UpdateState(ctx, "c-001", container.StateRunning); err != nil {
		t.Fatalf("failed to update state: %v", err)
	}

	fetched2, _ := repo.Get(ctx, "c-001")
	if fetched2.State != container.StateRunning || fetched2.StartedAt == nil {
		t.Fatalf("expected running with started_at, got %s", fetched2.State)
	}

	// List
	list, err := repo.List(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("unexpected list: %v", list)
	}

	// Delete
	if err := repo.Delete(ctx, "c-001"); err != nil {
		t.Fatalf("failed to delete container: %v", err)
	}
}

func TestImageRepository(t *testing.T) {
	db := setupTestDB(t)
	repo := NewImageRepository(db)
	ctx := context.Background()

	img := &image.Image{
		ID:   "img-abc",
		Tags: []string{"mmseg:latest", "mmseg:v1"},
		Manifest: image.ImageManifest{
			Base: "colab/python",
			Apt:  []string{"git"},
		},
		CreatedAt: time.Now().UTC(),
	}

	if err := repo.Create(ctx, img); err != nil {
		t.Fatalf("failed to create image: %v", err)
	}

	// Get by Tag
	fetched, err := repo.Get(ctx, "mmseg:latest")
	if err != nil {
		t.Fatalf("failed to get image by tag: %v", err)
	}
	if fetched.ID != "img-abc" || len(fetched.Tags) != 2 {
		t.Fatalf("unexpected fetched image: %+v", fetched)
	}
}

func TestVolumeRepository(t *testing.T) {
	db := setupTestDB(t)
	repo := NewVolumeRepository(db)
	ctx := context.Background()

	v := &volume.Volume{
		ID:        "vol-1",
		Name:      "my-dataset",
		Source:    "/data/test",
		Mode:      volume.ModeReadOnly,
		Immutable: true,
		CreatedAt: time.Now().UTC(),
	}

	if err := repo.Create(ctx, v); err != nil {
		t.Fatalf("failed to create volume: %v", err)
	}

	fetched, err := repo.Get(ctx, "my-dataset")
	if err != nil || fetched.Source != "/data/test" {
		t.Fatalf("failed to get volume: %v", err)
	}
}

func TestContextRepository(t *testing.T) {
	db := setupTestDB(t)
	repo := NewContextRepository(db)
	ctx := context.Background()

	c1 := &cboxContext.Context{
		Name:      "colab-a",
		Provider:  "colab",
		Profile:   "profile-a",
		IsCurrent: true,
	}
	c2 := &cboxContext.Context{
		Name:      "colab-b",
		Provider:  "colab",
		Profile:   "profile-b",
		IsCurrent: false,
	}

	_ = repo.Create(ctx, c1)
	_ = repo.Create(ctx, c2)

	curr, err := repo.GetCurrent(ctx)
	if err != nil || curr.Name != "colab-a" {
		t.Fatalf("expected colab-a current, got %v", curr)
	}

	_ = repo.SetCurrent(ctx, "colab-b")
	curr2, _ := repo.GetCurrent(ctx)
	if curr2.Name != "colab-b" {
		t.Fatalf("expected colab-b current after switch, got %v", curr2)
	}
}

func TestRuntimeRepository(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRuntimeRepository(db)
	ctx := context.Background()

	rt := &runtime.Runtime{
		ID:           "rt-001",
		Provider:     "colab",
		Session:      "sess-1",
		Profile:      "default",
		RequestedGPU: "L4",
		ActualGPU:    "L4",
		State:        runtime.StateReady,
		CreatedAt:    time.Now().UTC(),
		LastSeen:     time.Now().UTC(),
		Cache: runtime.RuntimeCache{
			Images:  map[string]bool{"img-1": true},
			Volumes: map[string]string{"vol-1": "hash"},
		},
	}

	if err := repo.Create(ctx, rt); err != nil {
		t.Fatalf("failed to create runtime: %v", err)
	}

	fetched, err := repo.Get(ctx, "rt-001")
	if err != nil || fetched.ActualGPU != "L4" {
		t.Fatalf("failed to get runtime: %v", err)
	}

	if err := repo.UpdateState(ctx, "rt-001", runtime.StateBusy); err != nil {
		t.Fatalf("failed to update state: %v", err)
	}

	fetched2, _ := repo.Get(ctx, "rt-001")
	if fetched2.State != runtime.StateBusy {
		t.Fatalf("expected state busy, got %s", fetched2.State)
	}
}

func TestEventRepository(t *testing.T) {
	db := setupTestDB(t)
	repo := NewEventRepository(db)
	ctx := context.Background()

	err := repo.Record(ctx, "container", "c-100", "container.started", map[string]string{"gpu": "L4"})
	if err != nil {
		t.Fatalf("failed to record event: %v", err)
	}

	events, err := repo.ListByObject(ctx, "container", "c-100")
	if err != nil || len(events) != 1 {
		t.Fatalf("expected 1 event, got %v", events)
	}
	if events[0].Type != "container.started" {
		t.Fatalf("expected container.started, got %s", events[0].Type)
	}
}
