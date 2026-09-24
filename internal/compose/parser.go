package compose

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

func ParseComposeFile(path string) (*ComposeConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read compose file %s: %w", path, err)
	}

	var cfg ComposeConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal compose file: %w", err)
	}

	return &cfg, nil
}

func ExtractGPUPreferences(gpuField any) []string {
	if gpuField == nil {
		return nil
	}

	switch v := gpuField.(type) {
	case string:
		return []string{v}
	case []any:
		var list []string
		for _, item := range v {
			if s, ok := item.(string); ok {
				list = append(list, s)
			}
		}
		return list
	case []string:
		return v
	case map[string]any:
		if pref, ok := v["preference"]; ok {
			return ExtractGPUPreferences(pref)
		}
	}
	return nil
}

func ExtractVolumeSpecs(volField []any) []string {
	var list []string
	for _, v := range volField {
		switch item := v.(type) {
		case string:
			list = append(list, item)
		case map[string]any:
			source, _ := item["source"].(string)
			target, _ := item["target"].(string)
			mode, _ := item["mode"].(string)
			if mode == "" {
				mode = "ro"
			}
			if source != "" && target != "" {
				list = append(list, fmt.Sprintf("%s:%s:%s", source, target, mode))
			}
		}
	}
	return list
}
