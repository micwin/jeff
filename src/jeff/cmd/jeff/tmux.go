package main

import (
	"errors"
	"fmt"
	"os"
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
		newTmuxRestartCmd(),
		newTmuxKillCmd(),
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

func newMenuCmd() *cobra.Command {
	menuCmd := &cobra.Command{
		Use:   "menu",
		Short: "Manage Jeff popup menu entries",
	}

	menuCmd.AddCommand(
		newMenuAddCmd(),
		newMenuDeleteCmd(),
		newMenuRenameCmd(),
		newMenuMoveCmd(),
		newMenuCleanCmd(),
		newMenuListCmd(),
		newMenuTuiCmd(),
		newMenuShowCmd(),
	)

	return menuCmd
}

func newMenuAddCmd() *cobra.Command {
	var label string
	var commandStr string
	var customID string
	var insertIndex int

	cmd := &cobra.Command{
		Use:   "add-entry",
		Short: "Add a tmux popup menu entry",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := commandContextFrom(cmd)
			if err != nil {
				return err
			}
			label = strings.TrimSpace(label)
			commandStr = strings.TrimSpace(commandStr)
			if commandStr == "" {
				return errors.New("--command is required")
			}
			if label == "" {
				label = commandStr
			}

			cfg, err := ctx.loadConfig()
			if err != nil {
				return err
			}
			entryID := customID
			if entryID == "" {
				entryID = generateMenuID(label, cfg)
			}
			for _, entry := range cfg.TmuxMenu {
				if entry.ID == entryID {
					return fmt.Errorf("menu entry id %q already exists", entryID)
				}
			}

			entry := config.TmuxMenuEntry{
				ID:      entryID,
				Label:   label,
				Command: commandStr,
			}
			cfg.TmuxMenu = append(cfg.TmuxMenu, entry)

			if insertIndex > 0 {
				if insertIndex > len(cfg.TmuxMenu) {
					insertIndex = len(cfg.TmuxMenu)
				}
				targetIdx := insertIndex - 1
				if targetIdx < len(cfg.TmuxMenu)-1 {
					copy(cfg.TmuxMenu[targetIdx+1:], cfg.TmuxMenu[targetIdx:len(cfg.TmuxMenu)-1])
				}
				cfg.TmuxMenu[targetIdx] = entry
			}

			if err := ctx.saveConfig(cfg); err != nil {
				return err
			}
			position := len(cfg.TmuxMenu)
			if insertIndex > 0 {
				position = insertIndex
			}
			fmt.Fprintf(ctx.stdout, "Added menu entry %s (%s) at position %d\n", entryID, label, position)
			return nil
		},
	}

	cmd.Flags().StringVar(&label, "label", "", "Menu label (defaults to command)")
	cmd.Flags().StringVar(&commandStr, "command", "", "Command to execute")
	cmd.Flags().StringVar(&customID, "id", "", "Optional entry id")
	cmd.Flags().IntVar(&insertIndex, "index", 0, "1-based position for the new entry (defaults to append)")

	return cmd
}

func newMenuDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete-entry <id>",
		Short: "Delete a tmux menu entry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := commandContextFrom(cmd)
			if err != nil {
				return err
			}
			entryID := args[0]
			cfg, err := ctx.loadConfig()
			if err != nil {
				return err
			}
			index := -1
			for i, entry := range cfg.TmuxMenu {
				if entry.ID == entryID {
					index = i
					break
				}
			}
			if index == -1 {
				return fmt.Errorf("no menu entry with id %q", entryID)
			}
			cfg.TmuxMenu = append(cfg.TmuxMenu[:index], cfg.TmuxMenu[index+1:]...)
			if err := ctx.saveConfig(cfg); err != nil {
				return err
			}
			fmt.Fprintf(ctx.stdout, "Deleted menu entry %s\n", entryID)
			return nil
		},
	}
	return cmd
}

func newMenuRenameCmd() *cobra.Command {
	var newLabel string
	cmd := &cobra.Command{
		Use:   "rename-entry <id>",
		Short: "Rename a menu entry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := commandContextFrom(cmd)
			if err != nil {
				return err
			}
			newLabel = strings.TrimSpace(newLabel)
			if newLabel == "" {
				return errors.New("--label is required")
			}
			cfg, err := ctx.loadConfig()
			if err != nil {
				return err
			}
			found := false
			for i := range cfg.TmuxMenu {
				if cfg.TmuxMenu[i].ID == args[0] {
					cfg.TmuxMenu[i].Label = newLabel
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("no menu entry %q", args[0])
			}
			if err := ctx.saveConfig(cfg); err != nil {
				return err
			}
			fmt.Fprintf(ctx.stdout, "Menu entry %s renamed to %s\n", args[0], newLabel)
			return nil
		},
	}
	cmd.Flags().StringVar(&newLabel, "label", "", "New label")
	return cmd
}

func newMenuMoveCmd() *cobra.Command {
	var position int
	cmd := &cobra.Command{
		Use:   "move-entry <id>",
		Short: "Move a menu entry to a new position (1-based index)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := commandContextFrom(cmd)
			if err != nil {
				return err
			}
			if position <= 0 {
				return errors.New("--position must be >= 1")
			}
			cfg, err := ctx.loadConfig()
			if err != nil {
				return err
			}
			idx := -1
			for i, entry := range cfg.TmuxMenu {
				if entry.ID == args[0] {
					idx = i
					break
				}
			}
			if idx == -1 {
				return fmt.Errorf("no menu entry %q", args[0])
			}
			if position > len(cfg.TmuxMenu) {
				position = len(cfg.TmuxMenu)
			}
			entry := cfg.TmuxMenu[idx]
			cfg.TmuxMenu = append(cfg.TmuxMenu[:idx], cfg.TmuxMenu[idx+1:]...)
			newIdx := position - 1
			if newIdx < 0 {
				newIdx = 0
			}
			if newIdx >= len(cfg.TmuxMenu) {
				cfg.TmuxMenu = append(cfg.TmuxMenu, entry)
			} else {
				cfg.TmuxMenu = append(cfg.TmuxMenu[:newIdx], append([]config.TmuxMenuEntry{entry}, cfg.TmuxMenu[newIdx:]...)...)
			}
			if err := ctx.saveConfig(cfg); err != nil {
				return err
			}
			fmt.Fprintf(ctx.stdout, "Moved entry %s to position %d\n", entry.ID, position)
			return nil
		},
	}
	cmd.Flags().IntVar(&position, "position", 1, "1-based index to move entry to")
	return cmd
}

func newMenuCleanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Remove all tmux menu entries",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := commandContextFrom(cmd)
			if err != nil {
				return err
			}
			cfg, err := ctx.loadConfig()
			if err != nil {
				return err
			}
			if len(cfg.TmuxMenu) == 0 {
				fmt.Fprintln(ctx.stdout, "Menu already empty.")
				return nil
			}
			cfg.TmuxMenu = nil
			if err := ctx.saveConfig(cfg); err != nil {
				return err
			}
			fmt.Fprintln(ctx.stdout, "Cleared all tmux menu entries.")
			return nil
		},
	}
	return cmd
}

func newMenuListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all configured menu entries",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := commandContextFrom(cmd)
			if err != nil {
				return err
			}
			cfg, err := ctx.loadConfig()
			if err != nil {
				return err
			}
			if len(cfg.TmuxMenu) == 0 {
				fmt.Fprintln(ctx.stdout, "No menu entries configured.")
				return nil
			}
			for i, entry := range cfg.TmuxMenu {
				fmt.Fprintf(ctx.stdout, "%2d. [%s] %s -> %s\n", i+1, entry.ID, entry.Label, entry.Command)
			}
			return nil
		},
	}
	return cmd
}

func newMenuShowCmd() *cobra.Command {
	var paneID string
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show popup menu inside tmux (bound to Ctrl-T)",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := commandContextFrom(cmd)
			if err != nil {
				return err
			}
			cfg, err := ctx.loadConfig()
			if err != nil {
				return err
			}
			target := paneID
			if target == "" {
				target = os.Getenv("TMUX_PANE")
			}
			if target == "" {
				return runMenuTui(ctx, "", cfg.TmuxMenu)
			}
			return displayTmuxMenu(cfg.TmuxMenu, target)
		},
	}
	cmd.Flags().StringVar(&paneID, "pane", "", "tmux pane id (internal)")
	return cmd
}

func newTmuxRestartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restart",
		Short: "Restart the jeff tmux session",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := commandContextFrom(cmd)
			if err != nil {
				return err
			}
			if err := runTmux("kill-session", "-t", tmuxSessionName); err != nil {
				if strings.Contains(err.Error(), "can't find session") || strings.Contains(err.Error(), "no server running") {
					// ignore
				} else {
					return fmt.Errorf("kill-session: %w", err)
				}
			}
			removeCtrlTBinding()
			return runTmuxOverlay(ctx)
		},
	}
	return cmd
}

func newTmuxKillCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "kill",
		Short: "Kill the jeff tmux session and remove its hooks",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := runTmux("kill-session", "-t", tmuxSessionName); err != nil {
				if !strings.Contains(err.Error(), "can't find session") {
					return fmt.Errorf("kill-session: %w", err)
				}
			}
			removeCtrlTBinding()
			return nil
		},
	}
	return cmd
}

func generateMenuID(label string, cfg *config.Config) string {
	base := strings.ToLower(label)
	base = strings.TrimSpace(base)
	if base == "" {
		base = "entry"
	}
	base = strings.ReplaceAll(base, " ", "-")
	base = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		return -1
	}, base)
	if base == "" {
		base = "entry"
	}
	unique := base
	count := 1
	for entryExists(unique, cfg.TmuxMenu) {
		count++
		unique = fmt.Sprintf("%s-%d", base, count)
	}
	return unique
}

func entryExists(id string, entries []config.TmuxMenuEntry) bool {
	for _, entry := range entries {
		if entry.ID == id {
			return true
		}
	}
	return false
}

func displayTmuxMenu(entries []config.TmuxMenuEntry, paneTarget string) error {
	if strings.TrimSpace(paneTarget) == "" {
		return errors.New("no tmux pane target")
	}
	args := []string{"display-menu", "-t", paneTarget, "-x", "R", "-y", "P", "-T", "Jeff Shortcuts"}
	for _, entry := range entries {
		cmd := fmt.Sprintf("run-shell %s", shellQuote(entry.Command))
		args = append(args, entry.Label, "", cmd)
	}
	args = append(args, "Close", "", "")
	return runTmux(args...)
}

func shellQuote(input string) string {
	return "'" + strings.ReplaceAll(input, "'", "'\"'\"'") + "'"
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
