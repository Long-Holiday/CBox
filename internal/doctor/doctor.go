package doctor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"cbox/internal/daemon"
)

type CheckItem struct {
	Name        string `json:"name"`
	Installed   bool   `json:"installed"`
	Version     string `json:"version,omitempty"`
	Description string `json:"description"`
	FixHint     string `json:"fix_hint,omitempty"`
}

type Report struct {
	Items   []CheckItem `json:"items"`
	AllGood bool        `json:"all_good"`
}

// CheckEnvironment performs environmental diagnosis
func CheckEnvironment(opts daemon.Options) *Report {
	report := &Report{AllGood: true}

	// 1. Check CBox Daemon
	daemonItem := CheckItem{
		Name:        "CBox Daemon",
		Description: "Background engine managing containers and ephemeral runtimes",
	}
	st, _ := daemon.Status(opts)
	if st != nil && st.Running {
		daemonItem.Installed = true
		daemonItem.Version = fmt.Sprintf("PID %d (v%s)", st.PID, st.Version)
	} else {
		daemonItem.Installed = false
		daemonItem.FixHint = "Run 'cbox daemon start' or let client auto-start it"
	}
	report.Items = append(report.Items, daemonItem)

	// 2. Check OpenSSH
	sshItem := CheckItem{
		Name:        "OpenSSH Client",
		Description: "Secure multiplexed shell transport to remote workers",
	}
	if sshPath, err := exec.LookPath("ssh"); err == nil {
		sshItem.Installed = true
		out, _ := exec.Command(sshPath, "-V").CombinedOutput()
		sshItem.Version = strings.TrimSpace(string(out))
	} else {
		sshItem.Installed = false
		sshItem.FixHint = "Install openssh-client via your system package manager (e.g. apt install openssh-client)"
		report.AllGood = false
	}
	report.Items = append(report.Items, sshItem)

	// 3. Check Rsync
	rsyncItem := CheckItem{
		Name:        "Rsync",
		Description: "High-performance incremental volume synchronization",
	}
	if rsyncPath, err := exec.LookPath("rsync"); err == nil {
		rsyncItem.Installed = true
		out, _ := exec.Command(rsyncPath, "--version").Output()
		lines := strings.Split(string(out), "\n")
		if len(lines) > 0 {
			rsyncItem.Version = strings.TrimSpace(lines[0])
		}
	} else {
		rsyncItem.Installed = false
		rsyncItem.FixHint = "Install rsync via your system package manager (e.g. apt install rsync)"
		report.AllGood = false
	}
	report.Items = append(report.Items, rsyncItem)

	// 4. Check Google Colab CLI
	colabItem := CheckItem{
		Name:        "Google Colab CLI",
		Description: "Command-line interface to spawn and control Colab GPU instances",
	}
	colabPath, colabErr := findExecutable("colab")
	if colabErr == nil {
		colabItem.Installed = true
		out, _ := exec.Command(colabPath, "version").Output()
		colabItem.Version = strings.TrimSpace(string(out))
		if colabItem.Version == "" {
			colabItem.Version = colabPath
		}
	} else {
		colabItem.Installed = false
		colabItem.FixHint = "Run 'cbox setup' or './scripts/install.sh' to install automatically"
		report.AllGood = false
	}
	report.Items = append(report.Items, colabItem)

	// 5. Check Colab Authentication
	authItem := CheckItem{
		Name:        "Colab Authentication",
		Description: "Google OAuth credentials for Colab API access",
	}
	if colabItem.Installed {
		home, _ := os.UserHomeDir()
		if colabCredentialsPresent(home) {
			authItem.Installed = true
			authItem.Version = "Credentials present"
		} else {
			authItem.FixHint = "Run 'colab sessions' or 'cbox setup' to authenticate"
			report.AllGood = false
		}
	} else {
		authItem.Installed = false
		authItem.FixHint = "Requires Google Colab CLI to be installed first"
		report.AllGood = false
	}
	report.Items = append(report.Items, authItem)

	return report
}

// SetupDependencies installs uv, google-colab-cli, and triggers authentication
func SetupDependencies(ctx context.Context) error {
	fmt.Println("==> Step 1/3: Checking environment and package managers...")

	// 1. Check or install uv
	uvPath, err := findExecutable("uv")
	if err != nil {
		fmt.Println("--> 'uv' not found. Installing uv (fast Python package manager)...")
		installCmd := exec.CommandContext(ctx, "bash", "-c", "curl -LsSf https://astral.sh/uv/install.sh | sh")
		installCmd.Stdout = os.Stdout
		installCmd.Stderr = os.Stderr
		if err := installCmd.Run(); err != nil {
			return fmt.Errorf("failed to install uv: %w", err)
		}

		// Update PATH in current process
		home, _ := os.UserHomeDir()
		uvBinDir := filepath.Join(home, ".local", "bin")
		_ = os.Setenv("PATH", uvBinDir+":"+os.Getenv("PATH"))

		uvPath, err = findExecutable("uv")
		if err != nil {
			return fmt.Errorf("uv installed but not found in PATH (~/.local/bin)")
		}
	}
	fmt.Printf("--> uv ready at: %s\n", uvPath)

	// 2. Install google-colab-cli with jupyter-kernel-client
	fmt.Println("\n==> Step 2/3: Installing google-colab-cli with jupyter-kernel-client...")
	installColabCmd := exec.CommandContext(ctx, uvPath, "tool", "install", "google-colab-cli",
		"--with", "jupyter-kernel-client @ git+https://github.com/googlecolab/jupyter-kernel-client.git",
		"--force",
	)
	installColabCmd.Stdout = os.Stdout
	installColabCmd.Stderr = os.Stderr
	if err := installColabCmd.Run(); err != nil {
		return fmt.Errorf("failed to install google-colab-cli: %w", err)
	}
	fmt.Println("--> google-colab-cli installed successfully!")

	// 3. Trigger authentication
	fmt.Println("\n==> Step 3/3: Authenticating with Google Colab...")
	colabPath, err := findExecutable("colab")
	if err != nil {
		home, _ := os.UserHomeDir()
		colabPath = filepath.Join(home, ".local", "bin", "colab")
	}

	authCmd := exec.CommandContext(ctx, colabPath, "sessions")
	authCmd.Stdin = os.Stdin
	authCmd.Stdout = os.Stdout
	authCmd.Stderr = os.Stderr

	if err := authCmd.Run(); err != nil {
		return fmt.Errorf("Colab authentication failed (retry with 'colab sessions'): %w", err)
	}
	home, _ := os.UserHomeDir()
	if !colabCredentialsPresent(home) {
		return fmt.Errorf("Colab credentials were not saved; run 'colab sessions' to authenticate")
	}
	fmt.Println("--> Colab authentication completed!")

	fmt.Println("\n==> Setup completed! You can now use 'cbox' to run containers on GPU.")
	return nil
}

// Colab performs OAuth when an API command runs and caches the token here.
// Check the cache without prompting for interactive login during doctor.
func colabCredentialsPresent(home string) bool {
	data, err := os.ReadFile(filepath.Join(home, ".config", "colab-cli", "token.json"))
	if err != nil {
		return false
	}
	var token struct {
		Token        string `json:"token"`
		RefreshToken string `json:"refresh_token"`
	}
	return json.Unmarshal(data, &token) == nil && (token.Token != "" || token.RefreshToken != "")
}

func findExecutable(name string) (string, error) {
	if p, err := exec.LookPath(name); err == nil {
		return p, nil
	}

	home, _ := os.UserHomeDir()
	candidatePaths := []string{
		filepath.Join(home, ".local", "bin", name),
		filepath.Join(home, ".cargo", "bin", name),
		filepath.Join("/usr/local/bin", name),
	}

	for _, p := range candidatePaths {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}

	return "", fmt.Errorf("executable %s not found", name)
}
