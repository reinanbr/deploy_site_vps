package autodeploy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	dockerTimeout      = 5 * time.Minute
	dockerQuickTimeout = 30 * time.Second
)

func composeArgs(cfg *Config) []string {
	args := []string{"compose"}
	if cfg.ComposeFile != "" {
		args = append(args, "-f", cfg.ComposeFile)
	}
	return args
}

// runDocker streams the command's combined output to stdout live (so long
// builds show progress instead of appearing to hang) while also buffering it
// to return to the caller for logging.
func runDocker(dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dockerTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = dir

	var buf bytes.Buffer
	tee := io.MultiWriter(os.Stdout, &buf)
	cmd.Stdout = tee
	cmd.Stderr = tee

	err := cmd.Run()
	out := strings.TrimSpace(buf.String())

	if ctx.Err() == context.DeadlineExceeded {
		return out, fmt.Errorf("docker command timed out after %s", dockerTimeout)
	}
	return out, err
}

// runDockerQuiet runs a fast, informational docker command (status checks)
// without teeing to stdout — the caller formats and prints the result itself,
// so streaming it live here would just print it twice.
func runDockerQuiet(dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dockerQuickTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()

	if ctx.Err() == context.DeadlineExceeded {
		return strings.TrimSpace(string(out)), fmt.Errorf("docker command timed out after %s", dockerQuickTimeout)
	}
	return strings.TrimSpace(string(out)), err
}

// DockerAvailable checks that the docker CLI (with the compose plugin) is on PATH.
func DockerAvailable() error {
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("'docker' not found on PATH")
	}
	return nil
}

// ComposeFileExists checks that cfg.ComposeFile resolves to a real file inside workdir.
func ComposeFileExists(workdir string, cfg *Config) error {
	p := cfg.ComposeFile
	if !filepath.IsAbs(p) {
		p = filepath.Join(workdir, p)
	}
	if _, err := os.Stat(p); err != nil {
		return fmt.Errorf("compose file not found: %s", p)
	}
	return nil
}

// ComposeUp builds (if needed) and starts the stack (or a single service) in
// detached mode.
func ComposeUp(workdir string, cfg *Config) (string, error) {
	args := composeArgs(cfg)
	args = append(args, "up", "-d", "--build")
	if cfg.ComposeService != "" {
		args = append(args, cfg.ComposeService)
	}
	return runDocker(workdir, args...)
}

// ComposeRunning reports whether the stack (or the configured service, if
// set) currently has any running containers.
func ComposeRunning(workdir string, cfg *Config) (bool, error) {
	args := composeArgs(cfg)
	args = append(args, "ps", "-q")
	if cfg.ComposeService != "" {
		args = append(args, cfg.ComposeService)
	}
	out, err := runDockerQuiet(workdir, args...)
	if err != nil {
		return false, fmt.Errorf("docker compose ps failed: %w (%s)", err, out)
	}
	return out != "", nil
}

// ComposeDown stops and removes the stack's containers, networks and
// anonymous volumes.
func ComposeDown(workdir string, cfg *Config) (string, error) {
	args := composeArgs(cfg)
	args = append(args, "down")
	return runDocker(workdir, args...)
}

// ComposePs reports the status of the stack's containers.
func ComposePs(workdir string, cfg *Config) (string, error) {
	args := composeArgs(cfg)
	args = append(args, "ps")
	return runDockerQuiet(workdir, args...)
}
