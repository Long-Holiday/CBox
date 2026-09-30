package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"cbox/internal/client"
	"cbox/internal/config"
	"cbox/internal/daemon"
	"cbox/internal/doctor"
	"cbox/internal/transport"
	pkgApi "cbox/pkg/api"
	"github.com/spf13/cobra"
)

var (
	socketFlag    string
	formatFlag    string
	debugFlag     bool
	autoStartFlag bool = true
	cboxClient    *client.Client
)

func getClient() *client.Client {
	if cboxClient != nil {
		return cboxClient
	}

	socketPath := socketFlag
	if socketPath == "" {
		paths, err := config.ResolvePaths("")
		if err == nil {
			socketPath = paths.SocketPath
		} else {
			socketPath = "/tmp/cbox/cbox.sock"
		}
	}

	// Auto-start daemon if requested and daemon is currently unreachable
	if autoStartFlag {
		opts := daemon.Options{
			SocketPath: socketPath,
			Debug:      debugFlag,
		}
		status, _ := daemon.Status(opts)
		if status == nil || !status.Running {
			fmt.Fprintln(os.Stderr, "CBox engine daemon is not running. Starting background engine...")
			pid, err := daemon.StartBackground(opts)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to auto-start daemon: %v\n", err)
			} else {
				fmt.Fprintf(os.Stderr, "CBox daemon started successfully (PID: %d)\n", pid)
			}
		}
	}

	cboxClient = client.NewClient(socketPath)
	return cboxClient
}

func main() {
	rootCmd := &cobra.Command{
		Use:   "cbox",
		Short: "CBox: Docker-like Ephemeral GPU Runtime Engine",
		Long:  "CBox runs your containers seamlessly on ephemeral remote GPU providers such as Google Colab.",
	}

	rootCmd.PersistentFlags().StringVarP(&socketFlag, "socket", "s", "", "cboxd unix socket path")
	rootCmd.PersistentFlags().StringVar(&formatFlag, "format", "table", "output format (table|json)")
	rootCmd.PersistentFlags().BoolVar(&debugFlag, "debug", false, "enable verbose debug output")
	rootCmd.PersistentFlags().BoolVar(&autoStartFlag, "autostart", true, "automatically start background daemon if not running")

	// Subcommands
	rootCmd.AddCommand(newRunCmd())
	rootCmd.AddCommand(newCreateCmd())
	rootCmd.AddCommand(newStartCmd())
	rootCmd.AddCommand(newStopCmd())
	rootCmd.AddCommand(newRestartCmd())
	rootCmd.AddCommand(newRmCmd())
	rootCmd.AddCommand(newPsCmd())
	rootCmd.AddCommand(newInspectCmd())
	rootCmd.AddCommand(newLogsCmd())
	rootCmd.AddCommand(newExecCmd())
	rootCmd.AddCommand(newStatsCmd())

	rootCmd.AddCommand(newBuildCmd())
	rootCmd.AddCommand(newImagesCmd())
	rootCmd.AddCommand(newRmiCmd())

	rootCmd.AddCommand(newVolumeCmd())
	rootCmd.AddCommand(newContextCmd())
	rootCmd.AddCommand(newComposeCmd())

	rootCmd.AddCommand(newVersionCmd())
	rootCmd.AddCommand(newEventsCmd())
	rootCmd.AddCommand(newDaemonCmd())
	rootCmd.AddCommand(newDoctorCmd())
	rootCmd.AddCommand(newSetupCmd())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// cbox run
func newRunCmd() *cobra.Command {
	var detach bool
	var name string
	var gpu string
	var highMem bool
	var volumes []string
	var envList []string
	var workDir string
	var restart string
	var resumeCmd string
	var secrets []string

	cmd := &cobra.Command{
		Use:   "run [OPTIONS] IMAGE [COMMAND] [ARG...]",
		Short: "Run a command in a new container",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli := getClient()
			ctx := context.Background()

			imageName := args[0]
			var command []string
			if len(args) > 1 {
				command = args[1:]
			}

			envMap := make(map[string]string)
			for _, e := range envList {
				parts := strings.SplitN(e, "=", 2)
				if len(parts) == 2 {
					envMap[parts[0]] = parts[1]
				}
			}

			var gpuPrefs []string
			if gpu != "" {
				for _, g := range strings.Split(gpu, ",") {
					gpuPrefs = append(gpuPrefs, strings.TrimSpace(g))
				}
			}

			var resumeSlice []string
			if resumeCmd != "" {
				resumeSlice = strings.Fields(resumeCmd)
			}

			req := pkgApi.ContainerCreateRequest{
				Name:          name,
				Image:         imageName,
				GPUPreference: gpuPrefs,
				HighMemory:    highMem,
				Command:       command,
				Env:           envMap,
				WorkDir:       workDir,
				Volumes:       volumes,
				RestartPolicy: restart,
				ResumeCommand: resumeSlice,
				Secrets:       secrets,
			}

			c, err := cli.CreateContainer(ctx, req)
			if err != nil {
				return err
			}

			started, err := cli.StartContainer(ctx, c.ID, detach)
			if err != nil {
				return err
			}

			if detach {
				fmt.Println(started.ID)
				return nil
			}

			// Non-detach: attach and stream logs until finished
			fmt.Printf("Container %s started. Streaming logs...\n", started.Name)
			for {
				time.Sleep(2 * time.Second)
				cont, err := cli.GetContainer(ctx, started.ID)
				if err != nil {
					return err
				}

				logs, err := cli.GetLogs(ctx, started.ID)
				if err == nil && logs.Stdout != "" {
					fmt.Print(logs.Stdout)
				}

				if cont.State != "running" && cont.State != "starting" && cont.State != "preparing" && cont.State != "provisioning" {
					if cont.ExitCode != nil && *cont.ExitCode != 0 {
						os.Exit(*cont.ExitCode)
					}
					break
				}
			}
			return nil
		},
	}

	cmd.Flags().BoolVarP(&detach, "detach", "d", false, "Run container in background")
	cmd.Flags().StringVar(&name, "name", "", "Assign a name to the container")
	cmd.Flags().StringVar(&gpu, "gpu", "", "GPU preference (e.g. L4,T4)")
	cmd.Flags().BoolVar(&highMem, "high-mem", false, "Request high memory runtime")
	cmd.Flags().StringArrayVarP(&volumes, "volume", "v", nil, "Bind mount a volume (source:target[:mode])")
	cmd.Flags().StringArrayVarP(&envList, "env", "e", nil, "Set environment variables")
	cmd.Flags().StringVarP(&workDir, "workdir", "w", "", "Working directory inside the container")
	cmd.Flags().StringVar(&restart, "restart", "no", "Restart policy (no, on-failure, unless-stopped, always)")
	cmd.Flags().StringVar(&resumeCmd, "resume-command", "", "Command to execute when resuming after recovery")
	cmd.Flags().StringArrayVar(&secrets, "secret", nil, "Environment secret names to inject")

	return cmd
}

// cbox create
func newCreateCmd() *cobra.Command {
	var name, gpu, workDir, restart, resumeCmd string
	var highMem bool
	var volumes, envList, secrets []string

	cmd := &cobra.Command{
		Use:   "create [OPTIONS] IMAGE [COMMAND] [ARG...]",
		Short: "Create a new container without starting it",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli := getClient()
			ctx := context.Background()

			imageName := args[0]
			var command []string
			if len(args) > 1 {
				command = args[1:]
			}

			envMap := make(map[string]string)
			for _, e := range envList {
				parts := strings.SplitN(e, "=", 2)
				if len(parts) == 2 {
					envMap[parts[0]] = parts[1]
				}
			}

			var gpuPrefs []string
			if gpu != "" {
				for _, g := range strings.Split(gpu, ",") {
					gpuPrefs = append(gpuPrefs, strings.TrimSpace(g))
				}
			}

			var resumeSlice []string
			if resumeCmd != "" {
				resumeSlice = strings.Fields(resumeCmd)
			}

			req := pkgApi.ContainerCreateRequest{
				Name:          name,
				Image:         imageName,
				GPUPreference: gpuPrefs,
				HighMemory:    highMem,
				Command:       command,
				Env:           envMap,
				WorkDir:       workDir,
				Volumes:       volumes,
				RestartPolicy: restart,
				ResumeCommand: resumeSlice,
				Secrets:       secrets,
			}

			c, err := cli.CreateContainer(ctx, req)
			if err != nil {
				return err
			}

			fmt.Println(c.ID)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Assign a name to the container")
	cmd.Flags().StringVar(&gpu, "gpu", "", "GPU preference (e.g. L4,T4)")
	cmd.Flags().BoolVar(&highMem, "high-mem", false, "Request high memory runtime")
	cmd.Flags().StringArrayVarP(&volumes, "volume", "v", nil, "Bind mount a volume")
	cmd.Flags().StringArrayVarP(&envList, "env", "e", nil, "Set environment variables")
	cmd.Flags().StringVarP(&workDir, "workdir", "w", "", "Working directory")
	cmd.Flags().StringVar(&restart, "restart", "no", "Restart policy")
	cmd.Flags().StringVar(&resumeCmd, "resume-command", "", "Resume command")
	cmd.Flags().StringArrayVar(&secrets, "secret", nil, "Secret environment variable")

	return cmd
}

// cbox start
func newStartCmd() *cobra.Command {
	var attach bool
	cmd := &cobra.Command{
		Use:   "start [OPTIONS] CONTAINER",
		Short: "Start one or more stopped containers",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli := getClient()
			ctx := context.Background()

			c, err := cli.StartContainer(ctx, args[0], !attach)
			if err != nil {
				return err
			}
			fmt.Println(c.Name)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&attach, "attach", "a", false, "Attach STDOUT/STDERR and forward signals")
	return cmd
}

// cbox stop
func newStopCmd() *cobra.Command {
	var timeout int
	cmd := &cobra.Command{
		Use:   "stop [OPTIONS] CONTAINER",
		Short: "Stop a running container",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli := getClient()
			ctx := context.Background()

			c, err := cli.StopContainer(ctx, args[0], timeout)
			if err != nil {
				return err
			}
			fmt.Println(c.Name)
			return nil
		},
	}
	cmd.Flags().IntVarP(&timeout, "time", "t", 10, "Seconds to wait before killing the container")
	return cmd
}

// cbox restart
func newRestartCmd() *cobra.Command {
	var timeout int
	cmd := &cobra.Command{
		Use:   "restart [OPTIONS] CONTAINER",
		Short: "Restart a running container",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli := getClient()
			ctx := context.Background()

			c, err := cli.RestartContainer(ctx, args[0], timeout)
			if err != nil {
				return err
			}
			fmt.Println(c.Name)
			return nil
		},
	}
	cmd.Flags().IntVarP(&timeout, "time", "t", 10, "Seconds to wait before killing the container")
	return cmd
}

// cbox rm
func newRmCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "rm [OPTIONS] CONTAINER",
		Short: "Remove a container",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli := getClient()
			ctx := context.Background()

			if err := cli.RemoveContainer(ctx, args[0], force); err != nil {
				return err
			}
			fmt.Println(args[0])
			return nil
		},
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force the removal of a running container")
	return cmd
}

// cbox ps
func newPsCmd() *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:   "ps [OPTIONS]",
		Short: "List containers",
		RunE: func(cmd *cobra.Command, args []string) error {
			cli := getClient()
			ctx := context.Background()

			containers, err := cli.ListContainers(ctx)
			if err != nil {
				return err
			}

			if !all {
				var filtered []pkgApi.ContainerResponse
				for _, c := range containers {
					if c.State == "running" || c.State == "starting" || c.State == "preparing" || c.State == "provisioning" {
						filtered = append(filtered, c)
					}
				}
				containers = filtered
			}

			if formatFlag == "json" {
				return PrintJSON(os.Stdout, containers)
			}

			PrintContainerTable(os.Stdout, containers)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&all, "all", "a", true, "Show all containers (default shows all)")
	return cmd
}

// cbox inspect
func newInspectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "inspect CONTAINER",
		Short: "Return low-level information on container",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli := getClient()
			ctx := context.Background()

			c, err := cli.GetContainer(ctx, args[0])
			if err != nil {
				return err
			}

			return PrintJSON(os.Stdout, c)
		},
	}
	return cmd
}

// cbox logs
func newLogsCmd() *cobra.Command {
	var follow bool
	cmd := &cobra.Command{
		Use:   "logs [OPTIONS] CONTAINER",
		Short: "Fetch the logs of a container",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli := getClient()
			ctx := context.Background()

			if !follow {
				logs, err := cli.GetLogs(ctx, args[0])
				if err != nil {
					return err
				}
				if logs.Stdout != "" {
					fmt.Print(logs.Stdout)
				}
				if logs.Stderr != "" {
					fmt.Fprint(os.Stderr, logs.Stderr)
				}
				return nil
			}

			// Follow mode: poll logs
			lastLen := 0
			for {
				logs, err := cli.GetLogs(ctx, args[0])
				if err == nil {
					if len(logs.Stdout) > lastLen {
						fmt.Print(logs.Stdout[lastLen:])
						lastLen = len(logs.Stdout)
					}
				}

				c, err := cli.GetContainer(ctx, args[0])
				if err == nil && c.State != "running" && c.State != "starting" && c.State != "preparing" {
					break
				}
				time.Sleep(1 * time.Second)
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow log output")
	return cmd
}

// cbox exec
func newExecCmd() *cobra.Command {
	var interactive bool
	var tty bool

	cmd := &cobra.Command{
		Use:   "exec [OPTIONS] CONTAINER COMMAND [ARG...]",
		Short: "Run a command in a running container",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli := getClient()
			ctx := context.Background()
			containerID := args[0]
			command := args[1:]

			// Section 23: Interactive exec via direct SSH PTY attach
			if interactive || tty {
				info, err := cli.GetTransportInfo(ctx, containerID)
				if err != nil {
					return fmt.Errorf("get transport info for exec: %w", err)
				}

				sshArgs := []string{}
				if info.ConfigFile != "" {
					sshArgs = append(sshArgs, "-F", info.ConfigFile)
				}
				sshArgs = append(sshArgs, "-tt", "--", info.Host, transport.ShellCommand(command))

				sshCmd := exec.Command("ssh", sshArgs...)
				sshCmd.Stdin = os.Stdin
				sshCmd.Stdout = os.Stdout
				sshCmd.Stderr = os.Stderr

				return sshCmd.Run()
			}

			// Non-interactive exec via API
			res, err := cli.Exec(ctx, containerID, command, false)
			if err != nil {
				return err
			}

			if res.Stdout != "" {
				fmt.Print(res.Stdout)
			}
			if res.Stderr != "" {
				fmt.Fprint(os.Stderr, res.Stderr)
			}

			if res.ExitCode != 0 {
				os.Exit(res.ExitCode)
			}
			return nil
		},
	}

	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Keep STDIN open")
	cmd.Flags().BoolVarP(&tty, "tty", "t", false, "Allocate a pseudo-TTY")
	return cmd
}

// cbox stats
func newStatsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stats CONTAINER",
		Short: "Display container resource usage statistics",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli := getClient()
			ctx := context.Background()

			stats, err := cli.GetStats(ctx, args[0])
			if err != nil {
				return err
			}

			if formatFlag == "json" {
				return PrintJSON(os.Stdout, stats)
			}

			fmt.Printf("CONTAINER:        %s\n", stats.ContainerID)
			fmt.Printf("GPU:              %s\n", stats.GPUName)
			fmt.Printf("GPU MEMORY:       %d MB / %d MB\n", stats.GPUMemoryUsedMB, stats.GPUMemoryTotalMB)
			fmt.Printf("GPU UTILIZATION:  %.1f%%\n", stats.GPUUtilization)
			fmt.Printf("SYSTEM MEMORY:    %d MB / %d MB\n", stats.MemoryUsedMB, stats.MemoryTotalMB)
			return nil
		},
	}
	return cmd
}

// cbox build
func newBuildCmd() *cobra.Command {
	var tag string
	var file string

	cmd := &cobra.Command{
		Use:   "build [OPTIONS] PATH",
		Short: "Build an image from a Cboxfile",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli := getClient()
			ctx := context.Background()

			contextDir := "."
			if len(args) > 0 {
				contextDir = args[0]
			}

			if file == "" {
				file = filepath.Join(contextDir, "Cboxfile")
			}

			img, err := cli.BuildImage(ctx, contextDir, file, tag)
			if err != nil {
				return err
			}

			fmt.Printf("Successfully built %s\n", img.ID)
			if tag != "" {
				fmt.Printf("Successfully tagged %s\n", tag)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&tag, "tag", "t", "", "Name and optionally a tag in the 'name:tag' format")
	cmd.Flags().StringVarP(&file, "file", "f", "", "Name of the Cboxfile")
	return cmd
}

// cbox images
func newImagesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "images",
		Short: "List images",
		RunE: func(cmd *cobra.Command, args []string) error {
			cli := getClient()
			ctx := context.Background()

			images, err := cli.ListImages(ctx)
			if err != nil {
				return err
			}

			if formatFlag == "json" {
				return PrintJSON(os.Stdout, images)
			}

			PrintImageTable(os.Stdout, images)
			return nil
		},
	}
	return cmd
}

// cbox rmi
func newRmiCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rmi IMAGE",
		Short: "Remove one or more images",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli := getClient()
			ctx := context.Background()

			if err := cli.DeleteImage(ctx, args[0]); err != nil {
				return err
			}
			fmt.Printf("Untagged: %s\n", args[0])
			return nil
		},
	}
	return cmd
}

// cbox volume
func newVolumeCmd() *cobra.Command {
	volCmd := &cobra.Command{
		Use:   "volume",
		Short: "Manage volumes",
	}

	createCmd := &cobra.Command{
		Use:   "create [OPTIONS] [VOLUME]",
		Short: "Create a volume",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			source, _ := cmd.Flags().GetString("source")
			mode, _ := cmd.Flags().GetString("mode")
			cli := getClient()
			ctx := context.Background()

			v, err := cli.CreateVolume(ctx, args[0], source, mode)
			if err != nil {
				return err
			}
			fmt.Println(v.Name)
			return nil
		},
	}
	createCmd.Flags().String("source", "", "Source path on local server")
	createCmd.Flags().String("mode", "ro", "Volume mode (ro, rw, output, cache)")

	lsCmd := &cobra.Command{
		Use:   "ls",
		Short: "List volumes",
		RunE: func(cmd *cobra.Command, args []string) error {
			cli := getClient()
			ctx := context.Background()

			volumes, err := cli.ListVolumes(ctx)
			if err != nil {
				return err
			}

			if formatFlag == "json" {
				return PrintJSON(os.Stdout, volumes)
			}

			PrintVolumeTable(os.Stdout, volumes)
			return nil
		},
	}

	inspectCmd := &cobra.Command{
		Use:   "inspect VOLUME",
		Short: "Display detailed information on one or more volumes",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli := getClient()
			ctx := context.Background()

			v, err := cli.GetVolume(ctx, args[0])
			if err != nil {
				return err
			}
			return PrintJSON(os.Stdout, v)
		},
	}

	rmCmd := &cobra.Command{
		Use:   "rm VOLUME",
		Short: "Remove a volume",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli := getClient()
			ctx := context.Background()

			if err := cli.DeleteVolume(ctx, args[0]); err != nil {
				return err
			}
			fmt.Println(args[0])
			return nil
		},
	}

	volCmd.AddCommand(createCmd, lsCmd, inspectCmd, rmCmd)
	return volCmd
}

// cbox context
func newContextCmd() *cobra.Command {
	ctxCmd := &cobra.Command{
		Use:   "context",
		Short: "Manage contexts and provider profiles",
	}

	createCmd := &cobra.Command{
		Use:   "create [OPTIONS] CONTEXT",
		Short: "Create a context",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			provider, _ := cmd.Flags().GetString("provider")
			profile, _ := cmd.Flags().GetString("profile")
			autoSched, _ := cmd.Flags().GetBool("auto-schedule")

			cli := getClient()
			ctx := context.Background()

			c, err := cli.CreateContext(ctx, pkgApi.ContextCreateRequest{
				Name:         args[0],
				Provider:     provider,
				Profile:      profile,
				AutoSchedule: autoSched,
			})
			if err != nil {
				return err
			}
			fmt.Println(c.Name)
			return nil
		},
	}
	createCmd.Flags().String("provider", "colab", "Provider name (default: colab)")
	createCmd.Flags().String("profile", "default", "Profile name")
	createCmd.Flags().Bool("auto-schedule", true, "Enable automatic scheduling")

	lsCmd := &cobra.Command{
		Use:   "ls",
		Short: "List contexts",
		RunE: func(cmd *cobra.Command, args []string) error {
			cli := getClient()
			ctx := context.Background()

			contexts, err := cli.ListContexts(ctx)
			if err != nil {
				return err
			}

			if formatFlag == "json" {
				return PrintJSON(os.Stdout, contexts)
			}

			PrintContextTable(os.Stdout, contexts)
			return nil
		},
	}

	useCmd := &cobra.Command{
		Use:   "use CONTEXT",
		Short: "Set the current cbox context",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli := getClient()
			ctx := context.Background()

			if err := cli.UseContext(ctx, args[0]); err != nil {
				return err
			}
			fmt.Printf("Current context is now %q\n", args[0])
			return nil
		},
	}

	rmCmd := &cobra.Command{
		Use:   "rm CONTEXT",
		Short: "Remove a context",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli := getClient()
			ctx := context.Background()

			if err := cli.DeleteContext(ctx, args[0]); err != nil {
				return err
			}
			fmt.Println(args[0])
			return nil
		},
	}

	ctxCmd.AddCommand(createCmd, lsCmd, useCmd, rmCmd)
	return ctxCmd
}

// cbox compose
func newComposeCmd() *cobra.Command {
	composeCmd := &cobra.Command{
		Use:   "compose",
		Short: "Multi-container orchestration",
	}

	var file string
	var detach bool

	upCmd := &cobra.Command{
		Use:   "up [OPTIONS]",
		Short: "Create and start containers",
		RunE: func(cmd *cobra.Command, args []string) error {
			cli := getClient()
			ctx := context.Background()

			containers, err := cli.ComposeUp(ctx, file, detach)
			if err != nil {
				return err
			}

			if formatFlag == "json" {
				return PrintJSON(os.Stdout, containers)
			}

			PrintContainerTable(os.Stdout, containers)
			return nil
		},
	}
	upCmd.Flags().StringVarP(&file, "file", "f", "cbox-compose.yaml", "Compose configuration file")
	upCmd.Flags().BoolVarP(&detach, "detach", "d", true, "Run containers in the background")

	downCmd := &cobra.Command{
		Use:   "down [OPTIONS]",
		Short: "Stop and remove containers",
		RunE: func(cmd *cobra.Command, args []string) error {
			cli := getClient()
			ctx := context.Background()

			if err := cli.ComposeDown(ctx, file); err != nil {
				return err
			}
			fmt.Println("All services stopped and removed.")
			return nil
		},
	}
	downCmd.Flags().StringVarP(&file, "file", "f", "cbox-compose.yaml", "Compose configuration file")

	psCmd := &cobra.Command{
		Use:   "ps [OPTIONS]",
		Short: "List compose containers",
		RunE: func(cmd *cobra.Command, args []string) error {
			cli := getClient()
			ctx := context.Background()

			containers, err := cli.ComposePs(ctx, file)
			if err != nil {
				return err
			}

			if formatFlag == "json" {
				return PrintJSON(os.Stdout, containers)
			}

			PrintContainerTable(os.Stdout, containers)
			return nil
		},
	}
	psCmd.Flags().StringVarP(&file, "file", "f", "cbox-compose.yaml", "Compose configuration file")

	logsCmd := &cobra.Command{
		Use:   "logs [OPTIONS]",
		Short: "View output from compose containers",
		RunE: func(cmd *cobra.Command, args []string) error {
			cli := getClient()
			ctx := context.Background()

			logs, err := cli.ComposeLogs(ctx, file)
			if err != nil {
				return err
			}

			for svc, content := range logs {
				fmt.Printf("=== Service: %s ===\n%s\n\n", svc, content)
			}
			return nil
		},
	}
	logsCmd.Flags().StringVarP(&file, "file", "f", "cbox-compose.yaml", "Compose configuration file")

	composeCmd.AddCommand(upCmd, downCmd, psCmd, logsCmd)
	return composeCmd
}

// cbox version
func newVersionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Show the CBox version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			cli := getClient()
			ctx := context.Background()

			v, err := cli.Version(ctx)
			if err != nil {
				// Daemon might not be running, display client info
				fmt.Println("Client: CBox v0.1.0 (Go)")
				fmt.Printf("Error contacting daemon: %v\n", err)
				return nil
			}

			if formatFlag == "json" {
				return PrintJSON(os.Stdout, v)
			}

			fmt.Printf("CBox Version: %s\nGo:           %s\nPlatform:     %s/%s\n",
				v.Version, v.GoVersion, v.OS, v.Arch,
			)
			return nil
		},
	}
	return cmd
}

// cbox events
func newEventsCmd() *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "events",
		Short: "Get real time events from the server",
		RunE: func(cmd *cobra.Command, args []string) error {
			cli := getClient()
			ctx := context.Background()

			events, err := cli.GetEvents(ctx, limit, "", "")
			if err != nil {
				return err
			}

			if formatFlag == "json" {
				return PrintJSON(os.Stdout, events)
			}

			for _, e := range events {
				fmt.Printf("[%s] %s (%s %s): %s\n",
					e.Timestamp.Format("2006-01-02 15:04:05"),
					e.Type, e.ObjectType, e.ObjectID, e.Payload,
				)
			}
			return nil
		},
	}
	cmd.Flags().IntVarP(&limit, "limit", "n", 20, "Number of events to retrieve")
	return cmd
}

// cbox daemon
func newDaemonCmd() *cobra.Command {
	var configPath string
	var socketPath string
	var tcpAddr string
	var debug bool

	daemonCmd := &cobra.Command{
		Use:   "daemon",
		Short: "Manage the CBox engine daemon",
		Long:  "Run or manage the background CBox engine daemon that manages containers, runtimes, and storage.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	daemonCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "", "path to config file")
	daemonCmd.PersistentFlags().StringVarP(&socketPath, "socket", "s", "", "override unix socket path")
	daemonCmd.PersistentFlags().StringVar(&tcpAddr, "tcp", "", "optional TCP listen address (e.g. 127.0.0.1:8080)")
	daemonCmd.PersistentFlags().BoolVar(&debug, "debug", false, "enable debug logging")

	// cbox daemon run (foreground)
	runCmd := &cobra.Command{
		Use:   "run",
		Short: "Run the CBox engine daemon in the foreground",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := daemon.Options{
				ConfigPath: configPath,
				SocketPath: socketPath,
				TCPAddr:    tcpAddr,
				Debug:      debug || debugFlag,
			}
			return daemon.Run(context.Background(), opts)
		},
	}
	daemonCmd.AddCommand(runCmd)

	// cbox daemon start (background)
	startCmd := &cobra.Command{
		Use:   "start",
		Short: "Start the CBox engine daemon in the background",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := daemon.Options{
				ConfigPath: configPath,
				SocketPath: socketPath,
				TCPAddr:    tcpAddr,
				Debug:      debug || debugFlag,
			}
			pid, err := daemon.StartBackground(opts)
			if err != nil {
				return err
			}
			fmt.Printf("CBox daemon started in background (PID: %d)\n", pid)
			return nil
		},
	}
	daemonCmd.AddCommand(startCmd)

	// cbox daemon stop
	stopCmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop the running CBox engine daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := daemon.Options{
				ConfigPath: configPath,
				SocketPath: socketPath,
			}
			if err := daemon.Stop(opts); err != nil {
				return err
			}
			fmt.Println("CBox daemon stopped")
			return nil
		},
	}
	daemonCmd.AddCommand(stopCmd)

	// cbox daemon restart
	restartCmd := &cobra.Command{
		Use:   "restart",
		Short: "Restart the CBox engine daemon in the background",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := daemon.Options{
				ConfigPath: configPath,
				SocketPath: socketPath,
				TCPAddr:    tcpAddr,
				Debug:      debug || debugFlag,
			}
			pid, err := daemon.Restart(opts)
			if err != nil {
				return err
			}
			fmt.Printf("CBox daemon restarted in background (PID: %d)\n", pid)
			return nil
		},
	}
	daemonCmd.AddCommand(restartCmd)

	// cbox daemon status
	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show CBox engine daemon status",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := daemon.Options{
				ConfigPath: configPath,
				SocketPath: socketPath,
			}
			st, err := daemon.Status(opts)
			if err != nil {
				return err
			}
			if formatFlag == "json" {
				return PrintJSON(os.Stdout, st)
			}
			if st.Running {
				fmt.Printf("Status:  Running\nPID:     %d\nSocket:  %s\nVersion: %s\n", st.PID, st.SocketPath, st.Version)
			} else {
				fmt.Printf("Status:  Stopped\nSocket:  %s\n", st.SocketPath)
				if st.Error != "" {
					fmt.Printf("Warning: %s\n", st.Error)
				}
			}
			return nil
		},
	}
	daemonCmd.AddCommand(statusCmd)

	return daemonCmd
}

// cbox doctor
func newDoctorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check system environment and dependencies",
		Long:  "Inspect dependencies (ssh, rsync, colab CLI, authentication, daemon) required by CBox.",
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, _ := config.ResolvePaths("")
			socket := socketFlag
			if socket == "" && paths != nil {
				socket = paths.SocketPath
			}
			rep := doctor.CheckEnvironment(daemon.Options{
				SocketPath: socket,
				Debug:      debugFlag,
			})

			if formatFlag == "json" {
				return PrintJSON(os.Stdout, rep)
			}

			fmt.Println("CBox Environment Diagnosis:")
			fmt.Println("----------------------------------------------------------------------")
			for _, item := range rep.Items {
				statusMark := "[\033[32m✔\033[0m]"
				if !item.Installed {
					statusMark = "[\033[31m✖\033[0m]"
				}
				fmt.Printf("%s %-22s : ", statusMark, item.Name)
				if item.Installed {
					if item.Version != "" {
						fmt.Printf("%s\n", item.Version)
					} else {
						fmt.Printf("OK\n")
					}
				} else {
					fmt.Printf("NOT FOUND\n")
					if item.FixHint != "" {
						fmt.Printf("    -> Hint: %s\n", item.FixHint)
					}
				}
			}
			fmt.Println("----------------------------------------------------------------------")
			if rep.AllGood {
				fmt.Println("All required dependencies are satisfied! CBox is ready to use.")
			} else {
				fmt.Println("Some dependencies are missing. Run 'cbox setup' to configure Colab automatically.")
			}
			return nil
		},
	}
	return cmd
}

// cbox setup
func newSetupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Automatically install Colab CLI dependencies and authenticate",
		Long:  "Setup uv, google-colab-cli with jupyter-kernel-client, and trigger Google account authentication.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return doctor.SetupDependencies(context.Background())
		},
	}
	return cmd
}
