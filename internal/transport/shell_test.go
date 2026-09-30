package transport

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestShellCommandPreservesArguments(t *testing.T) {
	value := "spaces 'quotes' $HOME $(printf injected) `printf injected`\nnext line"
	cmd := exec.Command("sh", "-c", ShellCommand([]string{"printf", "%s", value}))
	out, err := cmd.Output()
	if err != nil || string(out) != value {
		t.Fatalf("arguments changed: %q, %v", out, err)
	}
}

func TestSSHExecPreservesShellScript(t *testing.T) {
	dir := t.TempDir()
	sshBinary := filepath.Join(dir, "ssh")
	// Run the remote command through a local shell just as an SSH server does.
	script := "#!/bin/sh\n[ \"$1\" = -- ] || exit 90\n[ \"$2\" = test-host ] || exit 91\nexec sh -c \"$3\"\n"
	if err := os.WriteFile(sshBinary, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	trans := NewSSHTransport("test-host", "", nil)
	trans.SSHBinary = sshBinary
	workDir := filepath.Join(dir, "working dir 'quoted'")
	if err := os.Mkdir(workDir, 0700); err != nil {
		t.Fatal(err)
	}
	value := "literal $HOME $(printf injected) 'quoted'"
	result, err := trans.Exec(context.Background(), []string{"bash", "-c", "printf '%s\\n' \"$CBOX_TEST_VALUE\"; pwd"}, ExecOptions{WorkDir: workDir, Env: map[string]string{"CBOX_TEST_VALUE": value}})
	if err != nil || result.ExitCode != 0 || string(result.Stdout) != value+"\n"+workDir+"\n" {
		t.Fatalf("unexpected remote execution: %+v, %v", result, err)
	}
}
