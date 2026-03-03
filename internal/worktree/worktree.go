package worktree

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

const metadataDir = ".sandbox-worktrees"

// Metadata holds information about a sandbox's branch and git server.
type Metadata struct {
	SandboxName string `json:"sandbox_name"`
	Branch      string `json:"branch"`
	RepoRoot    string `json:"repo_root"`
	ServerPID   int    `json:"server_pid,omitempty"`
	ServerPort  int    `json:"server_port,omitempty"`
}

// RepoRoot returns the git repo root for the given directory.
func RepoRoot(ctx context.Context, dir string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--show-toplevel")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not a git repository (or git not installed): %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// GitUserConfig reads the git user.name and user.email config from the given directory.
func GitUserConfig(ctx context.Context, dir string) (name, email string) {
	if cmd := exec.CommandContext(ctx, "git", "config", "user.name"); cmd != nil {
		cmd.Dir = dir
		if out, err := cmd.Output(); err == nil {
			name = strings.TrimSpace(string(out))
		}
	}
	if cmd := exec.CommandContext(ctx, "git", "config", "user.email"); cmd != nil {
		cmd.Dir = dir
		if out, err := cmd.Output(); err == nil {
			email = strings.TrimSpace(string(out))
		}
	}
	return name, email
}

// metadataPath returns the path to the metadata JSON file for a sandbox.
func metadataPath(root, sandboxName string) string {
	return filepath.Join(root, metadataDir, sandboxName+".json")
}

// Create saves metadata for a sandbox branch association.
// Returns the repo root path.
func Create(ctx context.Context, repoDir, sandboxName, branch string, serverPID, serverPort int) (string, error) {
	root, err := RepoRoot(ctx, repoDir)
	if err != nil {
		return "", err
	}

	meta := Metadata{
		SandboxName: sandboxName,
		Branch:      branch,
		RepoRoot:    root,
		ServerPID:   serverPID,
		ServerPort:  serverPort,
	}

	dir := filepath.Join(root, metadataDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("creating metadata dir: %w", err)
	}

	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshaling metadata: %w", err)
	}
	if err := os.WriteFile(metadataPath(root, sandboxName), data, 0o644); err != nil {
		return "", fmt.Errorf("writing metadata: %w", err)
	}

	return root, nil
}

// Remove deletes metadata for a sandbox and kills the git server process.
// It is a no-op if no metadata exists.
func Remove(ctx context.Context, repoDir, sandboxName string) error {
	root, err := RepoRoot(ctx, repoDir)
	if err != nil {
		return err
	}

	meta, err := Lookup(root, sandboxName)
	if err != nil {
		return err
	}
	if meta == nil {
		return nil // no-op
	}

	// Kill the git HTTP server if running
	if meta.ServerPID > 0 {
		if proc, err := os.FindProcess(meta.ServerPID); err == nil {
			// Send SIGTERM; ignore errors (process may already be dead)
			proc.Signal(syscall.SIGTERM)
		}
	}

	// Delete metadata file
	if err := os.Remove(metadataPath(root, sandboxName)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("removing metadata: %w", err)
	}

	// Clean up empty metadata dir
	dir := filepath.Join(root, metadataDir)
	entries, err := os.ReadDir(dir)
	if err == nil && len(entries) == 0 {
		os.Remove(dir)
	}

	return nil
}

// Lookup reads the metadata for a sandbox from a known repo root.
// Returns nil, nil if no metadata exists.
func Lookup(repoRoot, sandboxName string) (*Metadata, error) {
	data, err := os.ReadFile(metadataPath(repoRoot, sandboxName))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var meta Metadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("parsing metadata: %w", err)
	}
	return &meta, nil
}

// LookupFromCwd finds metadata for a sandbox by walking up from the
// current working directory looking for the metadata directory.
func LookupFromCwd(sandboxName string) (*Metadata, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return LookupFromDir(dir, sandboxName)
}

// LookupFromDir finds metadata for a sandbox by walking up from dir
// looking for the metadata directory.
func LookupFromDir(dir, sandboxName string) (*Metadata, error) {
	for {
		metaFile := metadataPath(dir, sandboxName)
		if _, err := os.Stat(metaFile); err == nil {
			return Lookup(dir, sandboxName)
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, nil // reached filesystem root
		}
		dir = parent
	}
}
