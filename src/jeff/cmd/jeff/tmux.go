package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
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
		newTmuxInitCmd(),
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
	var parentID string
	var createMenu bool

	cmd := &cobra.Command{
		Use:   "add-entry",
		Short: "Add a menu entry",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := commandContextFrom(cmd)
			if err != nil {
				return err
			}
			label = strings.TrimSpace(label)
			commandStr = strings.TrimSpace(commandStr)

			entries, err := ctx.loadMenuEntries()
			if err != nil {
				return err
			}

			targetSlice, err := menuEntriesForParent(&entries, parentID)
			if err != nil {
				return err
			}

			entryType := config.MenuEntryTypeCommand
			if createMenu {
				entryType = config.MenuEntryTypeMenu
			}

			if entryType == config.MenuEntryTypeMenu {
				if label == "" {
					return errors.New("--label is required for menus")
				}
				commandStr = ""
			} else if commandStr == "" {
				return errors.New("--command is required (omit --menu to add commands)")
			}

			entryID := customID
			if entryID == "" {
				entryID = generateMenuID(labelOrCommand(label, commandStr), entries)
			}

			entry := config.TmuxMenuEntry{
				ID:    entryID,
				Label: label,
				Type:  entryType,
			}
			if entry.Type == config.MenuEntryTypeCommand {
				entry.Command = commandStr
				if entry.Label == "" {
					entry.Label = commandStr
				}
			} else {
				entry.Command = ""
			}

			idx := len(*targetSlice)
			if insertIndex > 0 {
				if insertIndex > len(*targetSlice)+1 {
					insertIndex = len(*targetSlice) + 1
				}
				idx = insertIndex - 1
			}
			insertMenuEntry(targetSlice, entry, idx)

			if err := ctx.saveMenuEntries(entries); err != nil {
				return err
			}
			position := idx + 1
			if parentID != "" {
				fmt.Fprintf(ctx.stdout, "Added menu entry %s (%s) under %s at position %d\n", entryID, entry.Label, parentID, position)
			} else {
				fmt.Fprintf(ctx.stdout, "Added menu entry %s (%s) at position %d\n", entryID, entry.Label, position)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&label, "label", "", "Menu label (defaults to command for commands)")
	cmd.Flags().StringVar(&commandStr, "command", "", "Command to execute")
	cmd.Flags().StringVar(&customID, "id", "", "Optional entry id")
	cmd.Flags().IntVar(&insertIndex, "index", 0, "1-based position for the new entry (defaults to append)")
	cmd.Flags().StringVar(&parentID, "parent", "", "Optional parent menu entry id")
	cmd.Flags().BoolVar(&createMenu, "menu", false, "Create a submenu instead of a command entry")

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
			entries, err := ctx.loadMenuEntries()
			if err != nil {
				return err
			}
			loc, ok := findMenuLocation(&entries, args[0])
			if !ok {
				return fmt.Errorf("no menu entry with id %q", args[0])
			}
			removeMenuEntry(loc.entries, loc.index)
			if err := ctx.saveMenuEntries(entries); err != nil {
				return err
			}
			fmt.Fprintf(ctx.stdout, "Deleted menu entry %s\n", args[0])
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
			entries, err := ctx.loadMenuEntries()
			if err != nil {
				return err
			}
			loc, ok := findMenuLocation(&entries, args[0])
			if !ok {
				return fmt.Errorf("no menu entry %q", args[0])
			}
			entry := loc.entry()
			newLabel = strings.TrimSpace(newLabel)
			if entry.Type == config.MenuEntryTypeMenu && newLabel == "" {
				return errors.New("--label is required for menus")
			}
			if entry.Type == config.MenuEntryTypeCommand && newLabel == "" && entry.Command != "" {
				newLabel = entry.Command
			}
			entry.Label = newLabel
			if err := ctx.saveMenuEntries(entries); err != nil {
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
			entries, err := ctx.loadMenuEntries()
			if err != nil {
				return err
			}
			loc, ok := findMenuLocation(&entries, args[0])
			if !ok {
				return fmt.Errorf("no menu entry %q", args[0])
			}
			entrySlice := loc.entries
			entry := (*entrySlice)[loc.index]
			removeMenuEntry(entrySlice, loc.index)
			if position > len(*entrySlice)+1 {
				position = len(*entrySlice) + 1
			}
			newIdx := position - 1
			if newIdx < 0 {
				newIdx = 0
			}
			insertMenuEntry(entrySlice, entry, newIdx)
			if err := ctx.saveMenuEntries(entries); err != nil {
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
			entries, err := ctx.loadMenuEntries()
			if err != nil {
				return err
			}
			if len(entries) == 0 {
				fmt.Fprintln(ctx.stdout, "Menu already empty.")
				return nil
			}
			if err := ctx.saveMenuEntries(nil); err != nil {
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
			entries, err := ctx.loadMenuEntries()
			if err != nil {
				return err
			}
			if len(entries) == 0 {
				fmt.Fprintln(ctx.stdout, "No menu entries configured.")
				return nil
			}
			printMenuEntries(ctx.stdout, entries, "")
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
			entries, err := ctx.loadMenuEntries()
			if err != nil {
				return err
			}
			target := paneID
			if target == "" {
				target = os.Getenv("TMUX_PANE")
			}
			if target == "" {
				return runMenuTui(ctx, "", entries)
			}
			return displayTmuxMenu(target)
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

func runTmuxInit(ctx *commandContext) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home: %w", err)
	}
	confPath := filepath.Join(home, ".tmux.conf")
	var contents []byte
	if data, err := os.ReadFile(confPath); err == nil {
		contents = data
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read %s: %w", confPath, err)
	}

	hasExtended := bytes.Contains(contents, []byte("set -g extended-keys on"))
	hasFormat := bytes.Contains(contents, []byte("set -g extended-keys-format csi-u"))
	if hasExtended && hasFormat {
		fmt.Fprintf(ctx.stdout, "tmux config already enables extended keys in %s\n", confPath)
		return nil
	}

	fmt.Fprintf(ctx.stdout, "Jeff can add the following snippet to %s to enable Shift+Enter in tmux:\n\n", confPath)
	fmt.Fprintf(ctx.stdout, "set -g extended-keys on\n")
	fmt.Fprintf(ctx.stdout, "set -g extended-keys-format csi-u\n\n")

	ok, err := promptYesNo(ctx, "Add snippet and restart tmux now? [y/N]: ")
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("aborted: tmux must have extended-keys enabled for Shift+Enter to work")
	}

	f, err := os.OpenFile(confPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open %s: %w", confPath, err)
	}
	snippet := "\n# Added by jeff tmux init\nset -g extended-keys on\nset -g extended-keys-format csi-u\n"
	if _, err := f.WriteString(snippet); err != nil {
		_ = f.Close()
		return fmt.Errorf("write %s: %w", confPath, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("flush %s: %w", confPath, err)
	}

	fmt.Fprintf(ctx.stdout, "Appended snippet to %s\n", confPath)
	if err := exec.Command("tmux", "kill-server").Run(); err != nil && !strings.Contains(err.Error(), "no server running") {
		fmt.Fprintf(ctx.stderr, "warning: tmux kill-server failed: %v\n", err)
	}
	fmt.Fprintln(ctx.stdout, "tmux server restarted (if it was running). Shift+Enter will now be forwarded once you start Jeff tmux again.")
	return nil
}

func promptYesNo(ctx *commandContext, message string) (bool, error) {
	fmt.Fprint(ctx.stdout, message)
	reader := bufio.NewReader(ctx.stdin)
	resp, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	resp = strings.TrimSpace(resp)
	resp = strings.ToLower(resp)
	return resp == "y" || resp == "yes", nil
}

func newTmuxInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Ensure tmux config enables extended key reporting (Shift+Enter, etc.)",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := commandContextFrom(cmd)
			if err != nil {
				return err
			}
			return runTmuxInit(ctx)
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

func generateMenuID(label string, entries []config.TmuxMenuEntry) string {
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
	ids := make(map[string]struct{})
	collectMenuIDs(entries, ids)
	for {
		if _, exists := ids[unique]; !exists {
			break
		}
		count++
		unique = fmt.Sprintf("%s-%d", base, count)
	}
	return unique
}

func displayTmuxMenu(paneTarget string) error {
	if strings.TrimSpace(paneTarget) == "" {
		return errors.New("no tmux pane target")
	}
	cmd := fmt.Sprintf("jeff menu tui --pane %s", shellQuote(paneTarget))
	return runTmux("display-popup", "-w", "40%", "-h", "90%", "-x", "R", "-E", cmd)
}

func collectMenuIDs(entries []config.TmuxMenuEntry, ids map[string]struct{}) {
	for i := range entries {
		if entries[i].ID != "" {
			ids[entries[i].ID] = struct{}{}
		}
		if len(entries[i].Children) > 0 {
			collectMenuIDs(entries[i].Children, ids)
		}
	}
}

type menuLocation struct {
	entries *[]config.TmuxMenuEntry
	index   int
}

func (loc *menuLocation) entry() *config.TmuxMenuEntry {
	return &(*loc.entries)[loc.index]
}

func findMenuLocation(entries *[]config.TmuxMenuEntry, id string) (*menuLocation, bool) {
	for i := range *entries {
		if (*entries)[i].ID == id {
			return &menuLocation{entries: entries, index: i}, true
		}
		if loc, ok := findMenuLocation(&(*entries)[i].Children, id); ok {
			return loc, true
		}
	}
	return nil, false
}

func menuEntriesForParent(entries *[]config.TmuxMenuEntry, parentID string) (*[]config.TmuxMenuEntry, error) {
	if parentID == "" {
		return entries, nil
	}
	loc, ok := findMenuLocation(entries, parentID)
	if !ok {
		return nil, fmt.Errorf("no menu entry %q", parentID)
	}
	entry := loc.entry()
	if entry.Type != config.MenuEntryTypeMenu {
		return nil, fmt.Errorf("entry %s is not a submenu", parentID)
	}
	ensureChildrenSlice(entry)
	return &entry.Children, nil
}

func printMenuEntries(w io.Writer, entries []config.TmuxMenuEntry, prefix string) {
	for _, entry := range entries {
		label := labelOrCommand(entry.Label, entry.Command)
		flag := ""
		if entry.Type == config.MenuEntryTypeMenu {
			flag = " (menu)"
		}
		fmt.Fprintf(w, "%s- [%s] %s%s\n", prefix, entry.ID, label, flag)
		if len(entry.Children) > 0 {
			printMenuEntries(w, entry.Children, prefix+"  ")
		}
	}
}

func labelOrCommand(label, command string) string {
	if strings.TrimSpace(label) != "" {
		return label
	}
	return strings.TrimSpace(command)
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
