package sidecars

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

type Stdio struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

type Tool struct {
	Name string
	Data []byte
}

var ErrNotFound = errors.New("sidecar not found")

func Lookup(name string) (Tool, bool) {
	tool, ok := embeddedTools[name]
	return tool, ok
}

func Run(ctx context.Context, name string, args []string, stdio Stdio) error {
	return RunWithEnv(ctx, name, args, stdio, nil)
}

func RunWithEnv(ctx context.Context, name string, args []string, stdio Stdio, env []string) error {
	tool, ok := Lookup(name)
	if !ok {
		return fmt.Errorf("%w: %s", ErrNotFound, name)
	}
	return runMemfd(ctx, tool, args, stdio, env)
}

func Output(ctx context.Context, name string, args []string) ([]byte, error) {
	return OutputWithEnv(ctx, name, args, nil)
}

func OutputWithEnv(ctx context.Context, name string, args []string, env []string) ([]byte, error) {
	tool, ok := Lookup(name)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, name)
	}
	return outputMemfd(ctx, tool, args, env)
}

func Start(ctx context.Context, name string, args []string, stdio Stdio, env []string) (*os.Process, error) {
	tool, ok := Lookup(name)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, name)
	}
	return startMemfd(ctx, tool, args, stdio, env)
}

func IsExecutableError(err error) bool {
	var exitErr *exec.ExitError
	return errors.As(err, &exitErr)
}

func ParseLineCompletions(out []byte, prefix string) []string {
	lines := strings.Split(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n")
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		if prefix == "" || strings.HasPrefix(line, prefix) {
			result = append(result, line)
		}
	}
	return result
}
