package lima

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

const limactlBin = "limactl"

// CreateOpts are options for creating a Lima instance.
type CreateOpts struct {
	Name         string
	TemplateFile string
}

// Create creates a new Lima instance from a template file.
func Create(ctx context.Context, opts CreateOpts) error {
	args := []string{"create", "--name=" + opts.Name, "--tty=false", opts.TemplateFile}
	cmd := exec.CommandContext(ctx, limactlBin, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("limactl create: %w", err)
	}
	return nil
}

// Start starts a Lima instance.
func Start(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, limactlBin, "start", name, "--tty=false")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("limactl start: %w", err)
	}
	return nil
}

// Stop stops a Lima instance.
func Stop(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, limactlBin, "stop", name)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("limactl stop: %w", err)
	}
	return nil
}

// Delete deletes a Lima instance.
func Delete(ctx context.Context, name string, force bool) error {
	args := []string{"delete", name}
	if force {
		args = append(args, "--force")
	}
	cmd := exec.CommandContext(ctx, limactlBin, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("limactl delete: %w", err)
	}
	return nil
}

// List returns all Lima instances.
func List(ctx context.Context) ([]Instance, error) {
	cmd := exec.CommandContext(ctx, limactlBin, "list", "--json")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("limactl list: %w", err)
	}

	if len(out) == 0 {
		return nil, nil
	}

	var instances []Instance
	// limactl list --json outputs one JSON object per line
	dec := json.NewDecoder(bytes.NewReader(out))
	for dec.More() {
		var inst Instance
		if err := dec.Decode(&inst); err != nil {
			return nil, fmt.Errorf("parsing limactl list output: %w", err)
		}
		instances = append(instances, inst)
	}
	return instances, nil
}

// ShellExec replaces the current process with limactl shell (for interactive attach).
func ShellExec(name string, workdir string, args ...string) error {
	bin, err := exec.LookPath(limactlBin)
	if err != nil {
		return fmt.Errorf("finding limactl: %w", err)
	}

	shellArgs := []string{limactlBin, "shell", "--workdir=" + workdir, name}
	shellArgs = append(shellArgs, args...)

	return syscall.Exec(bin, shellArgs, os.Environ())
}

// ShellRun runs a command in a Lima instance and returns its exit code.
func ShellRun(ctx context.Context, name string, workdir string, args ...string) (int, error) {
	shellArgs := []string{"shell", "--workdir=" + workdir, name}
	shellArgs = append(shellArgs, args...)

	cmd := exec.CommandContext(ctx, limactlBin, shellArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode(), nil
		}
		return 1, fmt.Errorf("limactl shell: %w", err)
	}
	return 0, nil
}
