package scheduler

import (
	"cbox/internal/container"
	"cbox/internal/runtime"
)

func ScoreRuntime(rt *runtime.Runtime, c *container.Container) int {
	// If runtime is not idle, it cannot be used
	if rt.State != runtime.StateIdle && rt.State != runtime.StateReady {
		return -1
	}

	// Check GPU preference matching
	matchedGPU := false
	if len(c.Resource.GPUPreference) == 0 {
		matchedGPU = true
	} else {
		for _, pref := range c.Resource.GPUPreference {
			if pref == rt.ActualGPU || pref == rt.RequestedGPU {
				matchedGPU = true
				break
			}
		}
	}

	if !matchedGPU {
		return -1
	}

	score := 100

	// Bonus for matching image cache
	if rt.Cache.Images != nil && rt.Cache.Images[c.ImageID] {
		score += 30
	}

	// Bonus for matching volume caches
	if rt.Cache.Volumes != nil {
		for _, m := range c.Mounts {
			volKey := m.VolumeID
			if volKey == "" {
				volKey = m.Source
			}
			if rt.Cache.Volumes[volKey] != "" {
				score += 50
			}
		}
	}

	return score
}
