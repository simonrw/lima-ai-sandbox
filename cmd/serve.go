package cmd

import (
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"strings"

	"github.com/simonrw/lima-ai-sandbox/internal/githttp"
	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:    "_serve",
	Short:  "Run git HTTP server (internal)",
	Hidden: true,
	RunE:   runServe,
}

var flagServeRepo string

func init() {
	serveCmd.Flags().StringVar(&flagServeRepo, "repo", "", "Path to git repository")
	serveCmd.MarkFlagRequired("repo") //nolint: errcheck
}

func runServe(cmd *cobra.Command, args []string) error {
	repoPath := flagServeRepo

	// Enable receive.denyCurrentBranch=updateInstead so pushes to checked-out branch work
	gitCfg := exec.Command("git", "-C", repoPath, "config", "receive.denyCurrentBranch", "updateInstead")
	if out, err := gitCfg.CombinedOutput(); err != nil {
		return fmt.Errorf("git config: %s: %w", strings.TrimSpace(string(out)), err)
	}

	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	port := ln.Addr().(*net.TCPAddr).Port
	// Print port to stdout for parent process to read
	fmt.Println(port)

	srv := &http.Server{Handler: githttp.Handler(repoPath)}
	return srv.Serve(ln)
}
