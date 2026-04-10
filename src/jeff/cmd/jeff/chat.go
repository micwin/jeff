package main

import (
	"errors"
	"flag"
	"fmt"
	"os/exec"
	"strings"
)

func runChat(ctx *commandContext, argv []string) error {
	flags := flag.NewFlagSet("chat", flag.ContinueOnError)
	flags.SetOutput(ctx.stderr)

	if err := flags.Parse(argv); err != nil {
		return err
	}
	if flags.NArg() > 0 {
		return errors.New("chat benötigt keine zusätzlichen Argumente")
	}

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
