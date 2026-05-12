//go:build !linux

package sidecars

import (
	"context"
	"errors"
	"os"
)

func runMemfd(ctx context.Context, tool Tool, args []string, stdio Stdio, env []string) error {
	return errors.New("embedded sidecars require Linux memfd_create")
}

func outputMemfd(ctx context.Context, tool Tool, args []string, env []string) ([]byte, error) {
	return nil, errors.New("embedded sidecars require Linux memfd_create")
}

func startMemfd(ctx context.Context, tool Tool, args []string, stdio Stdio, env []string) (*os.Process, error) {
	return nil, errors.New("embedded sidecars require Linux memfd_create")
}
