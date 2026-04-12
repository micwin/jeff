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

func newCodexInitCmd() *cobra.Command {
	var opts initOptions

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Configure or reuse a Codex session ID",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := commandContextFrom(cmd)
			if err != nil {
				return err
			}
			return runInit(ctx, opts)
		},
	}

	cmd.Flags().StringVar(&opts.session, "session", "", "Explicit Codex session ID")
	cmd.Flags().BoolVar(&opts.useLast, "last-session", false, "Reuse the latest saved session ID")
	cmd.Flags().StringVar(&opts.codexBinary, "codex-binary", "", "Persist a custom Codex CLI path")

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
		fmt.Fprintf(ctx.stdout, "Codex binary set to %s\n", cfg.CodexBinary)
	}

	if err := ctx.saveConfig(cfg); err != nil {
		return err
	}

	fmt.Fprintf(ctx.stdout, "Session %s saved.\n", session)
	return nil
}

func determineSession(ctx *commandContext, cfg *config.Config, opts initOptions) (string, error) {
	switch {
	case opts.useLast:
		if cfg.LastSession == "" {
			return "", errors.New("no previous session found – provide --session")
		}
		return cfg.LastSession, nil
	case opts.session != "":
		return strings.TrimSpace(opts.session), nil
	default:
		return promptForSession(ctx.stdin, ctx.stdout)
	}
}

func promptForSession(r io.Reader, w io.Writer) (string, error) {
	fmt.Fprint(w, "Enter session ID: ")
	reader := bufio.NewReader(r)
	value, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("read input: %w", err)
	}
	session := strings.TrimSpace(value)
	if session == "" {
		return "", errors.New("session ID must not be empty")
	}
	return session, nil
}
