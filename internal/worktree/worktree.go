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
)

const worktreeDir = ".sandbox-worktrees"

// Metadata holds information about a sandbox's worktree.
type Metadata struct {
	SandboxName  string `json:"sandbox_name"`
	WorktreePath string `json:"worktree_path"`
	Branch       string `json:"branch"`
	RepoRoot     string `json:"repo_root"`
}

// repoRoot returns the git repo root for the given directory.
func repoRoot(ctx context.Context, dir string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--show-toplevel")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not a git repository (or git not installed): %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// branchExists checks whether a local branch exists.
func branchExists(ctx context.Context, dir, branch string) (bool, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--verify", "refs/heads/"+branch)
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// metadataPath returns the path to the metadata JSON file for a sandbox.
func metadataPath(root, sandboxName string) string {
	return filepath.Join(root, worktreeDir, sandboxName+".json")
}

// Create creates a git worktree for the given sandbox and branch.
// If the branch does not exist, it is created from HEAD.
// Returns the absolute path to the worktree directory.
func Create(ctx context.Context, repoDir, sandboxName, branch string) (string, error) {
	root, err := repoRoot(ctx, repoDir)
	if err != nil {
		return "", err
	}

	wtPath := filepath.Join(root, worktreeDir, sandboxName)

	exists, err := branchExists(ctx, root, branch)
	if err != nil {
		return "", fmt.Errorf("checking branch: %w", err)
	}

	var args []string
	if exists {
		args = []string{"worktree", "add", wtPath, branch}
	} else {
		args = []string{"worktree", "add", "-b", branch, wtPath}
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = root
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git worktree add: %w", err)
	}

	// Write metadata
	meta := Metadata{
		SandboxName:  sandboxName,
		WorktreePath: wtPath,
		Branch:       branch,
		RepoRoot:     root,
	}

	metaDir := filepath.Join(root, worktreeDir)
	if err := os.MkdirAll(metaDir, 0o755); err != nil {
		return "", fmt.Errorf("creating metadata dir: %w", err)
	}

	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshaling metadata: %w", err)
	}
	if err := os.WriteFile(metadataPath(root, sandboxName), data, 0o644); err != nil {
		return "", fmt.Errorf("writing metadata: %w", err)
	}

	return wtPath, nil
}

// Remove removes the git worktree and metadata for a sandbox.
// It is a no-op if no metadata exists.
func Remove(ctx context.Context, repoDir, sandboxName string) error {
	root, err := repoRoot(ctx, repoDir)
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

	// Try normal remove, fall back to --force
	cmd := exec.CommandContext(ctx, "git", "worktree", "remove", meta.WorktreePath)
	cmd.Dir = root
	if err := cmd.Run(); err != nil {
		cmd2 := exec.CommandContext(ctx, "git", "worktree", "remove", "--force", meta.WorktreePath)
		cmd2.Dir = root
		if err2 := cmd2.Run(); err2 != nil {
			return fmt.Errorf("git worktree remove --force: %w", err2)
		}
	}

	// Delete metadata file
	if err := os.Remove(metadataPath(root, sandboxName)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("removing metadata: %w", err)
	}

	// Clean up empty worktree dir
	dir := filepath.Join(root, worktreeDir)
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
// current working directory looking for the .sandbox-worktrees directory.
func LookupFromCwd(sandboxName string) (*Metadata, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return LookupFromDir(dir, sandboxName)
}

// LookupFromDir finds metadata for a sandbox by walking up from dir
// looking for the .sandbox-worktrees directory.
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
