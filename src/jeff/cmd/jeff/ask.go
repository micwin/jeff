package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type askOptions struct {
	sessionOverride string
	codexBinary     string
	showTokens      bool
	timeout         time.Duration
	question        string
}

func newAskCmd() *cobra.Command {
	var opts askOptions

	cmd := &cobra.Command{
		Use:   "ask <question>",
		Short: "Send a question to Codex",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := commandContextFrom(cmd)
			if err != nil {
				return err
			}
			opts.question = strings.TrimSpace(strings.Join(args, " "))
			return runAsk(ctx, opts)
		},
	}

	cmd.Flags().StringVar(&opts.sessionOverride, "session", "", "Override the configured session ID")
	cmd.Flags().StringVar(&opts.codexBinary, "codex-binary", "", "Override the Codex CLI path")
	cmd.Flags().BoolVar(&opts.showTokens, "show-token-cost", false, "Print token usage after the answer")
	cmd.Flags().DurationVar(&opts.timeout, "timeout", 45*time.Second, "Set a response timeout (default 45s)")

	return cmd
}

func runAsk(ctx *commandContext, opts askOptions) error {
	if opts.question == "" {
		return errors.New("missing question – e.g. jeff ask \"What is this directory?\"")
	}

	cfg, err := ctx.loadConfig()
	if err != nil {
		return err
	}

	sessionID := strings.TrimSpace(opts.sessionOverride)
	if sessionID == "" {
		if cfg.ActiveSession != "" {
			sessionID = cfg.ActiveSession
		} else {
			sessionID = cfg.LastSession
		}
	}
	if sessionID == "" {
		return errors.New("no active session – run 'jeff init' first")
	}

	codexBinary := strings.TrimSpace(opts.codexBinary)
	if codexBinary == "" {
		codexBinary = cfg.CodexBinary
	}
	if codexBinary == "" {
		codexBinary = "codex"
	}

	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), opts.timeout)
	defer cancel()

	tmpFile, err := os.CreateTemp("", "jeff-codex-response-*.txt")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	cmdArgs := []string{
		"--sandbox", "danger-full-access",
		"--search",
		"exec",
		"--skip-git-repo-check",
		"--output-last-message", tmpPath,
		"resume", sessionID, opts.question,
	}

	var stdoutBuf, stderrBuf strings.Builder
	cmd := exec.CommandContext(ctxWithTimeout, codexBinary, cmdArgs...)
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	if err := cmd.Run(); err != nil {
		return formatCodexError(err, stdoutBuf.String(), stderrBuf.String())
	}

	allLogs := stdoutBuf.String()
	if stderrBuf.Len() > 0 {
		allLogs = allLogs + "\n" + stderrBuf.String()
	}
	statusLines := extractStatusLines(allLogs)

	answerBytes, err := os.ReadFile(tmpPath)
	if err != nil {
		return fmt.Errorf("codex antwort lesen: %w", err)
	}
	answer := strings.TrimSpace(string(answerBytes))
	if answer == "" {
		answer = strings.TrimSpace(stdoutBuf.String())
	}
	if ctx.status && len(statusLines) > 0 {
		for _, line := range statusLines {
			fmt.Fprintln(ctx.stdout, line)
		}
		fmt.Fprintln(ctx.stdout)
	}

	fmt.Fprintln(ctx.stdout, answer)

	if opts.showTokens {
		if usage := extractTokenUsage(stdoutBuf.String()); usage != "" {
			fmt.Fprintf(ctx.stdout, "Token: %s\n", usage)
		}
	}

	cfg.RecordSession(sessionID)
	if err := ctx.saveConfig(cfg); err != nil {
		return err
	}

	return nil
}

func formatCodexError(runErr error, stdout, stderr string) error {
	msg := strings.TrimSpace(stderr)
	if msg == "" {
		msg = strings.TrimSpace(stdout)
	}
	if msg != "" {
		return fmt.Errorf("codex client failed: %w\n%s", runErr, msg)
	}
	return fmt.Errorf("codex client failed: %w", runErr)
}

func extractTokenUsage(output string) string {
	lines := strings.Split(output, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "tokens used") {
			return line
		}
	}
	return ""
}
