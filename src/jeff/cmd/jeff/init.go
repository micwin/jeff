package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
)

func runInit(ctx *commandContext, args []string) error {
	flags := flag.NewFlagSet("init", flag.ContinueOnError)
	flags.SetOutput(ctx.stderr)

	sessionFlag := flags.String("session", "", "Session-ID für Codex")
	lastSessionFlag := flags.Bool("last-session", false, "Letzte gespeicherte Session reaktivieren")
	codexBinaryFlag := flags.String("codex-binary", "", "Pfad zur Codex-CLI speichern")

	if err := flags.Parse(args); err != nil {
		return err
	}

	cfg, err := ctx.loadConfig()
	if err != nil {
		return err
	}

	var session string
	switch {
	case *lastSessionFlag:
		if cfg.LastSession == "" {
			return errors.New("keine letzte Session vorhanden – bitte --session angeben")
		}
		session = cfg.LastSession
	case *sessionFlag != "":
		session = strings.TrimSpace(*sessionFlag)
	default:
		fmt.Fprint(ctx.stdout, "Session-ID eingeben: ")
		reader := bufio.NewReader(ctx.stdin)
		value, readErr := reader.ReadString('\n')
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return fmt.Errorf("eingabe lesen: %w", readErr)
		}
		session = strings.TrimSpace(value)
	}

	if session == "" {
		return errors.New("Session-ID darf nicht leer sein")
	}

	cfg.RecordSession(session)

	if *codexBinaryFlag != "" {
		cfg.CodexBinary = strings.TrimSpace(*codexBinaryFlag)
		fmt.Fprintf(ctx.stdout, "Codex-Binary gesetzt auf %s\n", cfg.CodexBinary)
	}

	if err := ctx.saveConfig(cfg); err != nil {
		return err
	}

	fmt.Fprintf(ctx.stdout, "Session %s gespeichert.\n", session)
	return nil
}
