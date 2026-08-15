package main

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"jeff/internal/agent"
)

func newChatCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "chat",
		Short: "Start the interactive Jeff chat",
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

	sessionID := configuredCodexSession(cfg)
	if sessionID == "" {
		return errors.New("no active Jeff session - run 'jeff codex init' first")
	}

	codexBinary := cfg.CodexBinary
	if codexBinary == "" {
		codexBinary = "codex"
	}

	args := codexYoloArgs(
		"--search",
	)
	resumeArgs, markInjected, err := chatResumeArgs(ctx, sessionID)
	if err != nil {
		return err
	}
	args = append(args, resumeArgs...)

	cmd := exec.Command(codexBinary, args...)
	cmd.Stdout = ctx.stdout
	cmd.Stderr = ctx.stderr
	cmd.Stdin = ctx.stdin

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() < 0 {
				fmt.Fprintln(ctx.stdout, "Unterirdisch!")
			} else {
				fmt.Fprintln(ctx.stdout, "Aaaaaaahhhh")
			}
		} else {
			fmt.Fprintln(ctx.stdout, "Unterirdisch!")
		}
		return fmt.Errorf("jeff chat failed: %w", err)
	}
	fmt.Fprintln(ctx.stdout, "jeff sagt tschüß")
	if markInjected {
		if err := agent.MarkPromptInjected(ctx.store, sessionID); err != nil {
			return err
		}
	}

	cfg.RecordSession(strings.TrimSpace(sessionID))
	if err := ctx.saveConfig(cfg); err != nil {
		return err
	}

	return nil
}

func chatResumeArgs(ctx *commandContext, sessionID string) ([]string, bool, error) {
	injected, err := agent.PromptInjected(ctx.store, sessionID)
	if err != nil {
		return nil, false, err
	}
	if injected {
		return codexSessionArgs(sessionID), false, nil
	}
	prompt, err := agent.SystemPrompt(ctx.store)
	if err != nil {
		return nil, false, err
	}
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return codexSessionArgs(sessionID), false, nil
	}
	return codexSessionArgs(sessionID, prompt), true, nil
}
