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
		{"config", "user.name", "Test User"},
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

func TestRepoRoot(t *testing.T) {
	repo := initRepo(t)
	ctx := context.Background()

	root, err := RepoRoot(ctx, repo)
	if err != nil {
		t.Fatalf("RepoRoot: %v", err)
	}

	// Resolve symlinks for comparison (t.TempDir may use /tmp which is a symlink)
	expected, _ := filepath.EvalSymlinks(repo)
	got, _ := filepath.EvalSymlinks(root)
	if got != expected {
		t.Errorf("RepoRoot = %q, want %q", got, expected)
	}
}

func TestRepoRootNotGitRepo(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	_, err := RepoRoot(ctx, dir)
	if err == nil {
		t.Fatal("expected error for non-git dir")
	}
}

func TestGitUserConfig(t *testing.T) {
	repo := initRepo(t)
	ctx := context.Background()

	name, email := GitUserConfig(ctx, repo)
	if name != "Test User" {
		t.Errorf("GitUserConfig name = %q, want %q", name, "Test User")
	}
	if email != "test@test.com" {
		t.Errorf("GitUserConfig email = %q, want %q", email, "test@test.com")
	}
}

func TestCreateSavesMetadata(t *testing.T) {
	repo := initRepo(t)
	ctx := context.Background()

	root, err := Create(ctx, repo, "sandbox-meta", "feature-x", 12345, 8080)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Verify returned root
	expected, _ := filepath.EvalSymlinks(repo)
	got, _ := filepath.EvalSymlinks(root)
	if got != expected {
		t.Errorf("Create returned root = %q, want %q", got, expected)
	}

	// Verify metadata file
	meta, err := Lookup(root, "sandbox-meta")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if meta == nil {
		t.Fatal("Lookup returned nil")
	}
	if meta.SandboxName != "sandbox-meta" {
		t.Errorf("SandboxName = %q, want %q", meta.SandboxName, "sandbox-meta")
	}
	if meta.Branch != "feature-x" {
		t.Errorf("Branch = %q, want %q", meta.Branch, "feature-x")
	}
	if meta.ServerPID != 12345 {
		t.Errorf("ServerPID = %d, want %d", meta.ServerPID, 12345)
	}
	if meta.ServerPort != 8080 {
		t.Errorf("ServerPort = %d, want %d", meta.ServerPort, 8080)
	}

	// Verify JSON is valid
	data, err := os.ReadFile(metadataPath(root, "sandbox-meta"))
	if err != nil {
		t.Fatalf("reading metadata file: %v", err)
	}
	var raw Metadata
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("parsing metadata JSON: %v", err)
	}
	if raw.ServerPID != 12345 {
		t.Errorf("JSON ServerPID = %d, want %d", raw.ServerPID, 12345)
	}
}

func TestRemoveCleansUpMetadata(t *testing.T) {
	repo := initRepo(t)
	ctx := context.Background()

	root, err := Create(ctx, repo, "sandbox-rm", "branch-rm", 0, 0)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := Remove(ctx, repo, "sandbox-rm"); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	// Metadata file should be gone
	if _, err := os.Stat(metadataPath(root, "sandbox-rm")); !os.IsNotExist(err) {
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

	if _, err := Create(ctx, repo, "sandbox-walk", "branch-walk", 99, 9999); err != nil {
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
	if meta.ServerPort != 9999 {
		t.Errorf("ServerPort = %d, want %d", meta.ServerPort, 9999)
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

	_, err := Create(ctx, dir, "sandbox-err", "branch-err", 0, 0)
	if err == nil {
		t.Fatal("expected error for non-git dir")
	}
}
