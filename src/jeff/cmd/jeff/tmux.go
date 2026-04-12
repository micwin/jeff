package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"jeff/internal/config"
)

func newTmuxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tmux",
		Short: "Launch the Jeff tmux overlay or configure its status bar",
		Long: `Manage or launch Jeff's tmux-based overlay.

Common tmux keys once running:
  Ctrl-b d    detach session (Jeff tmux keeps running)
  Ctrl-b c    new window
  Ctrl-b ,    rename window
  exit        leave the session entirely`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := commandContextFrom(cmd)
			if err != nil {
				return err
			}
			return runTmuxOverlay(ctx)
		},
	}

	cmd.AddCommand(
		newTmuxSetStatusCmd("left"),
		newTmuxSetStatusCmd("center"),
		newTmuxSetStatusCmd("right"),
		newTmuxSetLayoutCmd(),
		newTmuxSetIntervalCmd(),
	)

	return cmd
}

func newTmuxSetStatusCmd(region string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   fmt.Sprintf("set-status-%s <command>", region),
		Short: fmt.Sprintf("Set the %s status command", region),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := commandContextFrom(cmd)
			if err != nil {
				return err
			}

			useDefault, _ := cmd.Flags().GetBool("default")
			if useDefault {
				return resetStatusToDefault(ctx, region)
			}

			if len(args) == 0 {
				return errors.New("command is required (or use --default)")
			}

			raw := strings.Join(args, " ")
			payload := normalizeTemplateInput(raw)
			if payload == "" {
				return errors.New("command must not be empty")
			}

			// validate command before saving
			if _, err := renderTemplateWithShell(payload); err != nil {
				return fmt.Errorf("command validation failed: %w", err)
			}

			cfg, err := ctx.loadConfig()
			if err != nil {
				return err
			}

			switch region {
			case "left":
				cfg.ShellStatus.Left.Command = payload
			case "center":
				cfg.ShellStatus.Center.Command = payload
			case "right":
				cfg.ShellStatus.Right.Command = payload
			default:
				return fmt.Errorf("unsupported region %q", region)
			}

			if err := ctx.saveConfig(cfg); err != nil {
				return err
			}

			fmt.Fprintf(ctx.stdout, "%s status command set to %q\n", strings.ToUpper(region[:1])+region[1:], payload)
			return nil
		},
	}
	cmd.Flags().Bool("default", false, "Reset to the built-in default")
	return cmd
}

func resetStatusToDefault(ctx *commandContext, region string) error {
	cfg, err := ctx.loadConfig()
	if err != nil {
		return err
	}
	defaults := config.DefaultShellStatusConfig()
	var defaultValue string
	switch region {
	case "left":
		cfg.ShellStatus.Left.Command = ""
		defaultValue = defaults.Left.Command
	case "center":
		cfg.ShellStatus.Center.Command = ""
		defaultValue = defaults.Center.Command
	case "right":
		cfg.ShellStatus.Right.Command = ""
		defaultValue = defaults.Right.Command
	default:
		return fmt.Errorf("unsupported region %q", region)
	}
	if err := ctx.saveConfig(cfg); err != nil {
		return err
	}
	fmt.Fprintf(ctx.stdout, "%s status command reset to default (%s)\n", strings.ToUpper(region[:1])+region[1:], defaultValue)
	return nil
}

func newTmuxSetLayoutCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set-layout <left>:<center>:<right>",
		Short: "Set the proportional layout for the status segments",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := commandContextFrom(cmd)
			if err != nil {
				return err
			}
			left, center, right, err := parseTriple(args[0])
			if err != nil {
				return err
			}
			cfg, err := ctx.loadConfig()
			if err != nil {
				return err
			}
			cfg.ShellStatus.Layout.Left = left
			cfg.ShellStatus.Layout.Center = center
			cfg.ShellStatus.Layout.Right = right
			if err := ctx.saveConfig(cfg); err != nil {
				return err
			}
			fmt.Fprintf(ctx.stdout, "Layout set to %d:%d:%d\n", left, center, right)
			return nil
		},
	}
	return cmd
}

func newTmuxSetIntervalCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set-interval <left>:<center>:<right>",
		Short: "Set refresh intervals (seconds) for each status segment",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := commandContextFrom(cmd)
			if err != nil {
				return err
			}
			left, center, right, err := parseTriple(args[0])
			if err != nil {
				return err
			}
			cfg, err := ctx.loadConfig()
			if err != nil {
				return err
			}
			cfg.ShellStatus.Left.Interval = left
			cfg.ShellStatus.Center.Interval = center
			cfg.ShellStatus.Right.Interval = right
			if err := ctx.saveConfig(cfg); err != nil {
				return err
			}
			fmt.Fprintf(ctx.stdout, "Intervals set to %d:%d:%d seconds\n", left, center, right)
			return nil
		},
	}
	return cmd
}

func parseTriple(input string) (int, int, int, error) {
	parts := strings.Split(input, ":")
	if len(parts) != 3 {
		return 0, 0, 0, errors.New("expected format left:center:right")
	}

	var values [3]int
	for i, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return 0, 0, 0, errors.New("ratios must be positive integers")
		}
		val, err := strconv.Atoi(part)
		if err != nil || val <= 0 {
			return 0, 0, 0, errors.New("ratios must be positive integers")
		}
		values[i] = val
	}

	return values[0], values[1], values[2], nil
}

func normalizeTemplateInput(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if len(trimmed) >= 2 {
		if (trimmed[0] == '\'' && trimmed[len(trimmed)-1] == '\'') ||
			(trimmed[0] == '"' && trimmed[len(trimmed)-1] == '"') {
			return trimmed[1 : len(trimmed)-1]
		}
	}
	return trimmed
}
