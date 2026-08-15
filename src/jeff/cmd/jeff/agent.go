package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"jeff/internal/agent"
)

type agentStore interface {
	AgentDir() (string, error)
}

func newAgentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Manage Jeff's persistent personal agent memory",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_ = cmd.Help()
			return errors.New("missing agent subcommand")
		},
	}
	cmd.AddCommand(newAgentStatusCmd(), newAgentRememberCmd(), newAgentInstructionsCmd(), newAgentContactCmd())
	return cmd
}

func newAgentStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Print Jeff agent storage paths",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, err := commandContextFrom(cmd)
			if err != nil {
				return err
			}
			dataDir, err := ctx.store.DataDir()
			if err != nil {
				return err
			}
			cacheDir, err := ctx.store.CacheDir()
			if err != nil {
				return err
			}
			agentDir, err := ctx.store.AgentDir()
			if err != nil {
				return err
			}
			vaultlineDir, err := ctx.store.VaultlineDir()
			if err != nil {
				return err
			}
			fmt.Fprintf(ctx.stdout, "config: %s\n", ctx.store.Dir())
			fmt.Fprintf(ctx.stdout, "data: %s\n", dataDir)
			fmt.Fprintf(ctx.stdout, "cache: %s\n", cacheDir)
			fmt.Fprintf(ctx.stdout, "memcastle: %s\n", agentDir)
			fmt.Fprintf(ctx.stdout, "skills: %s\n", filepath.Join(dataDir, "skills"))
			fmt.Fprintf(ctx.stdout, "commands: %s\n", filepath.Join(dataDir, "commands"))
			fmt.Fprintf(ctx.stdout, "reports: %s\n", filepath.Join(dataDir, "reports"))
			fmt.Fprintf(ctx.stdout, "vaultline: %s\n", vaultlineDir)
			return nil
		},
	}
}

func newAgentRememberCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remember <text>",
		Short: "Append a logbook entry to the memory castle",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := commandContextFrom(cmd)
			if err != nil {
				return err
			}
			path, err := agent.RecordLog(ctx.store, joinArgs(args))
			if err != nil {
				return err
			}
			fmt.Fprintf(ctx.stdout, "Remembered in %s\n", path)
			return nil
		},
	}
}

func newAgentInstructionsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "instructions",
		Short: "Print instructions for agents that need to contact Jeff",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, err := commandContextFrom(cmd)
			if err != nil {
				return err
			}
			agentDir, err := ctx.store.AgentDir()
			if err != nil {
				return err
			}
			fmt.Fprintf(ctx.stdout, `Jeff Agent Contact

Read first:
  %s

Contact Jeff:
  jeff agent contact "Goal: <one line>
Facts:
- <dense facts>
Need: <exact requested output>
Risk: <known constraint>"

Fallback handoffs:
  %s

Rules:
  Use this only for process coordination, handoff structure, conflicting agent
  responsibilities, or where findings belong. Do not include secrets or secret
  values. Do not replace persistent session ids for convenience.
`, filepath.Join(agentDir, "codex", "index.md"), filepath.Join(agentDir, "gatehouse", "specialists"))
			return nil
		},
	}
}

func newAgentContactCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "contact <prompt>",
		Short: "Contact Jeff's narrow coordinator session",
		Long: `Contact Jeff's narrow coordinator session.

The prompt may be passed as arguments or piped on stdin. If direct contact is
blocked by a restricted specialist sandbox, Jeff writes a dated handoff file
below the memory castle gatehouse and prints that path.`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := commandContextFrom(cmd)
			if err != nil {
				return err
			}
			prompt, err := agentContactPrompt(ctx.stdin, args)
			if err != nil {
				return err
			}
			answer, err := runJeffCoordinator(prompt)
			if err == nil {
				fmt.Fprintln(ctx.stdout, answer)
				return nil
			}
			handoff, writeErr := writeAgentContactFallback(ctx.store, prompt, err.Error())
			if writeErr != nil {
				return fmt.Errorf("contact Jeff failed: %v; write fallback: %w", err, writeErr)
			}
			return fmt.Errorf("contact Jeff failed; wrote handoff: %s", handoff)
		},
	}
}

func agentContactPrompt(stdinReader io.Reader, args []string) (string, error) {
	prompt := joinArgs(args)
	if prompt == "" {
		var stdin bytes.Buffer
		if _, err := stdin.ReadFrom(stdinReader); err != nil {
			return "", fmt.Errorf("read stdin: %w", err)
		}
		prompt = strings.TrimSpace(stdin.String())
	}
	if prompt == "" {
		return "", errors.New("missing contact prompt")
	}
	return prompt, nil
}

func runJeffCoordinator(prompt string) (string, error) {
	command := exec.Command("codex-ctl", "exec", "jeff-coordinator", "--", prompt)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("%s", msg)
	}
	return strings.TrimSpace(stdout.String()), nil
}

func writeAgentContactFallback(store agentStore, prompt string, errorText string) (string, error) {
	agentDir, err := store.AgentDir()
	if err != nil {
		return "", err
	}
	handoffDir := filepath.Join(agentDir, "gatehouse", "specialists")
	if err := os.MkdirAll(handoffDir, 0o755); err != nil {
		return "", err
	}
	now := time.Now()
	path := filepath.Join(handoffDir, now.Format("20060102-150405")+"-jeff-contact-fallback.md")
	content := fmt.Sprintf(`# Jeff Contact Fallback

Created: %s

Direct coordinator call failed.

## Error

`+"```text"+`
%s
`+"```"+`

## Prompt

`+"```text"+`
%s
`+"```"+`
`, now.Format(time.RFC3339), strings.TrimSpace(errorText), prompt)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", err
	}
	return path, nil
}
