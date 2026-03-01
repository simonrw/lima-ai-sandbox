package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/simonrw/lima-ai-sandbox/internal/lima"
	"github.com/spf13/cobra"
)

var execCmd = &cobra.Command{
	Use:   "exec <name> -- <command...>",
	Short: "Run a command in a sandbox",
	Long:  "Execute a one-off command inside a running sandbox VM.",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runExec,
}

var flagExecWorkdir string

func init() {
	execCmd.Flags().StringVar(&flagExecWorkdir, "workdir", "/workspace", "Working directory inside the VM")
}

func runExec(cmd *cobra.Command, args []string) error {
	name := args[0]
	if len(args) < 2 {
		return fmt.Errorf("specify a command after the instance name")
	}
	command := args[1:]

	exitCode, err := lima.ShellRun(context.Background(), name, flagExecWorkdir, command...)
	if err != nil {
		return err
	}
	if exitCode != 0 {
		os.Exit(exitCode)
	}
	return nil
}
