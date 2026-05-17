package runner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)

type Result struct {
	Command  []string
	Output   string
	ExitCode int
}

type CLI struct {
	RootDir string
	BunPath string
	Timeout time.Duration
}

func NewCLI(rootDir, bunPath string) *CLI {
	return &CLI{
		RootDir: rootDir,
		BunPath: bunPath,
		Timeout: 30 * time.Minute,
	}
}

// envs holds the parsed Environment(s) form input: either a single env to
// tag (Single) or a list to expand into siblings (Multi). At most one of
// the two is non-empty.
type EnvSpec struct {
	Single string
	Multi  []string
}

func (e EnvSpec) flags() []string {
	if len(e.Multi) > 0 {
		return []string{"--envs", strings.Join(e.Multi, ",")}
	}
	if e.Single != "" {
		return []string{"--env", e.Single}
	}
	return nil
}

func (c *CLI) Setup(ctx context.Context, appName string, services []string, env EnvSpec) (Result, error) {
	args := []string{"shelf", "setup", appName, "-s", strings.Join(services, ",")}
	args = append(args, env.flags()...)
	return c.run(ctx, args...)
}

func (c *CLI) Add(ctx context.Context, appName string, services []string, env EnvSpec) (Result, error) {
	args := []string{"shelf", "add", appName, "-s", strings.Join(services, ",")}
	args = append(args, env.flags()...)
	return c.run(ctx, args...)
}

func (c *CLI) Remove(ctx context.Context, appName string) (Result, error) {
	return c.run(ctx, "shelf", "remove", appName, "--force")
}

func (c *CLI) Detach(ctx context.Context, appName string, services []string) (Result, error) {
	args := []string{"shelf", "detach", appName, "-s", strings.Join(services, ",")}
	return c.run(ctx, args...)
}

func (c *CLI) Backup(ctx context.Context, appName string, all bool, services []string) (Result, error) {
	args := []string{"shelf", "backup"}
	if all {
		args = append(args, "--all")
	} else {
		args = append(args, appName)
	}
	if len(services) > 0 {
		args = append(args, "-s", strings.Join(services, ","))
	}
	return c.run(ctx, args...)
}

func (c *CLI) Restore(ctx context.Context, appName, filePath string) (Result, error) {
	return c.run(ctx, "shelf", "restore", appName, "--file", filePath, "--force")
}

func (c *CLI) StartInfrastructure(ctx context.Context) (Result, error) {
	return c.runDirect(ctx, "docker", "compose", "--env-file", ".env", "up", "-d")
}

func (c *CLI) run(ctx context.Context, args ...string) (Result, error) {
	return c.runDirect(ctx, c.BunPath, args...)
}

func (c *CLI) runDirect(ctx context.Context, commandName string, args ...string) (Result, error) {
	if c.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.Timeout)
		defer cancel()
	}

	command := append([]string{commandName}, args...)
	cmd := exec.CommandContext(ctx, commandName, args...)
	cmd.Dir = c.RootDir
	cmd.Env = os.Environ()

	output, err := cmd.CombinedOutput()
	result := Result{
		Command:  command,
		Output:   cleanOutput(output),
		ExitCode: 0,
	}

	if err == nil {
		return result, nil
	}

	if ctx.Err() != nil {
		result.ExitCode = -1
		return result, fmt.Errorf("%s timed out: %w", strings.Join(command, " "), ctx.Err())
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
	} else {
		result.ExitCode = -1
	}

	return result, fmt.Errorf("%s failed with exit code %d", strings.Join(command, " "), result.ExitCode)
}

func cleanOutput(output []byte) string {
	text := ansiPattern.ReplaceAllString(string(output), "")
	text = strings.ReplaceAll(text, "\r\n", "\n")
	lines := strings.Split(text, "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "$ ") {
			continue
		}
		if strings.HasPrefix(line, "error: script \"shelf\" exited with code") {
			continue
		}
		cleaned = append(cleaned, line)
	}
	return strings.Join(cleaned, "\n")
}
