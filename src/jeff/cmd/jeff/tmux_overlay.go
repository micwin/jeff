package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"jeff/internal/config"
)

const (
	defaultStatusInterval = 5
	tmuxSessionName       = "jeff-shell"
)

type statusUpdate struct {
	pos  alignment
	text string
}

type alignment int

const (
	alignLeft alignment = iota
	alignCenter
	alignRight
)

var shellPathOnce sync.Once
var cachedShellPath string

func userShellPath() string {
	shellPathOnce.Do(func() {
		cachedShellPath = os.Getenv("SHELL")
		if cachedShellPath == "" {
			cachedShellPath = "/bin/sh"
		}
	})
	return cachedShellPath
}

func runTmuxOverlay(ctx *commandContext) error {
	if _, err := exec.LookPath("tmux"); err != nil {
		return errors.New("tmux is required for 'jeff tmux'. Please install tmux (e.g., apt install tmux) and try again")
	}

	if err := ensureTmuxSession(userShellPath()); err != nil {
		return err
	}

	cfg, err := ctx.loadConfig()
	if err != nil {
		return err
	}

	if err := applyTmuxBaseConfig(cfg); err != nil {
		return fmt.Errorf("configure tmux: %w", err)
	}

	statusCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	updateCh := make(chan statusUpdate, 8)
	go statusAggregator(statusCtx, ctx.store, updateCh)
	go pollStatusRegion(statusCtx, ctx.store, "left", alignLeft, updateCh)
	go pollStatusRegion(statusCtx, ctx.store, "center", alignCenter, updateCh)
	go pollStatusRegion(statusCtx, ctx.store, "right", alignRight, updateCh)

	cmd := exec.Command("tmux", "attach-session", "-t", tmuxSessionName)
	cmd.Stdin = ctx.stdin
	cmd.Stdout = ctx.stdout
	cmd.Stderr = ctx.stderr

	err = cmd.Run()
	cancel()
	close(updateCh)
	return err
}

func applyTmuxBaseConfig(cfg *config.Config) error {
	if err := runTmux("start-server"); err != nil {
		return err
	}
	sessionTarget := fmt.Sprintf("%s:", tmuxSessionName)
	if err := runTmux("set-option", "-t", sessionTarget, "status", "on"); err != nil {
		return err
	}
	if err := runTmux("set-option", "-t", sessionTarget, "status-format[0]", "#{@jeff-status-line}"); err != nil {
		return err
	}
	if err := runTmux("set-option", "-t", sessionTarget, "status-position", "bottom"); err != nil {
		return err
	}
	if err := configureCtrlTBinding(); err != nil {
		return err
	}

	// Ensure an initial value so the bar renders immediately.
	initial := buildStatusLine(80, cfg.ShellStatus.Layout, "", "", "")
	tmuxSetSessionOption("jeff-status-line", initial)
	return nil
}

func pollStatusRegion(ctx context.Context, store *config.Store, regionName string, pos alignment, updates chan<- statusUpdate) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		cfg, err := store.Load()
		if err != nil {
			time.Sleep(time.Second)
			continue
		}

		region := regionFromConfig(cfg, regionName)
		command := strings.TrimSpace(region.Command)
		interval := region.Interval
		if interval <= 0 {
			interval = defaultStatusInterval
		}

		text, runErr := renderTemplateWithShell(normalizeTemplateInput(command))
		if runErr != nil {
			text = fmt.Sprintf("err: %s", runErr.Error())
		}

		select {
		case updates <- statusUpdate{pos: pos, text: text}:
		case <-ctx.Done():
			return
		}

		timer := time.NewTimer(time.Duration(interval) * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func statusAggregator(ctx context.Context, store *config.Store, updates <-chan statusUpdate) {
	left, center, right := "", "", ""
	width := 80

	widthTicker := time.NewTicker(2 * time.Second)
	defer widthTicker.Stop()

	refresh := func() {
		cfg, err := store.Load()
		if err != nil {
			return
		}
		layout := cfg.ShellStatus.Layout
		line := buildStatusLine(width, layout, left, center, right)
		tmuxSetSessionOption("jeff-status-line", line)
		tmuxRefreshClient()
	}

	refresh()

	for {
		select {
		case <-ctx.Done():
			return
		case upd, ok := <-updates:
			if !ok {
				return
			}
			switch upd.pos {
			case alignLeft:
				left = upd.text
			case alignCenter:
				center = upd.text
			case alignRight:
				right = upd.text
			}
			refresh()
		case <-widthTicker.C:
			if w := readTmuxWidth(); w > 0 {
				width = w
				refresh()
			}
		}
	}
}

func regionFromConfig(cfg *config.Config, name string) config.ShellStatusRegion {
	switch name {
	case "center":
		return cfg.ShellStatus.Center
	case "right":
		return cfg.ShellStatus.Right
	default:
		return cfg.ShellStatus.Left
	}
}

func buildStatusLine(width int, layout config.ShellStatusLayout, left, center, right string) string {
	if width <= 0 {
		width = 80
	}
	if layout.Left <= 0 {
		layout.Left = 3
	}
	if layout.Center <= 0 {
		layout.Center = 4
	}
	if layout.Right <= 0 {
		layout.Right = 3
	}

	total := layout.Left + layout.Center + layout.Right
	if total <= 0 {
		total = 1
	}

	leftWidth := width * layout.Left / total
	centerWidth := width * layout.Center / total
	rightWidth := width - leftWidth - centerWidth

	leftSeg := formatSegment(left, leftWidth, alignLeft)
	centerSeg := formatSegment(center, centerWidth, alignCenter)
	rightSeg := formatSegment(right, rightWidth, alignRight)

	return leftSeg + centerSeg + rightSeg
}

func formatSegment(text string, width int, pos alignment) string {
	if width <= 0 {
		return ""
	}
	if len(text) > width {
		text = text[:width]
	}
	switch pos {
	case alignLeft:
		return fmt.Sprintf("%-*s", width, text)
	case alignCenter:
		padding := width - len(text)
		if padding <= 0 {
			return text
		}
		leftPad := padding / 2
		rightPad := padding - leftPad
		return strings.Repeat(" ", leftPad) + text + strings.Repeat(" ", rightPad)
	case alignRight:
		return fmt.Sprintf("%*s", width, text)
	default:
		return text
	}
}

func runStatusCommand(command string) (string, error) {
	cmd := exec.Command(userShellPath(), "-c", command)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("%s: %s", command, msg)
	}
	return strings.TrimSpace(string(out)), nil
}

func runTmux(args ...string) error {
	cmd := exec.Command("tmux", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(output))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("tmux %s failed: %s", strings.Join(args, " "), msg)
	}
	return nil
}

func renderTemplateWithShell(template string) (string, error) {
	if strings.TrimSpace(template) == "" {
		return "", nil
	}
	escaped := strings.ReplaceAll(template, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	script := fmt.Sprintf("printf '%%s' \"%s\"", escaped)
	cmd := exec.Command(userShellPath(), "-c", script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("%s: %s", template, msg)
	}
	return string(out), nil
}

func tmuxSetSessionOption(name, value string) {
	target := fmt.Sprintf("%s:", tmuxSessionName)
	_ = runTmux("set-option", "-t", target, "@"+name, value)
}

func tmuxRefreshClient() {
	_ = runTmux("refresh-client", "-S", "-t", tmuxSessionName)
}

func readTmuxWidth() int {
	out, err := exec.Command("tmux", "display-message", "-p", "-t", tmuxSessionName, "#{window_width}").Output()
	if err != nil {
		return 0
	}
	val, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0
	}
	return val
}

func ensureTmuxSession(shell string) error {
	hasCmd := exec.Command("tmux", "has-session", "-t", tmuxSessionName)
	if err := hasCmd.Run(); err == nil {
		return nil
	}

	createCmd := exec.Command("tmux", "new-session", "-d", "-s", tmuxSessionName, shell)
	if output, err := createCmd.CombinedOutput(); err != nil {
		msg := strings.TrimSpace(string(output))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("tmux new-session failed: %s", msg)
	}

	return nil
}

func configureCtrlTBinding() error {
	if err := runTmux("unbind-key", "-q", "-n", "C-t"); err != nil {
		return err
	}
	format := fmt.Sprintf("#{==:#{session_name},%s}", tmuxSessionName)
	popupCmd := fmt.Sprintf("run-shell %s", shellQuote("tmux display-popup -w 40% -h 90% -x R -E \"jeff menu tui --pane '#{pane_id}'\""))
	return runTmux(
		"bind-key",
		"-n",
		"C-t",
		"if-shell",
		"-F",
		format,
		popupCmd,
		"send-keys C-t",
	)
}

func removeCtrlTBinding() {
	_ = runTmux("unbind-key", "-q", "-n", "C-t")
}
