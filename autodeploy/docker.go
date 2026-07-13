package autodeploy

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const dockerTimeout = 5 * time.Minute

func composeArgs(cfg *Config) []string {
	args := []string{"compose"}
	if cfg.ComposeFile != "" {
		args = append(args, "-f", cfg.ComposeFile)
	}
	return args
}

func runDocker(dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dockerTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()

	if ctx.Err() == context.DeadlineExceeded {
		return strings.TrimSpace(string(out)), fmt.Errorf("docker command timed out after %s", dockerTimeout)
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

// ComposePs reports the status of the stack's containers.
func ComposePs(workdir string, cfg *Config) (string, error) {
	args := composeArgs(cfg)
	args = append(args, "ps")
	return runDocker(workdir, args...)
}
