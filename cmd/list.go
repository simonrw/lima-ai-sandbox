package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/simonrw/lima-ai-sandbox/internal/lima"
	"github.com/simonrw/lima-ai-sandbox/internal/naming"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List sandbox instances",
	RunE:  runList,
}

var (
	flagListJSON  bool
	flagListQuiet bool
)

func init() {
	listCmd.Flags().BoolVar(&flagListJSON, "json", false, "Output as JSON")
	listCmd.Flags().BoolVarP(&flagListQuiet, "quiet", "q", false, "Only print names")
}

func runList(cmd *cobra.Command, args []string) error {
	instances, err := lima.List(context.Background())
	if err != nil {
		return err
	}

	// Filter to sandbox instances
	var sandboxes []lima.Instance
	for _, inst := range instances {
		if naming.IsSandbox(inst.Name) {
			sandboxes = append(sandboxes, inst)
		}
	}

	if flagListJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(sandboxes)
	}

	if flagListQuiet {
		for _, s := range sandboxes {
			fmt.Println(s.Name)
		}
		return nil
	}

	if len(sandboxes) == 0 {
		fmt.Println("No sandbox instances.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tSTATUS\tARCH\tCPUS\tMEMORY\tDISK")
	for _, s := range sandboxes {
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\t%s\n",
			s.Name, s.Status, s.Arch, s.CPUs,
			formatBytes(s.Memory), formatBytes(s.Disk))
	}
	return w.Flush()
}

func formatBytes(b int64) string {
	if b == 0 {
		return "-"
	}
	const gib = 1024 * 1024 * 1024
	if b >= gib {
		return fmt.Sprintf("%.0fGiB", float64(b)/float64(gib))
	}
	const mib = 1024 * 1024
	return fmt.Sprintf("%.0fMiB", float64(b)/float64(mib))
}
