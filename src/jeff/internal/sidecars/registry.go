package sidecars

import (
	"context"
	"errors"
	"fmt"
	"io"
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
	tool, ok := Lookup(name)
	if !ok {
		return fmt.Errorf("%w: %s", ErrNotFound, name)
	}
	return runMemfd(ctx, tool, args, stdio)
}

func Output(ctx context.Context, name string, args []string) ([]byte, error) {
	tool, ok := Lookup(name)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, name)
	}
	return outputMemfd(ctx, tool, args)
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
