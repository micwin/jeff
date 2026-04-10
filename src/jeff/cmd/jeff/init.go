package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"jeff/internal/config"
)

type initOptions struct {
	session     string
	useLast     bool
	codexBinary string
}

func newInitCmd() *cobra.Command {
	var opts initOptions

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Session-ID setzen oder letzte Session wiederverwenden",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := commandContextFrom(cmd)
			if err != nil {
				return err
			}
			return runInit(ctx, opts)
		},
	}

	cmd.Flags().StringVar(&opts.session, "session", "", "Session-ID für Codex")
	cmd.Flags().BoolVar(&opts.useLast, "last-session", false, "Letzte gespeicherte Session reaktivieren")
	cmd.Flags().StringVar(&opts.codexBinary, "codex-binary", "", "Pfad zur Codex-CLI speichern")

	return cmd
}

func runInit(ctx *commandContext, opts initOptions) error {
	cfg, err := ctx.loadConfig()
	if err != nil {
		return err
	}

	session, err := determineSession(ctx, cfg, opts)
	if err != nil {
		return err
	}

	cfg.RecordSession(session)

	if opts.codexBinary != "" {
		cfg.CodexBinary = strings.TrimSpace(opts.codexBinary)
		fmt.Fprintf(ctx.stdout, "Codex-Binary gesetzt auf %s\n", cfg.CodexBinary)
	}

	if err := ctx.saveConfig(cfg); err != nil {
		return err
	}

	fmt.Fprintf(ctx.stdout, "Session %s gespeichert.\n", session)
	return nil
}

func determineSession(ctx *commandContext, cfg *config.Config, opts initOptions) (string, error) {
	switch {
	case opts.useLast:
		if cfg.LastSession == "" {
			return "", errors.New("keine letzte Session vorhanden – bitte --session angeben")
		}
		return cfg.LastSession, nil
	case opts.session != "":
		return strings.TrimSpace(opts.session), nil
	default:
		return promptForSession(ctx.stdin, ctx.stdout)
	}
}

func promptForSession(r io.Reader, w io.Writer) (string, error) {
	fmt.Fprint(w, "Session-ID eingeben: ")
	reader := bufio.NewReader(r)
	value, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("eingabe lesen: %w", err)
	}
	session := strings.TrimSpace(value)
	if session == "" {
		return "", errors.New("Session-ID darf nicht leer sein")
	}
	return session, nil
}
