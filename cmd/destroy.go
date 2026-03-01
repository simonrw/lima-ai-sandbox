package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/simonrw/lima-ai-sandbox/internal/lima"
	"github.com/simonrw/lima-ai-sandbox/internal/naming"
	"github.com/spf13/cobra"
)

var destroyCmd = &cobra.Command{
	Use:   "destroy [name...]",
	Short: "Destroy sandbox instances",
	Long:  "Stop and delete sandbox VMs.",
	RunE:  runDestroy,
}

var (
	flagDestroyAll   bool
	flagDestroyForce bool
)

func init() {
	destroyCmd.Flags().BoolVar(&flagDestroyAll, "all", false, "Destroy all sandbox instances")
	destroyCmd.Flags().BoolVarP(&flagDestroyForce, "force", "f", false, "Force delete (skip stop)")
}

func runDestroy(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	names := args
	if flagDestroyAll {
		instances, err := lima.List(ctx)
		if err != nil {
			return err
		}
		for _, inst := range instances {
			if naming.IsSandbox(inst.Name) {
				names = append(names, inst.Name)
			}
		}
	}

	if len(names) == 0 {
		return fmt.Errorf("specify instance names or use --all")
	}

	var lastErr error
	for _, name := range names {
		fmt.Fprintf(os.Stderr, "Destroying %s...\n", name)

		if !flagDestroyForce {
			if err := lima.Stop(ctx, name); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: stop %s: %v\n", name, err)
			}
		}

		if err := lima.Delete(ctx, name, true); err != nil {
			fmt.Fprintf(os.Stderr, "Error: delete %s: %v\n", name, err)
			lastErr = err
			continue
		}

		fmt.Fprintf(os.Stderr, "Destroyed %s.\n", name)
	}
	return lastErr
}
