package volume

import (
	"context"
	"fmt"
	"path"
	"strings"

	"cbox/internal/transport"
)

const GoogleDrivePrefix = "gdrive://"
const GoogleDriveRoot = "/content/drive"

func IsGoogleDriveMount(m Mount) bool { return strings.HasPrefix(m.Source, GoogleDrivePrefix) }

func ValidateGoogleDriveMount(source, target string, mode VolumeMode) error {
	if source == "" || strings.HasPrefix(source, "/") || strings.ContainsAny(source+target, "\x00\n\r:") {
		return fmt.Errorf("Google Drive source must be a relative path such as MyDrive/datasets")
	}
	for _, part := range strings.Split(source, "/") {
		if part == ".." {
			return fmt.Errorf("Google Drive source must not contain '..'")
		}
	}
	if mode != ModeReadOnly && mode != ModeReadWrite {
		return fmt.Errorf("Google Drive mounts support only ro or rw")
	}
	target = path.Clean(target)
	if !path.IsAbs(target) || target == "/" || target == "/content" || target == GoogleDriveRoot || strings.HasPrefix(target, GoogleDriveRoot+"/") {
		return fmt.Errorf("Google Drive target must be absolute and outside %s and its ancestors", GoogleDriveRoot)
	}
	return nil
}

// Bind mounts avoid copying cloud files and allow per-target read-only access.
const googleDriveBindScript = `import os, sys, subprocess
root, relative, target, mode = sys.argv[1:]
source = os.path.realpath(os.path.join(root, relative))
root = os.path.realpath(root)
if os.path.commonpath([root, source]) != root:
    raise RuntimeError('Google Drive source escapes the Drive root')
if not os.path.exists(source):
    raise RuntimeError('Google Drive source does not exist: ' + source)
target = os.path.abspath(target)
resolved = os.path.realpath(target)
if resolved == '/' or resolved == '/content' or resolved == root or resolved.startswith(root + '/'):
    raise RuntimeError('Unsafe Google Drive target: ' + target)
if os.path.islink(target):
    raise RuntimeError('Google Drive target must not be a symlink: ' + target)
os.makedirs(os.path.dirname(target), exist_ok=True)
if os.path.isdir(source):
    os.makedirs(target, exist_ok=True)
elif not os.path.exists(target):
    open(target, 'a').close()
if os.path.ismount(target):
    if not os.path.samefile(source, target):
        raise RuntimeError('Target is already mounted from another source: ' + target)
else:
    subprocess.run(['mount', '--bind', source, target], check=True)
subprocess.run(['mount', '-o', 'remount,bind,' + mode, target], check=True)
`

func prepareGoogleDriveMount(ctx context.Context, trans transport.Transport, m Mount) error {
	source := strings.TrimPrefix(m.Source, GoogleDrivePrefix)
	if err := ValidateGoogleDriveMount(source, m.Target, m.Mode); err != nil {
		return err
	}
	res, err := trans.Exec(ctx, []string{"python3", "-c", googleDriveBindScript, GoogleDriveRoot, source, m.Target, string(m.Mode)}, transport.ExecOptions{})
	if err != nil {
		return fmt.Errorf("mount Google Drive at %s: %w", m.Target, err)
	}
	if res == nil {
		return fmt.Errorf("mount Google Drive at %s: empty result", m.Target)
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("mount Google Drive at %s: %s", m.Target, strings.TrimSpace(string(res.Stderr)))
	}
	return nil
}
