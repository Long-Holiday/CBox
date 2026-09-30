package compose

import (
	"fmt"
	"os"
	"strings"

	"cbox/internal/volume"

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

	for name, svc := range cfg.Services {
		if _, err := ParseVolumeSpecs(svc.Volumes); err != nil {
			return nil, fmt.Errorf("service %s: %w", name, err)
		}
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

// ExtractVolumeSpecs is retained for callers that already validated the config.
func ExtractVolumeSpecs(volField []any) []string {
	specs, _ := ParseVolumeSpecs(volField)
	return specs
}

func ParseVolumeSpecs(volField []any) ([]string, error) {
	var list []string
	for i, v := range volField {
		var spec string
		switch item := v.(type) {
		case string:
			spec = item
		case map[string]any:
			source, _ := item["source"].(string)
			target, _ := item["target"].(string)
			mode, _ := item["mode"].(string)
			if mode == "" {
				mode = "ro"
			}
			if typ, exists := item["type"]; exists {
				switch typ {
				case "google-drive":
					source = volume.GoogleDrivePrefix + strings.TrimPrefix(source, volume.GoogleDrivePrefix)
				case "bind":
				default:
					return nil, fmt.Errorf("volume %d: unsupported type %v", i+1, typ)
				}
			}
			spec = fmt.Sprintf("%s:%s:%s", source, target, mode)
		default:
			return nil, fmt.Errorf("volume %d: expected a string or mapping", i+1)
		}
		if _, err := volume.ParseMountSpec(spec); err != nil {
			return nil, fmt.Errorf("volume %d: %w", i+1, err)
		}
		list = append(list, spec)
	}
	return list, nil
}
