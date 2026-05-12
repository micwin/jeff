package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

var commandNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

func newExecuteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "execute <command> [args...]",
		Short:             "Execute a stored Jeff Bash command",
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: completeStoredCommandNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := commandContextFrom(cmd)
			if err != nil {
				return err
			}
			return runStoredCommand(ctx, args[0], args[1:])
		},
	}
	return cmd
}

func completeStoredCommandNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	ctx, err := commandContextFrom(cmd)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	names, err := listStoredCommandNames(ctx)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	filtered := make([]string, 0, len(names))
	for _, name := range names {
		if strings.HasPrefix(name, toComplete) {
			filtered = append(filtered, name)
		}
	}
	return filtered, cobra.ShellCompDirectiveNoFileComp
}

func listStoredCommandNames(ctx *commandContext) ([]string, error) {
	root, err := ctx.store.CommandsDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == "shared" {
			continue
		}
		runPath := filepath.Join(root, name, "run.sh")
		if fileExists(runPath) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names, nil
}

func runStoredCommand(ctx *commandContext, name string, args []string) error {
	if !commandNamePattern.MatchString(name) {
		return fmt.Errorf("invalid command name %q", name)
	}
	root, err := ctx.store.CommandsDir()
	if err != nil {
		return err
	}
	cmdDir := filepath.Join(root, name)
	runPath := filepath.Join(cmdDir, "run.sh")
	if _, err := os.Stat(runPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("command %q has no run.sh", name)
		}
		return err
	}

	script, err := buildExecuteScript(root, cmdDir)
	if err != nil {
		return err
	}
	command := exec.Command("bash", append([]string{"-c", script, "jeff-execute-" + name}, args...)...)
	command.Dir = cmdDir
	command.Stdin = ctx.stdin
	command.Stdout = ctx.stdout
	command.Stderr = ctx.stderr
	return command.Run()
}

func buildExecuteScript(root, cmdDir string) (string, error) {
	sharedDir := filepath.Join(root, "shared")
	shared, err := filepath.Glob(filepath.Join(sharedDir, "*.sh"))
	if err != nil {
		return "", err
	}
	sort.Strings(shared)

	var lines []string
	lines = append(lines, "set -euo pipefail")
	for _, path := range shared {
		lines = append(lines, "source "+shellQuote(path))
	}
	initPath := filepath.Join(cmdDir, "init.sh")
	cleanupPath := filepath.Join(cmdDir, "cleanup.sh")
	runPath := filepath.Join(cmdDir, "run.sh")
	if fileExists(initPath) {
		lines = append(lines, "source "+shellQuote(initPath))
	}
	if fileExists(cleanupPath) {
		lines = append(lines, "jeff_execute_cleanup() { source "+shellQuote(cleanupPath)+"; }")
		lines = append(lines, "trap jeff_execute_cleanup EXIT")
	}
	lines = append(lines, "source "+shellQuote(runPath)+" \"$@\"")
	return strings.Join(lines, "\n"), nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
