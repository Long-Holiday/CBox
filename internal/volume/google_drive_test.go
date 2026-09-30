package volume

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"cbox/internal/transport"
)

func TestGoogleDriveMountSpecs(t *testing.T) {
	for _, mode := range []string{"ro", "rw"} {
		m, err := ParseMountSpec("gdrive://MyDrive/data set:/data:" + mode)
		if err != nil || m.Source != "gdrive://MyDrive/data set" || !IsGoogleDriveMount(*m) || string(m.Mode) != mode {
			t.Fatalf("unexpected mount: %+v, %v", m, err)
		}
	}
	for _, spec := range []string{
		"gdrive://:/data:ro", "gdrive:///etc:/data:ro", "gdrive://MyDrive/../secret:/data:ro",
		"gdrive://MyDrive:/data:output", "gdrive://MyDrive:/data:cache",
		"gdrive://MyDrive:relative:ro", "gdrive://MyDrive:/:ro", "gdrive://MyDrive:/content:ro",
		"gdrive://MyDrive:/content/drive/subdir:rw", "gdrive://MyDrive:/data:ro:extra",
	} {
		if _, err := ParseMountSpec(spec); err == nil {
			t.Errorf("accepted invalid mount %s", spec)
		}
	}
}

type driveTransport struct {
	transport.Transport
	commands [][]string
	result   *transport.ExecResult
	err      error
	syncs    int
}

func (t *driveTransport) Exec(_ context.Context, command []string, _ transport.ExecOptions) (*transport.ExecResult, error) {
	t.commands = append(t.commands, command)
	return t.result, t.err
}
func (t *driveTransport) SyncTo(context.Context, string, string, transport.SyncOptions) error {
	t.syncs++
	return nil
}
func (t *driveTransport) SyncFrom(context.Context, string, string, transport.SyncOptions) error {
	t.syncs++
	return nil
}

func TestPrepareGoogleDriveSkipsLocalSyncAndCache(t *testing.T) {
	m, _ := ParseMountSpec("gdrive://MyDrive/data's $(name):/data:ro")
	trans := &driveTransport{result: &transport.ExecResult{}}
	service := NewService(nil, 0, nil)
	cache := map[string]string{}
	if err := service.PrepareMounts(context.Background(), trans, []Mount{*m}, cache); err != nil {
		t.Fatal(err)
	}
	if err := service.SyncManager().FinalSyncOutputs(context.Background(), trans, []Mount{*m}); err != nil {
		t.Fatal(err)
	}
	want := []string{"python3", "-c", googleDriveBindScript, GoogleDriveRoot, "MyDrive/data's $(name)", "/data", "ro"}
	if len(trans.commands) != 1 || !reflect.DeepEqual(trans.commands[0], want) || trans.syncs != 0 || len(cache) != 0 {
		t.Fatalf("unexpected cloud volume preparation: %+v, cache=%v", trans, cache)
	}
}

func TestPrepareGoogleDriveErrors(t *testing.T) {
	m, _ := ParseMountSpec("gdrive://MyDrive/data:/data:rw")
	for _, trans := range []*driveTransport{
		{err: errors.New("offline")},
		{result: &transport.ExecResult{ExitCode: 1, Stderr: []byte("source missing")}},
		{},
	} {
		err := NewService(nil, 0, nil).PrepareMounts(context.Background(), trans, []Mount{*m}, nil)
		if err == nil || !strings.Contains(err.Error(), "Google Drive") {
			t.Fatalf("expected cloud error, got %v", err)
		}
	}
}

// Execute the remote script with mount calls recorded instead of requiring root.
func TestGoogleDriveBindScript(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 is unavailable")
	}
	root := filepath.Join(t.TempDir(), "drive")
	source := filepath.Join(root, "MyDrive", "dataset")
	if err := os.MkdirAll(source, 0755); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	script := "import subprocess, json\ncalls = []\nsubprocess.run = lambda command, **kwargs: calls.append(command)\n" + googleDriveBindScript + "\nprint(json.dumps(calls))\n"
	for _, mode := range []string{"ro", "rw"} {
		target := filepath.Join(t.TempDir(), "target")
		output, err := exec.Command(python, "-c", script, root, "MyDrive/dataset", target, mode).CombinedOutput()
		if err != nil {
			t.Fatalf("bind script failed: %s, %v", output, err)
		}
		var commands [][]string
		if err := json.Unmarshal(output, &commands); err != nil {
			t.Fatal(err)
		}
		want := [][]string{{"mount", "--bind", source, target}, {"mount", "-o", "remount,bind," + mode, target}}
		if !reflect.DeepEqual(commands, want) {
			t.Fatalf("unexpected mount calls: %v", commands)
		}
	}
	for _, relative := range []string{"escape", "MyDrive/missing"} {
		target := filepath.Join(t.TempDir(), "target")
		if output, err := exec.Command(python, "-c", script, root, relative, target, "ro").CombinedOutput(); err == nil {
			t.Fatalf("accepted unsafe or missing source %s: %s", relative, output)
		}
		if _, err := os.Stat(target); !os.IsNotExist(err) {
			t.Fatalf("created target for invalid source: %v", err)
		}
	}
}
