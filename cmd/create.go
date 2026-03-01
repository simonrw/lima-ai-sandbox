package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/simonrw/lima-ai-sandbox/internal/lima"
	"github.com/simonrw/lima-ai-sandbox/internal/naming"
	"github.com/simonrw/lima-ai-sandbox/internal/template"
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
)

func init() {
	createCmd.Flags().StringVar(&flagName, "name", "", "Instance name (default: auto-generated)")
	createCmd.Flags().StringVar(&flagProjectDir, "project-dir", "", "Project directory to mount at /workspace (default: cwd)")
	createCmd.Flags().StringVar(&flagAPIKey, "api-key", "", "Anthropic API key (default: $ANTHROPIC_API_KEY)")
	createCmd.Flags().IntVar(&flagCPUs, "cpus", 0, "Number of CPUs")
	createCmd.Flags().StringVar(&flagMemory, "memory", "", "Memory size (e.g. 4GiB)")
	createCmd.Flags().StringVar(&flagDisk, "disk", "", "Disk size (e.g. 50GiB)")
	createCmd.Flags().BoolVar(&flagNoAttach, "no-attach", false, "Don't attach after creation")
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

	// Render template
	tmplFile, err := template.Render(template.Params{
		ProjectDir: projectDir,
		APIKey:     apiKey,
		CPUs:       flagCPUs,
		Memory:     flagMemory,
		Disk:       flagDisk,
	})
	if err != nil {
		return fmt.Errorf("rendering template: %w", err)
	}
	defer os.Remove(tmplFile)

	fmt.Fprintf(os.Stderr, "Creating sandbox %s...\n", name)

	// Create instance
	if err := lima.Create(ctx, lima.CreateOpts{
		Name:         name,
		TemplateFile: tmplFile,
	}); err != nil {
		return err
	}

	// Clean up on interrupt during start
	cleanup := func() {
		fmt.Fprintf(os.Stderr, "\nCleaning up %s...\n", name)
		lima.Delete(context.Background(), name, true)
	}

	// Start instance
	fmt.Fprintf(os.Stderr, "Starting %s...\n", name)
	if err := lima.Start(ctx, name); err != nil {
		cleanup()
		return err
	}

	fmt.Fprintf(os.Stderr, "Sandbox %s is ready.\n", name)

	if flagNoAttach {
		return nil
	}

	// Attach: replace process with limactl shell
	fmt.Fprintf(os.Stderr, "Attaching to %s...\n", name)
	return lima.ShellExec(name, "/workspace", "claude")
}
