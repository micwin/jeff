package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"jeff/internal/agent"
)

type initOptions struct {
	session     string
	useLast     bool
	codexBinary string
	force       bool
}

func newCodexInitCmd() *cobra.Command {
	var opts initOptions

	cmd := &cobra.Command{
		Use:   "init [sessionid]",
		Short: "Initialize Jeff's single Codex agent session",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := commandContextFrom(cmd)
			if err != nil {
				return err
			}
			if len(args) > 0 {
				opts.session = args[0]
			}
			return runInit(ctx, opts)
		},
	}

	cmd.Flags().StringVar(&opts.session, "session", "", "Explicit Codex session ID (deprecated; prefer positional sessionid)")
	cmd.Flags().BoolVar(&opts.useLast, "last", false, "Use the most recent Codex session")
	cmd.Flags().StringVar(&opts.codexBinary, "codex-binary", "", "Persist a custom Codex CLI path")
	cmd.Flags().BoolVar(&opts.force, "force", false, "Overwrite an existing Jeff Codex initialization")

	return cmd
}

func runInit(ctx *commandContext, opts initOptions) error {
	cfg, err := ctx.loadConfig()
	if err != nil {
		return err
	}
	if cfg.Codex.Initialized && cfg.Codex.SessionID != "" && !opts.force {
		return errors.New("jeff codex is already initialized – use --force to overwrite")
	}

	session, err := determineSession(ctx, opts)
	if err != nil {
		return err
	}

	if _, err := agent.Bootstrap(ctx.store, false); err != nil {
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

	fmt.Fprintf(ctx.stdout, "Jeff Codex session %s initialized.\n", session)
	return nil
}

func determineSession(ctx *commandContext, opts initOptions) (string, error) {
	switch {
	case opts.useLast:
		return resolveLastCodexSessionID()
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
