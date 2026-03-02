package worktree

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// initRepo creates a temporary bare-bones git repo with one commit and returns its path.
func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	for _, args := range [][]string{
		{"init"},
		{"config", "user.email", "test@test.com"},
		{"config", "user.name", "Test"},
		{"commit", "--allow-empty", "-m", "init"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return dir
}

func TestCreateNewBranch(t *testing.T) {
	repo := initRepo(t)
	ctx := context.Background()

	wtPath, err := Create(ctx, repo, "sandbox-test1", "feature-new")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Worktree directory should exist
	if _, err := os.Stat(wtPath); err != nil {
		t.Fatalf("worktree dir not found: %v", err)
	}

	// Should be inside .sandbox-worktrees
	expected := filepath.Join(repo, worktreeDir, "sandbox-test1")
	if wtPath != expected {
		t.Errorf("worktree path = %q, want %q", wtPath, expected)
	}
}

func TestCreateExistingBranch(t *testing.T) {
	repo := initRepo(t)
	ctx := context.Background()

	// Create a branch first
	cmd := exec.Command("git", "branch", "existing-branch")
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git branch: %v\n%s", err, out)
	}

	wtPath, err := Create(ctx, repo, "sandbox-test2", "existing-branch")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if _, err := os.Stat(wtPath); err != nil {
		t.Fatalf("worktree dir not found: %v", err)
	}
}

func TestMetadataWrittenAndReadable(t *testing.T) {
	repo := initRepo(t)
	ctx := context.Background()

	_, err := Create(ctx, repo, "sandbox-meta", "branch-meta")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	meta, err := Lookup(repo, "sandbox-meta")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if meta == nil {
		t.Fatal("Lookup returned nil")
	}
	if meta.SandboxName != "sandbox-meta" {
		t.Errorf("SandboxName = %q, want %q", meta.SandboxName, "sandbox-meta")
	}
	if meta.Branch != "branch-meta" {
		t.Errorf("Branch = %q, want %q", meta.Branch, "branch-meta")
	}
	if meta.RepoRoot != repo {
		t.Errorf("RepoRoot = %q, want %q", meta.RepoRoot, repo)
	}

	// Also verify the JSON file is valid
	data, err := os.ReadFile(metadataPath(repo, "sandbox-meta"))
	if err != nil {
		t.Fatalf("reading metadata file: %v", err)
	}
	var raw Metadata
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("parsing metadata JSON: %v", err)
	}
}

func TestRemoveCleansUp(t *testing.T) {
	repo := initRepo(t)
	ctx := context.Background()

	wtPath, err := Create(ctx, repo, "sandbox-rm", "branch-rm")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := Remove(ctx, repo, "sandbox-rm"); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	// Worktree dir should be gone
	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Errorf("worktree dir still exists after Remove")
	}

	// Metadata file should be gone
	if _, err := os.Stat(metadataPath(repo, "sandbox-rm")); !os.IsNotExist(err) {
		t.Errorf("metadata file still exists after Remove")
	}
}

func TestRemoveNoopWhenNoMetadata(t *testing.T) {
	repo := initRepo(t)
	ctx := context.Background()

	// Should not error when there's no metadata
	if err := Remove(ctx, repo, "sandbox-nonexistent"); err != nil {
		t.Fatalf("Remove with no metadata: %v", err)
	}
}

func TestLookupFromDir(t *testing.T) {
	repo := initRepo(t)
	ctx := context.Background()

	_, err := Create(ctx, repo, "sandbox-walk", "branch-walk")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Create a nested directory inside the repo
	nested := filepath.Join(repo, "a", "b", "c")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// LookupFromDir should find metadata by walking up
	meta, err := LookupFromDir(nested, "sandbox-walk")
	if err != nil {
		t.Fatalf("LookupFromDir: %v", err)
	}
	if meta == nil {
		t.Fatal("LookupFromDir returned nil")
	}
	if meta.Branch != "branch-walk" {
		t.Errorf("Branch = %q, want %q", meta.Branch, "branch-walk")
	}
}

func TestLookupFromDirNotFound(t *testing.T) {
	dir := t.TempDir()

	meta, err := LookupFromDir(dir, "sandbox-nope")
	if err != nil {
		t.Fatalf("LookupFromDir: %v", err)
	}
	if meta != nil {
		t.Errorf("expected nil metadata, got %+v", meta)
	}
}

func TestCreateErrorNotGitRepo(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	_, err := Create(ctx, dir, "sandbox-err", "branch-err")
	if err == nil {
		t.Fatal("expected error for non-git dir")
	}
}
