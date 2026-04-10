package main

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

func newChatCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "chat",
		Short: "Interaktive Codex-Session starten",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := commandContextFrom(cmd)
			if err != nil {
				return err
			}
			return runChat(ctx)
		},
	}
	return cmd
}

func runChat(ctx *commandContext) error {
	cfg, err := ctx.loadConfig()
	if err != nil {
		return err
	}

	sessionID := cfg.ActiveSession
	if sessionID == "" {
		sessionID = cfg.LastSession
	}
	if sessionID == "" {
		return errors.New("keine Session bekannt – bitte zuerst 'jeff init' ausführen")
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
		return fmt.Errorf("codex chat fehlgeschlagen: %w", err)
	}

	cfg.RecordSession(strings.TrimSpace(sessionID))
	if err := ctx.saveConfig(cfg); err != nil {
		return err
	}

	return nil
}
