package worker

import (
	"embed"
	"os"
	"path/filepath"
)

//go:embed embedded/*.sh
var embeddedScripts embed.FS

func GetScript(name string) ([]byte, error) {
	// 1. Try local disk path if available
	localPath := filepath.Join("worker", name)
	if data, err := os.ReadFile(localPath); err == nil {
		return data, nil
	}

	// 2. Fallback to embedded script inside the binary
	return embeddedScripts.ReadFile("embedded/" + name)
}

func ListScripts() []string {
	return []string{"bootstrap.sh", "entrypoint.sh", "health.sh", "exec.sh"}
}
