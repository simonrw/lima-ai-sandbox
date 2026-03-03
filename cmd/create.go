package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/simonrw/lima-ai-sandbox/internal/config"
	"github.com/simonrw/lima-ai-sandbox/internal/lima"
	"github.com/simonrw/lima-ai-sandbox/internal/naming"
	"github.com/simonrw/lima-ai-sandbox/internal/template"
	"github.com/simonrw/lima-ai-sandbox/internal/worktree"
	"github.com/spf13/cobra"
)

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create and start a new sandbox VM",
	Long:  "Create a new ephemeral Lima VM, install Claude Code, and attach to an interactive session.",
	RunE:  runCreate,
}

var (
	flagName       string
	flagProjectDir string
	flagAPIKey     string
	flagCPUs       int
	flagMemory     string
	flagDisk       string
	flagNoAttach   bool
	flagBranch     string
)

func init() {
	createCmd.Flags().StringVar(&flagName, "name", "", "Instance name (default: auto-generated)")
	createCmd.Flags().StringVar(&flagProjectDir, "project-dir", "", "Project directory to mount at /workspace (default: cwd)")
	createCmd.Flags().StringVar(&flagAPIKey, "api-key", "", "Anthropic API key (default: $ANTHROPIC_API_KEY)")
	createCmd.Flags().IntVar(&flagCPUs, "cpus", 0, "Number of CPUs")
	createCmd.Flags().StringVar(&flagMemory, "memory", "", "Memory size (e.g. 4GiB)")
	createCmd.Flags().StringVar(&flagDisk, "disk", "", "Disk size (e.g. 50GiB)")
	createCmd.Flags().BoolVar(&flagNoAttach, "no-attach", false, "Don't attach after creation")
	createCmd.Flags().StringVar(&flagBranch, "branch", "", "Clone this branch inside the VM (served via git HTTP)")
}

func runCreate(cmd *cobra.Command, args []string) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	// Resolve API key
	apiKey := flagAPIKey
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}
	if apiKey == "" {
		return fmt.Errorf("API key required: set --api-key or $ANTHROPIC_API_KEY")
	}

	// Resolve project dir
	projectDir := flagProjectDir
	if projectDir == "" {
		var err error
		projectDir, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("getting working directory: %w", err)
		}
	}
	projectDir, err := filepath.Abs(projectDir)
	if err != nil {
		return fmt.Errorf("resolving project dir: %w", err)
	}

	// Generate or use provided name
	name := flagName
	if name == "" {
		var err error
		name, err = naming.Generate()
		if err != nil {
			return fmt.Errorf("generating name: %w", err)
		}
	}

	// Build template params
	params := template.Params{
		ProjectDir: projectDir,
		APIKey:     apiKey,
		CPUs:       flagCPUs,
		Memory:     flagMemory,
		Disk:       flagDisk,
	}

	var serverProcess *os.Process

	// If --branch is set, start a git HTTP server and configure clone-based provisioning
	if flagBranch != "" {
		fmt.Fprintf(os.Stderr, "Starting git HTTP server for branch %s...\n", flagBranch)

		repoRoot, err := worktree.RepoRoot(ctx, projectDir)
		if err != nil {
			return fmt.Errorf("finding repo root: %w", err)
		}

		gitUserName, gitUserEmail := worktree.GitUserConfig(ctx, repoRoot)

		// Find our own executable
		self, err := os.Executable()
		if err != nil {
			return fmt.Errorf("finding executable: %w", err)
		}

		// Start _serve as a detached child process
		proc := exec.Command(self, "_serve", "--repo", repoRoot)
		proc.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		proc.Stderr = os.Stderr

		// Read port from child's stdout
		stdout, err := proc.StdoutPipe()
		if err != nil {
			return fmt.Errorf("creating stdout pipe: %w", err)
		}

		if err := proc.Start(); err != nil {
			return fmt.Errorf("starting git server: %w", err)
		}
		serverProcess = proc.Process

		scanner := bufio.NewScanner(stdout)
		if !scanner.Scan() {
			serverProcess.Kill()
			return fmt.Errorf("git server did not report port")
		}
		port, err := strconv.Atoi(scanner.Text())
		if err != nil {
			serverProcess.Kill()
			return fmt.Errorf("invalid port from git server: %w", err)
		}

		// Save metadata
		if _, err := worktree.Create(ctx, projectDir, name, flagBranch, serverProcess.Pid, port); err != nil {
			serverProcess.Signal(syscall.SIGTERM)
			return fmt.Errorf("saving metadata: %w", err)
		}

		// Configure template for git HTTP clone
		params.GitURL = fmt.Sprintf("http://host.lima.internal:%d/", port)
		params.Branch = flagBranch
		params.GitUserName = gitUserName
		params.GitUserEmail = gitUserEmail
		params.ProjectDir = "" // no mount in branch mode

		fmt.Fprintf(os.Stderr, "Git server started on port %d (PID %d)\n", port, serverProcess.Pid)
	}

	// Render template
	tmplFile, err := template.Render(params)
	if err != nil {
		if serverProcess != nil {
			serverProcess.Signal(syscall.SIGTERM)
		}
		return fmt.Errorf("rendering template: %w", err)
	}
	defer os.Remove(tmplFile)

	fmt.Fprintf(os.Stderr, "Creating sandbox %s...\n", name)

	// Create instance
	if err := lima.Create(ctx, lima.CreateOpts{
		Name:         name,
		TemplateFile: tmplFile,
	}); err != nil {
		if flagBranch != "" {
			worktree.Remove(context.Background(), projectDir, name)
		}
		return err
	}

	// Clean up on failure
	cleanup := func() {
		fmt.Fprintf(os.Stderr, "\nCleaning up %s...\n", name)
		lima.Delete(context.Background(), name, true)
		if flagBranch != "" {
			worktree.Remove(context.Background(), projectDir, name)
		}
	}

	// Start instance
	fmt.Fprintf(os.Stderr, "Starting %s...\n", name)
	if err := lima.Start(ctx, name); err != nil {
		cleanup()
		return err
	}

	// Run post-checkout steps if --branch was used
	if flagBranch != "" {
		cfg, err := config.Load(projectDir)
		if err != nil {
			cleanup()
			return fmt.Errorf("loading .sandbox.yml: %w", err)
		}
		for _, step := range cfg.PostCheckout {
			fmt.Fprintf(os.Stderr, "Running post-checkout: %s\n", step)
			exitCode, err := lima.ShellRun(ctx, name, "/workspace", "bash", "-c", step)
			if err != nil {
				cleanup()
				return fmt.Errorf("post-checkout step %q: %w", step, err)
			}
			if exitCode != 0 {
				cleanup()
				return fmt.Errorf("post-checkout step %q exited with code %d", step, exitCode)
			}
		}
	}

	fmt.Fprintf(os.Stderr, "Sandbox %s is ready.\n", name)

	if flagNoAttach {
		return nil
	}

	// Attach: replace process with limactl shell
	fmt.Fprintf(os.Stderr, "Attaching to %s...\n", name)
	return lima.ShellExec(name, "/workspace", "claude")
}
