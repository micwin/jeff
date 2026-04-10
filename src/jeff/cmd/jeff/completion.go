package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"jeff/internal/completion"
)

func runCompletion(ctx *commandContext, args []string) error {
	flags := flag.NewFlagSet("completion", flag.ContinueOnError)
	flags.SetOutput(ctx.stderr)

	dirFlag := flags.String("dir", "", "Zielverzeichnis für Completion-Skripte")
	printFlag := flags.Bool("print", false, "Skript direkt ausgeben statt zu schreiben")

	if err := flags.Parse(args); err != nil {
		return err
	}

	remaining := flags.Args()
	if len(remaining) == 0 {
		return errors.New("bitte Ziel-Shell angeben (bash, zsh oder fish)")
	}
	shell := strings.ToLower(remaining[0])

	script, err := completion.Script(shell)
	if err != nil {
		return err
	}

	if *printFlag {
		fmt.Fprint(ctx.stdout, script)
		return nil
	}

	cfg, err := ctx.loadConfig()
	if err != nil {
		return err
	}

	targetDir := *dirFlag
	if targetDir == "" {
		targetDir = ctx.store.CompletionDir(cfg)
	}

	targetDir, err = filepath.Abs(targetDir)
	if err != nil {
		return fmt.Errorf("verzeichnis auflösen: %w", err)
	}

	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("verzeichnis anlegen: %w", err)
	}

	filename := fmt.Sprintf("jeff.%s", shell)
	targetPath := filepath.Join(targetDir, filename)

	if err := os.WriteFile(targetPath, []byte(script), 0o644); err != nil {
		return fmt.Errorf("skript schreiben: %w", err)
	}

	cfg.CompletionDir = targetDir
	if err := ctx.saveConfig(cfg); err != nil {
		return err
	}

	fmt.Fprintf(ctx.stdout, "Completion für %s gespeichert in %s\n", shell, targetPath)
	return nil
}
