package cmd

import (
	"context"
	"fmt"

	"github.com/simonrw/lima-ai-sandbox/internal/lima"
	"github.com/simonrw/lima-ai-sandbox/internal/naming"
	"github.com/spf13/cobra"
)

var attachCmd = &cobra.Command{
	Use:   "attach [name]",
	Short: "Attach to a running sandbox",
	Long:  "Attach to a running sandbox VM. If no name is given and exactly one sandbox is running, attach to it.",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runAttach,
}

var (
	flagAttachWorkdir string
	flagAttachShell   bool
)

func init() {
	attachCmd.Flags().StringVar(&flagAttachWorkdir, "workdir", "/workspace", "Working directory inside the VM")
	attachCmd.Flags().BoolVar(&flagAttachShell, "shell", false, "Open a bash shell instead of claude")
}

func runAttach(cmd *cobra.Command, args []string) error {
	name, err := resolveInstance(args)
	if err != nil {
		return err
	}

	command := "claude"
	if flagAttachShell {
		command = "bash"
	}

	return lima.ShellExec(name, flagAttachWorkdir, command)
}

// resolveInstance returns the instance name from args, or auto-detects if exactly one sandbox is running.
func resolveInstance(args []string) (string, error) {
	if len(args) == 1 {
		return args[0], nil
	}

	instances, err := lima.List(context.Background())
	if err != nil {
		return "", err
	}

	var running []string
	for _, inst := range instances {
		if naming.IsSandbox(inst.Name) && inst.Status == "Running" {
			running = append(running, inst.Name)
		}
	}

	switch len(running) {
	case 0:
		return "", fmt.Errorf("no running sandbox instances found")
	case 1:
		return running[0], nil
	default:
		return "", fmt.Errorf("multiple running sandboxes, specify one: %v", running)
	}
}
