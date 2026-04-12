package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"jeff/internal/config"
)

func newMenuTuiCmd() *cobra.Command {
	var paneID string
	cmd := &cobra.Command{
		Use:   "tui",
		Short: "Interactive Jeff menu editor (used by Ctrl-T)",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := commandContextFrom(cmd)
			if err != nil {
				return err
			}
			target := paneID
			if target == "" {
				target = os.Getenv("TMUX_PANE")
			}
			cfg, err := ctx.loadConfig()
			if err != nil {
				return err
			}

			return runMenuTui(ctx, target, cfg.TmuxMenu)
		},
	}
	cmd.Flags().StringVar(&paneID, "pane", "", "tmux pane id (internal)")
	cmd.Hidden = true
	return cmd
}

type menuMode int

const (
	modeList menuMode = iota
	modeAddLabel
	modeAddCommand
	modeEditLabel
	modeEditCommand
	modeConfirmDelete
	modeMove
)

type menuModel struct {
	ctx        *commandContext
	paneID     string
	entries    []config.TmuxMenuEntry
	selected   int
	mode       menuMode
	textInput  textinput.Model
	pending    config.TmuxMenuEntry
	pendingIdx int

	moveOriginal    []config.TmuxMenuEntry
	moveOriginalIdx int

	statusMessage string
	err           error
	runCommand    string
}

func runMenuTui(ctx *commandContext, paneID string, entries []config.TmuxMenuEntry) error {
	model := newMenuModel(ctx, paneID, entries)
	p := tea.NewProgram(model, tea.WithAltScreen())
	final, err := p.Run()
	if err != nil {
		return err
	}
	result, ok := final.(*menuModel)
	if !ok {
		return nil
	}
	if result.runCommand != "" {
		return runMenuCommand(ctx, paneID, result.runCommand)
	}
	return nil
}

func runMenuCommand(ctx *commandContext, paneID, command string) error {
	if strings.TrimSpace(command) == "" {
		return nil
	}
	if paneID == "" {
		cmd := exec.Command(userShellPath(), "-c", command)
		cmd.Stdin = ctx.stdin
		cmd.Stdout = ctx.stdout
		cmd.Stderr = ctx.stderr
		return cmd.Run()
	}
	return runTmux("run-shell", "-t", paneID, command)
}

func newMenuModel(ctx *commandContext, paneID string, entries []config.TmuxMenuEntry) *menuModel {
	cloned := append([]config.TmuxMenuEntry(nil), entries...)
	return &menuModel{
		ctx:      ctx,
		paneID:   paneID,
		entries:  cloned,
		selected: 0,
		mode:     modeList,
	}
}

func (m *menuModel) Init() tea.Cmd {
	return nil
}

func (m *menuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)
	default:
		return m, nil
	}
}

func (m *menuModel) View() string {
	var b strings.Builder
	if m.paneID != "" {
		fmt.Fprintf(&b, "Jeff shortcuts (pane %s)\n\n", m.paneID)
	} else {
		b.WriteString("Jeff shortcuts\n\n")
	}

	if len(m.entries) == 0 {
		b.WriteString("  (no entries yet)\n")
	}

	for i, entry := range m.entries {
		cursor := " "
		if i == m.selected && len(m.entries) > 0 {
			if m.mode == modeMove {
				cursor = "◉"
			} else {
				cursor = ">"
			}
		}
		label := entry.Label
		if label == "" {
			label = entry.Command
		}
		fmt.Fprintf(&b, " %s %-3d %-20s %s\n", cursor, i+1, truncate(label, 20), entry.Command)
	}

	b.WriteString("\n")

	switch m.mode {
	case modeAddLabel:
		b.WriteString("Add entry — Label (optional): " + m.textInput.View() + "\n")
		b.WriteString("Enter to continue • Esc cancels\n")
	case modeAddCommand:
		b.WriteString("Add entry — Command: " + m.textInput.View() + "\n")
		b.WriteString("Enter to save • Esc cancels\n")
	case modeEditLabel:
		b.WriteString("Edit entry — Label (optional): " + m.textInput.View() + "\n")
		b.WriteString("Enter to continue • Esc cancels\n")
	case modeEditCommand:
		b.WriteString("Edit entry — Command: " + m.textInput.View() + "\n")
		b.WriteString("Enter to save • Esc cancels\n")
	case modeConfirmDelete:
		if entry := m.currentEntry(); entry != nil {
			fmt.Fprintf(&b, "Delete %q? y/N (Esc to cancel)\n", entry.Label)
		}
	case modeMove:
		b.WriteString("Move mode: ↑/↓ or j/k to reposition • Enter to confirm • Esc cancels\n")
	default:
		b.WriteString("Keys: ↑/↓ or j/k navigate • Enter run • a append • i insert • e edit • d delete • m move • Esc/q close\n")
	}

	if m.err != nil {
		fmt.Fprintf(&b, "\nError: %s\n", m.err.Error())
	} else if m.statusMessage != "" {
		fmt.Fprintf(&b, "\n%s\n", m.statusMessage)
	}

	return b.String()
}

func (m *menuModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.mode {
	case modeAddLabel, modeAddCommand, modeEditLabel, modeEditCommand:
		return m.handleInputKey(msg)
	case modeConfirmDelete:
		return m.handleConfirmKey(msg)
	case modeMove:
		return m.handleMoveKey(msg)
	default:
		return m.handleListKey(msg)
	}
}

func (m *menuModel) handleListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "esc", "q":
		return m, tea.Quit
	case "up", "k":
		m.moveSelection(-1, false)
	case "down", "j":
		m.moveSelection(1, false)
	case "enter":
		m.triggerCurrentEntry()
		return m, tea.Quit
	case "a":
		m.beginAppend()
	case "i":
		m.beginInsert()
	case "e":
		m.beginEdit()
	case "d":
		m.beginDelete()
	case "m":
		m.beginMove()
	}
	return m, nil
}

func (m *menuModel) handleInputKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.mode = modeList
		m.statusMessage = "Canceled"
		return m, nil
	case tea.KeyEnter:
		value := strings.TrimSpace(m.textInput.Value())
		requireValue := true
		if m.mode == modeAddLabel || m.mode == modeEditLabel {
			requireValue = false
		}
		if value == "" && requireValue {
			m.statusMessage = "Value cannot be empty"
			return m, nil
		}
		switch m.mode {
		case modeAddLabel:
			m.pending.Label = value
			m.mode = modeAddCommand
			m.textInput = newTextInput("", "Command")
		case modeAddCommand:
			m.pending.Command = value
			m.finishAdd()
		case modeEditLabel:
			m.pending.Label = value
			m.mode = modeEditCommand
			m.textInput = newTextInput(m.pending.Command, "Command")
		case modeEditCommand:
			m.pending.Command = value
			m.finishEdit()
		}
		return m, nil
	default:
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		return m, cmd
	}
}

func (m *menuModel) handleConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		m.deleteSelected()
	case "esc", "n", "N":
		m.mode = modeList
		m.statusMessage = "Delete canceled"
	}
	return m, nil
}

func (m *menuModel) handleMoveKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		m.moveSelection(-1, true)
	case "down", "j":
		m.moveSelection(1, true)
	case "enter":
		m.mode = modeList
		m.statusMessage = "Entry reordered"
		m.persistEntries()
	case "esc":
		m.entries = append([]config.TmuxMenuEntry(nil), m.moveOriginal...)
		m.selected = m.moveOriginalIdx
		m.mode = modeList
		m.statusMessage = "Move canceled"
	}
	return m, nil
}

func (m *menuModel) beginAppend() {
	m.beginAddAt(len(m.entries))
}

func (m *menuModel) beginInsert() {
	idx := m.selected
	if idx < 0 || idx > len(m.entries) {
		idx = len(m.entries)
	}
	m.beginAddAt(idx)
}

func (m *menuModel) beginAddAt(idx int) {
	if idx < 0 {
		idx = 0
	}
	if idx > len(m.entries) {
		idx = len(m.entries)
	}
	m.pendingIdx = idx
	m.pending = config.TmuxMenuEntry{}
	m.textInput = newTextInput("", "New label (optional)")
	m.mode = modeAddLabel
	m.statusMessage = ""
}

func (m *menuModel) finishAdd() {
	entry := m.pending
	if entry.Command == "" {
		m.statusMessage = "Command required"
		return
	}
	if entry.Label == "" {
		entry.Label = entry.Command
	}

	cfg, err := m.ctx.loadConfig()
	if err != nil {
		m.err = err
		return
	}
	entry.ID = generateMenuID(entry.Label, cfg)

	idx := m.pendingIdx
	if idx < 0 {
		idx = 0
	}
	if idx > len(m.entries) {
		idx = len(m.entries)
	}
	m.entries = insertEntry(m.entries, entry, idx)
	m.selected = idx
	m.mode = modeList
	m.persistEntries()
	m.statusMessage = fmt.Sprintf("Added %s", entry.Label)
}

func (m *menuModel) beginEdit() {
	entry := m.currentEntry()
	if entry == nil {
		m.statusMessage = "Nothing to edit"
		return
	}
	m.pendingIdx = m.selected
	m.pending = *entry
	m.textInput = newTextInput(entry.Label, "Label")
	m.mode = modeEditLabel
	m.statusMessage = ""
}

func (m *menuModel) finishEdit() {
	if m.pendingIdx < 0 || m.pendingIdx >= len(m.entries) {
		return
	}
	if m.pending.Command == "" {
		m.statusMessage = "Command required"
		return
	}
	if m.pending.Label == "" {
		m.pending.Label = m.pending.Command
	}
	m.entries[m.pendingIdx].Label = m.pending.Label
	m.entries[m.pendingIdx].Command = m.pending.Command
	m.mode = modeList
	m.selected = m.pendingIdx
	m.persistEntries()
	m.statusMessage = fmt.Sprintf("Updated %s", m.pending.Label)
}

func (m *menuModel) beginDelete() {
	if m.currentEntry() == nil {
		m.statusMessage = "Menu is empty"
		return
	}
	m.mode = modeConfirmDelete
	m.statusMessage = ""
}

func (m *menuModel) deleteSelected() {
	if len(m.entries) == 0 || m.selected < 0 || m.selected >= len(m.entries) {
		m.mode = modeList
		return
	}
	id := m.entries[m.selected].ID
	m.entries = append(m.entries[:m.selected], m.entries[m.selected+1:]...)
	if m.selected >= len(m.entries) && m.selected > 0 {
		m.selected--
	}
	m.mode = modeList
	m.persistEntries()
	m.statusMessage = fmt.Sprintf("Deleted %s", id)
}

func (m *menuModel) beginMove() {
	if len(m.entries) == 0 {
		m.statusMessage = "Menu is empty"
		return
	}
	m.moveOriginal = append([]config.TmuxMenuEntry(nil), m.entries...)
	m.moveOriginalIdx = m.selected
	m.mode = modeMove
	m.statusMessage = ""
}

func (m *menuModel) moveSelection(delta int, moveEntry bool) {
	if len(m.entries) == 0 {
		m.selected = 0
		return
	}
	newIdx := m.selected + delta
	if newIdx < 0 {
		newIdx = 0
	}
	if newIdx >= len(m.entries) {
		newIdx = len(m.entries) - 1
	}
	if moveEntry && newIdx != m.selected {
		entry := m.entries[m.selected]
		m.entries = append(m.entries[:m.selected], m.entries[m.selected+1:]...)
		if newIdx > m.selected {
			newIdx--
		}
		if newIdx < 0 {
			newIdx = 0
		}
		if newIdx > len(m.entries) {
			newIdx = len(m.entries)
		}
		m.entries = insertEntry(m.entries, entry, newIdx)
	}
	m.selected = newIdx
}

func (m *menuModel) triggerCurrentEntry() {
	entry := m.currentEntry()
	if entry == nil {
		m.runCommand = ""
	} else {
		m.runCommand = entry.Command
	}
}

func (m *menuModel) currentEntry() *config.TmuxMenuEntry {
	if len(m.entries) == 0 || m.selected < 0 || m.selected >= len(m.entries) {
		return nil
	}
	return &m.entries[m.selected]
}

func (m *menuModel) persistEntries() {
	cfg, err := m.ctx.loadConfig()
	if err != nil {
		m.err = err
		return
	}
	cfg.TmuxMenu = append([]config.TmuxMenuEntry(nil), m.entries...)
	if err := m.ctx.saveConfig(cfg); err != nil {
		m.err = err
	}
}

func newTextInput(value, placeholder string) textinput.Model {
	input := textinput.New()
	input.Prompt = ""
	input.Placeholder = placeholder
	input.CharLimit = 256
	input.SetValue(value)
	input.Focus()
	return input
}

func insertEntry(entries []config.TmuxMenuEntry, entry config.TmuxMenuEntry, idx int) []config.TmuxMenuEntry {
	if idx < 0 {
		idx = 0
	}
	if idx > len(entries) {
		idx = len(entries)
	}
	result := append(entries[:idx], append([]config.TmuxMenuEntry{entry}, entries[idx:]...)...)
	return result
}

func truncate(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	return text[:limit-1] + "…"
}
