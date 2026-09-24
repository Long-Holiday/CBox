package scheduler

import (
	"context"
	"testing"
	"time"

	"cbox/internal/container"
	"cbox/internal/provider"
	"cbox/internal/runtime"
	"cbox/internal/volume"
)

func TestScoreRuntime(t *testing.T) {
	cont := &container.Container{
		ImageID: "img-123",
		Resource: container.ResourceSpec{
			GPUPreference: []string{"L4", "T4"},
		},
		Mounts: []volume.Mount{
			{VolumeID: "vol-abc", Target: "/data"},
		},
	}

	// Busy runtime -> -1
	rtBusy := &runtime.Runtime{
		ActualGPU: "L4",
		State:     runtime.StateBusy,
	}
	if sc := ScoreRuntime(rtBusy, cont); sc != -1 {
		t.Fatalf("expected -1 for busy runtime, got %d", sc)
	}

	// Mismatched GPU -> -1
	rtMismatch := &runtime.Runtime{
		ActualGPU: "A100",
		State:     runtime.StateIdle,
	}
	if sc := ScoreRuntime(rtMismatch, cont); sc != -1 {
		t.Fatalf("expected -1 for mismatched GPU, got %d", sc)
	}

	// Idle matching GPU, no cache -> 100
	rtMatch := &runtime.Runtime{
		ActualGPU: "T4",
		State:     runtime.StateIdle,
	}
	if sc := ScoreRuntime(rtMatch, cont); sc != 100 {
		t.Fatalf("expected 100, got %d", sc)
	}

	// With Image cache hit -> 100 + 30 = 130
	rtImageMatch := &runtime.Runtime{
		ActualGPU: "L4",
		State:     runtime.StateIdle,
		Cache: runtime.RuntimeCache{
			Images: map[string]bool{"img-123": true},
		},
	}
	if sc := ScoreRuntime(rtImageMatch, cont); sc != 130 {
		t.Fatalf("expected 130, got %d", sc)
	}

	// With Image and Volume cache hit -> 100 + 30 + 50 = 180
	rtAllMatch := &runtime.Runtime{
		ActualGPU: "L4",
		State:     runtime.StateIdle,
		Cache: runtime.RuntimeCache{
			Images:  map[string]bool{"img-123": true},
			Volumes: map[string]string{"vol-abc": "hash123"},
		},
	}
	if sc := ScoreRuntime(rtAllMatch, cont); sc != 180 {
		t.Fatalf("expected 180, got %d", sc)
	}
}

func TestSchedulerAcquisition(t *testing.T) {
	mockProv := provider.NewMockProvider("mock")
	rtSvc := runtime.NewService(nil, 30*time.Minute, nil)
	rtSvc.RegisterProvider(mockProv)

	sched := NewScheduler(rtSvc, nil)

	cont := &container.Container{
		ID: "cont-1",
		Resource: container.ResourceSpec{
			GPUPreference: []string{"T4"},
		},
	}

	// 1. Initial acquisition should provision a new runtime
	rt1, err := sched.AcquireRuntime(context.Background(), cont)
	if err != nil {
		t.Fatalf("failed to acquire runtime: %v", err)
	}
	if rt1.ActualGPU != "T4" {
		t.Fatalf("expected T4, got %s", rt1.ActualGPU)
	}
	if rt1.State != runtime.StateBusy {
		t.Fatalf("expected state busy after acquisition, got %s", rt1.State)
	}

	// 2. While rt1 is busy, next container requires new runtime
	cont2 := &container.Container{
		ID: "cont-2",
		Resource: container.ResourceSpec{
			GPUPreference: []string{"T4"},
		},
	}
	rt2, err := sched.AcquireRuntime(context.Background(), cont2)
	if err != nil {
		t.Fatalf("failed to acquire runtime 2: %v", err)
	}
	if rt2.ID == rt1.ID {
		t.Fatal("expected distinct runtime while rt1 is busy")
	}

	// 3. Mark rt1 as idle, cont3 should reuse rt1
	rtSvc.Pool().MarkIdle(rt1.ID)
	cont3 := &container.Container{
		ID: "cont-3",
		Resource: container.ResourceSpec{
			GPUPreference: []string{"T4"},
		},
	}
	rt3, err := sched.AcquireRuntime(context.Background(), cont3)
	if err != nil {
		t.Fatalf("failed to acquire runtime 3: %v", err)
	}
	if rt3.ID != rt1.ID {
		t.Fatalf("expected rt3 to reuse idle rt1, got %s vs %s", rt3.ID, rt1.ID)
	}
}
