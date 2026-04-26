//go:build linux

package sidecars

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"

	"golang.org/x/sys/unix"
)

func runMemfd(ctx context.Context, tool Tool, args []string, stdio Stdio) error {
	file, err := memfdFile(tool)
	if err != nil {
		return err
	}
	defer file.Close()

	cmd := exec.CommandContext(ctx, "/proc/self/fd/3", args...)
	cmd.ExtraFiles = []*os.File{file}
	cmd.Stdin = stdio.Stdin
	cmd.Stdout = stdio.Stdout
	cmd.Stderr = stdio.Stderr
	return cmd.Run()
}

func outputMemfd(ctx context.Context, tool Tool, args []string) ([]byte, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runMemfd(ctx, tool, args, Stdio{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if err != nil {
		if stderr.Len() > 0 {
			return stdout.Bytes(), fmt.Errorf("%w: %s", err, stderr.String())
		}
		return stdout.Bytes(), err
	}
	return stdout.Bytes(), nil
}

func memfdFile(tool Tool) (*os.File, error) {
	if len(tool.Data) == 0 {
		return nil, fmt.Errorf("sidecar %s is empty", tool.Name)
	}

	fd, err := unix.MemfdCreate("jeff-"+tool.Name, 0)
	if err != nil {
		return nil, fmt.Errorf("memfd_create %s: %w", tool.Name, err)
	}
	file := os.NewFile(uintptr(fd), "jeff-"+tool.Name)
	if file == nil {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("wrap memfd %s", tool.Name)
	}

	if _, err := file.Write(tool.Data); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("write sidecar %s: %w", tool.Name, err)
	}
	if _, err := file.Seek(0, 0); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("rewind sidecar %s: %w", tool.Name, err)
	}
	if err := file.Chmod(0o500); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("chmod sidecar %s: %w", tool.Name, err)
	}

	return file, nil
}
