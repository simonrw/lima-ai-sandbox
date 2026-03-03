package githttp

import (
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// initRepo creates a temporary git repo with one commit and returns its path.
func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	for _, args := range [][]string{
		{"init"},
		{"config", "user.email", "test@test.com"},
		{"config", "user.name", "Test"},
		{"config", "receive.denyCurrentBranch", "updateInstead"},
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

func TestCloneViaHTTP(t *testing.T) {
	repo := initRepo(t)

	srv := httptest.NewServer(Handler(repo))
	defer srv.Close()

	cloneDir := filepath.Join(t.TempDir(), "clone")
	cmd := exec.Command("git", "clone", srv.URL, cloneDir)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git clone: %v\n%s", err, out)
	}

	// Verify the clone has the init commit
	cmd = exec.Command("git", "log", "--oneline")
	cmd.Dir = cloneDir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git log: %v", err)
	}
	if !strings.Contains(string(out), "init") {
		t.Errorf("clone log does not contain init commit: %s", out)
	}
}

func TestPushViaHTTP(t *testing.T) {
	repo := initRepo(t)

	srv := httptest.NewServer(Handler(repo))
	defer srv.Close()

	// Clone
	cloneDir := filepath.Join(t.TempDir(), "clone")
	cmd := exec.Command("git", "clone", srv.URL, cloneDir)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git clone: %v\n%s", err, out)
	}

	// Make a commit in the clone
	for _, args := range [][]string{
		{"config", "user.email", "test@test.com"},
		{"config", "user.name", "Test"},
		{"commit", "--allow-empty", "-m", "second commit"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = cloneDir
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	// Push to origin
	cmd = exec.Command("git", "push", "origin", "HEAD")
	cmd.Dir = cloneDir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git push: %v\n%s", err, out)
	}

	// Verify the original repo has the new commit
	cmd = exec.Command("git", "log", "--oneline")
	cmd.Dir = repo
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git log: %v", err)
	}
	if !strings.Contains(string(out), "second commit") {
		t.Errorf("origin log does not contain pushed commit: %s", out)
	}
}

func TestCloneBranchViaHTTP(t *testing.T) {
	repo := initRepo(t)

	// Create a branch in the origin
	cmd := exec.Command("git", "branch", "feature")
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git branch: %v\n%s", err, out)
	}

	srv := httptest.NewServer(Handler(repo))
	defer srv.Close()

	// Clone specific branch
	cloneDir := filepath.Join(t.TempDir(), "clone")
	cmd = exec.Command("git", "clone", "--branch", "feature", srv.URL, cloneDir)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git clone --branch: %v\n%s", err, out)
	}

	// Verify we're on the right branch
	cmd = exec.Command("git", "symbolic-ref", "--short", "HEAD")
	cmd.Dir = cloneDir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git symbolic-ref: %v", err)
	}
	branch := strings.TrimSpace(string(out))
	if branch != "feature" {
		t.Errorf("branch = %q, want %q", branch, "feature")
	}
}
