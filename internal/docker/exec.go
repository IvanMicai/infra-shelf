package docker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// ExecError is returned by Exec / ExecWithStdin when the underlying docker
// process exits non-zero. It preserves the container/command/stderr so callers
// can surface meaningful messages to the user.
type ExecError struct {
	Container string
	Command   []string
	ExitCode  int
	Stderr    string
}

func (e *ExecError) Error() string {
	if e.Stderr != "" {
		return fmt.Sprintf("docker exec %s failed (exit %d): %s",
			e.Container, e.ExitCode, e.Stderr)
	}
	return fmt.Sprintf("docker exec %s failed (exit %d)", e.Container, e.ExitCode)
}

// Exec runs `docker exec <container> <command...>` and returns stdout (trimmed
// of trailing whitespace). Mirrors the behavior of the legacy TS lib's
// dockerExec helper. Use ExecWithStdin when you need to pipe input into the
// container process (e.g. `psql < dump.sql`).
func Exec(ctx context.Context, container string, command ...string) (string, error) {
	return ExecWithStdin(ctx, container, nil, command...)
}

// ExecWithStdin is like Exec but pipes stdin into the docker exec process.
// Returns the trimmed stdout.
func ExecWithStdin(ctx context.Context, container string, stdin io.Reader, command ...string) (string, error) {
	args := append([]string{"exec"}, container)
	if stdin != nil {
		// -i keeps stdin attached so the container process actually receives bytes.
		args = []string{"exec", "-i", container}
	}
	args = append(args, command...)

	cmd := exec.CommandContext(ctx, "docker", args...)
	if stdin != nil {
		cmd.Stdin = stdin
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		return strings.TrimRight(stdout.String(), "\r\n\t "), nil
	}

	exitCode := -1
	var exitErr *exec.ExitError
	if asErr, ok := err.(*exec.ExitError); ok {
		exitErr = asErr
		exitCode = exitErr.ExitCode()
	}
	return strings.TrimRight(stdout.String(), "\r\n\t "), &ExecError{
		Container: container,
		Command:   command,
		ExitCode:  exitCode,
		Stderr:    strings.TrimSpace(stderr.String()),
	}
}

// IsContainerRunning returns true when the named container exists and its
// State.Running is true. Errors (missing container, docker daemon down) return
// false so callers can treat the answer as a simple boolean.
func IsContainerRunning(ctx context.Context, container string) bool {
	cmd := exec.CommandContext(ctx, "docker", "inspect", "--format={{.State.Running}}", container)
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "true"
}
