package image

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"cbox/internal/transport"
)

type Materializer struct {
	logger *slog.Logger
}

func NewMaterializer(logger *slog.Logger) *Materializer {
	if logger == nil {
		logger = slog.Default()
	}
	return &Materializer{logger: logger}
}

func (m *Materializer) Materialize(ctx context.Context, trans transport.Transport, img *Image, contextDir string) error {
	readyFile := fmt.Sprintf("/content/.cbox/images/%s/ready", img.ID)

	// Check if already materialized on this remote worker
	res, err := trans.Exec(ctx, []string{"test", "-f", readyFile}, transport.ExecOptions{})
	if err == nil && res.ExitCode == 0 {
		m.logger.Info("image cache hit on remote runtime", "image_id", img.ID)
		return nil
	}

	m.logger.Info("materializing image on remote runtime", "image_id", img.ID)

	// Ensure image directory
	imgDir := fmt.Sprintf("/content/.cbox/images/%s", img.ID)
	_, _ = trans.Exec(ctx, []string{"mkdir", "-p", imgDir}, transport.ExecOptions{})

	// 1. APT packages
	if len(img.Manifest.Apt) > 0 {
		m.logger.Info("installing apt packages", "packages", img.Manifest.Apt)
		aptCmd := fmt.Sprintf("apt-get update -qq && DEBIAN_FRONTEND=noninteractive apt-get install -y -qq %s", strings.Join(img.Manifest.Apt, " "))
		res, err := trans.Exec(ctx, []string{"bash", "-c", aptCmd}, transport.ExecOptions{})
		if err != nil || res.ExitCode != 0 {
			return fmt.Errorf("apt install failed: %v (exit %d: %s)", err, res.ExitCode, string(res.Stderr))
		}
	}

	// 2. PIP packages
	if len(img.Manifest.Pip) > 0 {
		m.logger.Info("installing pip packages", "packages", img.Manifest.Pip)
		pipCmd := fmt.Sprintf("pip install -q %s", strings.Join(img.Manifest.Pip, " "))
		res, err := trans.Exec(ctx, []string{"bash", "-c", pipCmd}, transport.ExecOptions{})
		if err != nil || res.ExitCode != 0 {
			return fmt.Errorf("pip install failed: %v (exit %d: %s)", err, res.ExitCode, string(res.Stderr))
		}
	}

	// 3. COPY files
	for _, copySpec := range img.Manifest.Copies {
		srcPath := filepath.Join(contextDir, copySpec.Source)
		m.logger.Info("copying file into image", "source", srcPath, "target", copySpec.Target)

		// Ensure remote destination dir
		targetDir := filepath.Dir(copySpec.Target)
		_, _ = trans.Exec(ctx, []string{"mkdir", "-p", targetDir}, transport.ExecOptions{})

		if err := trans.CopyTo(ctx, srcPath, copySpec.Target); err != nil {
			return fmt.Errorf("copy %s to %s failed: %w", srcPath, copySpec.Target, err)
		}
	}

	// 4. RUN commands
	for _, runCmd := range img.Manifest.Runs {
		m.logger.Info("running build step", "command", runCmd)
		opts := transport.ExecOptions{}
		if img.Manifest.WorkDir != "" {
			opts.WorkDir = img.Manifest.WorkDir
		}
		res, err := trans.Exec(ctx, []string{"bash", "-c", runCmd}, opts)
		if err != nil || res.ExitCode != 0 {
			return fmt.Errorf("run command %q failed: %v (exit %d: %s)", runCmd, err, res.ExitCode, string(res.Stderr))
		}
	}

	// 5. Create ready marker
	markerCmd := fmt.Sprintf("touch %q", readyFile)
	_, _ = trans.Exec(ctx, []string{"bash", "-c", markerCmd}, transport.ExecOptions{})

	m.logger.Info("image successfully materialized", "image_id", img.ID)
	return nil
}
