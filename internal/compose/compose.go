// Package compose deploys and tears down saved Docker Compose stacks by
// shelling out to the `docker compose` CLI, one working directory per stack.
package compose

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
)

// validNamePattern matches the conservative subset of Compose project names
// we accept from users: this both satisfies Compose's own naming rules and,
// since the name becomes a directory name and a command-line argument,
// rules out path traversal or argument-injection tricks (e.g. "../../etc"
// or a name starting with "-").
var validNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,62}$`)

// ErrInvalidName is returned when a stack name fails validation.
var ErrInvalidName = fmt.Errorf("stack name must match %s", validNamePattern.String())

// ValidateName reports whether name is safe to use as both a Compose
// project name and a filesystem directory name.
func ValidateName(name string) error {
	if !validNamePattern.MatchString(name) {
		return ErrInvalidName
	}
	return nil
}

// Deployer runs `docker compose` against a per-stack working directory
// under baseDir.
type Deployer struct {
	baseDir string
	// run executes a command and returns its combined stdout+stderr; it's a
	// field (rather than calling exec directly) so tests can substitute a
	// fake without invoking a real docker binary.
	run func(ctx context.Context, dir, name string, args ...string) ([]byte, error)
}

// NewDeployer creates a Deployer that writes stack files under baseDir.
func NewDeployer(baseDir string) *Deployer {
	return &Deployer{baseDir: baseDir, run: runCommand}
}

func runCommand(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	return cmd.CombinedOutput()
}

// stackDir returns the working directory for a validated stack name.
func (d *Deployer) stackDir(name string) string {
	return filepath.Join(d.baseDir, name)
}

// composeFileName is always passed to `docker compose -f` as a bare
// filename, never a joined path: the command already runs with cmd.Dir set
// to the stack's directory, and the docker CLI resolves a relative -f path
// against its own working directory at exec time (not ours) — passing the
// full joined path there would resolve twice and double the directory
// prefix (e.g. ".../my-app/my-app/docker-compose.yml").
const composeFileName = "docker-compose.yml"

// Up writes composeContent to the stack's working directory and runs
// `docker compose up -d`. It returns the command's combined output either
// way, so callers can show it to the user even on failure.
func (d *Deployer) Up(ctx context.Context, name, composeContent string) ([]byte, error) {
	if err := ValidateName(name); err != nil {
		return nil, err
	}

	dir := d.stackDir(name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("creating stack directory: %w", err)
	}

	if err := os.WriteFile(filepath.Join(dir, composeFileName), []byte(composeContent), 0o644); err != nil {
		return nil, fmt.Errorf("writing compose file: %w", err)
	}

	out, err := d.run(ctx, dir, "docker", "compose", "-p", name, "-f", composeFileName, "up", "-d", "--remove-orphans")
	if err != nil {
		return out, fmt.Errorf("docker compose up: %w", err)
	}
	return out, nil
}

// Down runs `docker compose down` against a previously deployed stack.
func (d *Deployer) Down(ctx context.Context, name string) ([]byte, error) {
	if err := ValidateName(name); err != nil {
		return nil, err
	}

	dir := d.stackDir(name)
	if _, err := os.Stat(filepath.Join(dir, composeFileName)); err != nil {
		return nil, fmt.Errorf("stack %s has no deployed compose file: %w", name, err)
	}

	out, err := d.run(ctx, dir, "docker", "compose", "-p", name, "-f", composeFileName, "down")
	if err != nil {
		return out, fmt.Errorf("docker compose down: %w", err)
	}
	return out, nil
}
