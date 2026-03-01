package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "sandbox",
	Short: "Manage ephemeral Lima VMs for isolated Claude Code sessions",
	Long:  "Create, manage, and destroy ephemeral Lima VMs that provide hypervisor-level isolation for running Claude Code.",
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(createCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(attachCmd)
	rootCmd.AddCommand(destroyCmd)
	rootCmd.AddCommand(execCmd)
}
