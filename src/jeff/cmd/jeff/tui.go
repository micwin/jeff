package main

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

func newCodexTuiCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tui",
		Short: "Start an interactive Codex TUI session",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := commandContextFrom(cmd)
			if err != nil {
				return err
			}
			return runTUI(ctx)
		},
	}
	return cmd
}

func runTUI(ctx *commandContext) error {
	cfg, err := ctx.loadConfig()
	if err != nil {
		return err
	}

	sessionID := cfg.ActiveSession
	if sessionID == "" {
		sessionID = cfg.LastSession
	}
	if sessionID == "" {
		return errors.New("no active session – run 'jeff codex init' first")
	}

	codexBinary := cfg.CodexBinary
	if codexBinary == "" {
		codexBinary = "codex"
	}

	args := []string{
		"--sandbox", "danger-full-access",
		"--search",
		"resume", sessionID,
	}

	cmd := exec.Command(codexBinary, args...)
	cmd.Stdout = ctx.stdout
	cmd.Stderr = ctx.stderr
	cmd.Stdin = ctx.stdin

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("codex tui failed: %w", err)
	}

	cfg.RecordSession(strings.TrimSpace(sessionID))
	if err := ctx.saveConfig(cfg); err != nil {
		return err
	}

	return nil
}
