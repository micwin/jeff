//go:build !linux

package sidecars

import (
	"context"
	"errors"
)

func runMemfd(ctx context.Context, tool Tool, args []string, stdio Stdio) error {
	return errors.New("embedded sidecars require Linux memfd_create")
}

func outputMemfd(ctx context.Context, tool Tool, args []string) ([]byte, error) {
	return nil, errors.New("embedded sidecars require Linux memfd_create")
}
