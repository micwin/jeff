// vl_executor.go selects and runs the Vaultline CLI implementation. It compares
// the embedded backpack binary with a local PATH binary and keeps daemon/store
// lifecycle concerns in vl_runtime.go.
package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"jeff/internal/sidecars"
)

type vaultlineUseMode int

const (
	vaultlineUseAuto vaultlineUseMode = iota
	vaultlineUseBackpack
	vaultlineUseLocal
)

type vaultlineRunner struct {
	source string
	path   string
}

type vaultlineVersion struct {
	major int
	minor int
	patch int
}

var vaultlineVersionPattern = regexp.MustCompile(`^v?([0-9]+)\.([0-9]+)\.([0-9]+)`)

func vaultlineRunnerFor(ctx context.Context, mode vaultlineUseMode) (*vaultlineRunner, error) {
	switch mode {
	case vaultlineUseBackpack:
		if _, ok := sidecars.Lookup("vaultline"); !ok {
			return nil, fmt.Errorf("vaultline sidecar is not embedded in this Jeff build; run scripts/build.sh to build Jeff with sidecars")
		}
		return &vaultlineRunner{source: "backpack"}, nil
	case vaultlineUseLocal:
		path, err := exec.LookPath("vaultline")
		if err != nil {
			return nil, fmt.Errorf("local vaultline binary not found in PATH")
		}
		return &vaultlineRunner{source: "local", path: path}, nil
	default:
		return autoVaultlineRunner(ctx)
	}
}

func autoVaultlineRunner(ctx context.Context) (*vaultlineRunner, error) {
	backpack := &vaultlineRunner{source: "backpack"}
	_, hasBackpack := sidecars.Lookup("vaultline")
	localPath, localErr := exec.LookPath("vaultline")
	hasLocal := localErr == nil
	if !hasBackpack && !hasLocal {
		return nil, fmt.Errorf("vaultline sidecar is not embedded and local vaultline binary was not found")
	}
	if !hasBackpack {
		return &vaultlineRunner{source: "local", path: localPath}, nil
	}
	if !hasLocal {
		return backpack, nil
	}
	local := &vaultlineRunner{source: "local", path: localPath}
	backpackVersion, backpackOK := vaultlineRunnerVersion(ctx, backpack)
	localVersion, localOK := vaultlineRunnerVersion(ctx, local)
	if localOK && (!backpackOK || compareVaultlineVersions(localVersion, backpackVersion) > 0) {
		return local, nil
	}
	return backpack, nil
}

func vaultlineRunnerVersion(ctx context.Context, runner *vaultlineRunner) (vaultlineVersion, bool) {
	out, err := runner.Output(ctx, []string{"version"})
	if err != nil {
		return vaultlineVersion{}, false
	}
	return parseVaultlineVersion(string(out))
}

func parseVaultlineVersion(value string) (vaultlineVersion, bool) {
	line := strings.TrimSpace(strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n")[0])
	match := vaultlineVersionPattern.FindStringSubmatch(line)
	if match == nil {
		return vaultlineVersion{}, false
	}
	major, err := strconv.Atoi(match[1])
	if err != nil {
		return vaultlineVersion{}, false
	}
	minor, err := strconv.Atoi(match[2])
	if err != nil {
		return vaultlineVersion{}, false
	}
	patch, err := strconv.Atoi(match[3])
	if err != nil {
		return vaultlineVersion{}, false
	}
	return vaultlineVersion{major: major, minor: minor, patch: patch}, true
}

func compareVaultlineVersions(left, right vaultlineVersion) int {
	switch {
	case left.major != right.major:
		return left.major - right.major
	case left.minor != right.minor:
		return left.minor - right.minor
	default:
		return left.patch - right.patch
	}
}

func (r *vaultlineRunner) Run(ctx context.Context, args []string, stdio sidecars.Stdio) error {
	if r.source == "local" {
		cmd := exec.CommandContext(ctx, r.path, args...)
		cmd.Stdin = stdio.Stdin
		cmd.Stdout = stdio.Stdout
		cmd.Stderr = stdio.Stderr
		return cmd.Run()
	}
	return sidecars.Run(ctx, "vaultline", args, stdio)
}

func (r *vaultlineRunner) Output(ctx context.Context, args []string) ([]byte, error) {
	if r.source == "local" {
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		cmd := exec.CommandContext(ctx, r.path, args...)
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err := cmd.Run()
		if err != nil {
			if stderr.Len() > 0 {
				return stdout.Bytes(), fmt.Errorf("%w: %s", err, stderr.String())
			}
			return stdout.Bytes(), err
		}
		return stdout.Bytes(), nil
	}
	return sidecars.Output(ctx, "vaultline", args)
}

func vaultlineRunnerNotFound(err error) bool {
	return errors.Is(err, sidecars.ErrNotFound)
}
