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

type menuMode int

const (
	modeList menuMode = iota
	modeAddCommand
	modeAddLabel
	modeEditCommand
	modeEditLabel
	modeConfirmDelete
	modeMove
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

			entries, err := ctx.loadMenuEntries()
			if err != nil {
				return err
			}

			return runMenuTui(ctx, target, entries)
		},
	}
	cmd.Flags().StringVar(&paneID, "pane", "", "tmux pane id (internal)")
	cmd.Hidden = true
	return cmd
}

func runMenuTui(ctx *commandContext, paneID string, entries []config.TmuxMenuEntry) error {
	model := newMenuModel(ctx, paneID, entries)
	program := tea.NewProgram(model, tea.WithAltScreen())
	final, err := program.Run()
	if err != nil {
		return err
	}

	if result, ok := final.(*menuModel); ok {
		if result.runCommand != "" {
			return runMenuCommand(ctx, paneID, result.runCommand)
		}
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

	return runTmux("send-keys", "-t", paneID, command, "C-m")
}

type menuModel struct {
	ctx      *commandContext
	paneID   string
	entries  []config.TmuxMenuEntry
	path     []int
	selected int

	mode          menuMode
	textInput     textinput.Model
	pending       config.TmuxMenuEntry
	pendingIdx    int
	pendingType   string
	labelOptional bool

	moveOriginal    []config.TmuxMenuEntry
	moveOriginalIdx int

	statusMessage string
	err           error
	runCommand    string
}

func newMenuModel(ctx *commandContext, paneID string, entries []config.TmuxMenuEntry) *menuModel {
	return &menuModel{
		ctx:      ctx,
		paneID:   paneID,
		entries:  cloneEntries(entries),
		selected: 0,
		mode:     modeList,
	}
}

func (m *menuModel) Init() tea.Cmd { return nil }

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
	if len(m.path) == 0 {
		b.WriteString("Jeff shortcuts\n\n")
	} else {
		b.WriteString(fmt.Sprintf("Jeff shortcuts (%s)\n\n", m.breadcrumb()))
	}

	entries := m.currentEntriesVal()
	if len(entries) == 0 {
		b.WriteString("  (no entries yet)\n")
	}

	for i, entry := range entries {
		cursor := " "
		if i == m.selected && !m.isUpSelected() {
			if m.mode == modeMove {
				cursor = "◉"
			} else {
				cursor = ">"
			}
		}
		label := labelOrCommand(entry.Label, entry.Command)
		if entry.Type == config.MenuEntryTypeMenu {
			label += " ▸"
		}
		if entry.Type == config.MenuEntryTypeCommand {
			fmt.Fprintf(&b, " %s %-3d %-20s\n", cursor, i+1, truncate(label, 20))
		} else {
			fmt.Fprintf(&b, " %s %-3d %-20s\n", cursor, i+1, truncate(label, 20))
		}
	}

	if len(m.path) > 0 {
		cursor := " "
		if m.isUpSelected() {
			cursor = ">"
		}
		b.WriteString(fmt.Sprintf(" %s     .. (up)\n", cursor))
	}

	b.WriteString("\n")

	switch m.mode {
	case modeAddCommand:
		b.WriteString("Add entry — Command: " + m.textInput.View() + "\n")
		b.WriteString("Enter to continue • Esc cancels\n")
	case modeAddLabel:
		labelPrompt := "Add entry — Label"
		if m.labelOptional {
			labelPrompt += " (optional)"
		}
		b.WriteString(labelPrompt + ": " + m.textInput.View() + "\n")
		b.WriteString("Enter to save • Esc cancels\n")
	case modeEditCommand:
		b.WriteString("Edit entry — Command: " + m.textInput.View() + "\n")
		b.WriteString("Enter to continue • Esc cancels\n")
	case modeEditLabel:
		labelPrompt := "Edit entry — Label"
		if m.labelOptional {
			labelPrompt += " (optional)"
		}
		b.WriteString(labelPrompt + ": " + m.textInput.View() + "\n")
		b.WriteString("Enter to save • Esc cancels\n")
	case modeConfirmDelete:
		if entry := m.currentEntry(); entry != nil {
			fmt.Fprintf(&b, "Delete %q? y/N (Esc to cancel)\n", entry.Label)
		}
	case modeMove:
		b.WriteString("Move mode: ↑/↓ move • Enter confirm • Esc cancels\n")
	default:
		help := "Keys: ↑/↓ move • Enter run/open • a append cmd • i insert cmd • A append menu • I insert menu • e edit • d delete • m move • Esc/q close"
		b.WriteString(wrapText(help, 70) + "\n")
	}

	if m.err != nil {
		fmt.Fprintf(&b, "\nError: %s\n", m.err)
	} else if m.statusMessage != "" {
		fmt.Fprintf(&b, "\n%s\n", m.statusMessage)
	}

	return b.String()
}

func (m *menuModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.mode {
	case modeAddCommand, modeAddLabel, modeEditCommand, modeEditLabel:
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
	switch msg.Type {
	case tea.KeyLeft:
		if m.leaveSubmenu() {
			return m, nil
		}
	case tea.KeyRight:
		if m.enterSubmenu() {
			return m, nil
		}
	}
	switch msg.String() {
	case "ctrl+c", "esc", "q":
		return m, tea.Quit
	case "up", "k":
		m.moveSelection(-1, false)
	case "down", "j":
		m.moveSelection(1, false)
	case "enter":
		if m.activateSelection() {
			return m, tea.Quit
		}
	case "a":
		m.beginAppendCommand()
	case "i":
		m.beginInsertCommand()
	case "A":
		m.beginAppendMenu()
	case "I":
		m.beginInsertMenu()
	case "e":
		m.beginEdit()
	case "d", "delete":
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
		requireLabel := (m.mode == modeAddLabel || m.mode == modeEditLabel) && !m.labelOptional
		requireCommand := m.mode == modeAddCommand || m.mode == modeEditCommand
		if value == "" && (requireLabel || requireCommand) {
			m.statusMessage = "Value cannot be empty"
			return m, nil
		}
		switch m.mode {
		case modeAddCommand:
			m.pending.Command = value
			m.labelOptional = true
			m.mode = modeAddLabel
			m.textInput = newTextInput("", "Label (optional)")
		case modeAddLabel:
			m.pending.Label = value
			m.finishAddEntry()
		case modeEditCommand:
			m.pending.Command = value
			m.labelOptional = true
			m.mode = modeEditLabel
			m.textInput = newTextInput(m.pending.Label, "Label (optional)")
		case modeEditLabel:
			m.pending.Label = value
			m.finishEditEntry()
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
		m.entries = cloneEntries(m.moveOriginal)
		m.selected = m.moveOriginalIdx
		m.mode = modeList
		m.statusMessage = "Move canceled"
		m.persistEntries()
	}
	return m, nil
}

func (m *menuModel) beginAppendCommand() {
	idx := len(m.currentEntriesVal())
	m.beginAddEntry(idx, config.MenuEntryTypeCommand)
}

func (m *menuModel) beginInsertCommand() {
	m.beginAddEntry(m.insertIndex(), config.MenuEntryTypeCommand)
}

func (m *menuModel) beginAppendMenu() {
	idx := len(m.currentEntriesVal())
	m.beginAddEntry(idx, config.MenuEntryTypeMenu)
}

func (m *menuModel) beginInsertMenu() {
	m.beginAddEntry(m.insertIndex(), config.MenuEntryTypeMenu)
}

func (m *menuModel) beginAddEntry(idx int, entryType string) {
	slice := m.currentEntries()
	if idx < 0 {
		idx = 0
	}
	if idx > len(*slice) {
		idx = len(*slice)
	}
	m.pending = config.TmuxMenuEntry{}
	m.pendingIdx = idx
	m.pendingType = entryType
	if entryType == config.MenuEntryTypeCommand {
		m.mode = modeAddCommand
		m.textInput = newTextInput("", "Command")
	} else {
		m.labelOptional = false
		m.mode = modeAddLabel
		m.textInput = newTextInput("", "New label")
	}
	m.statusMessage = ""
}

func (m *menuModel) finishAddEntry() {
	entry := m.pending
	entry.Type = m.pendingType
	if entry.Type == config.MenuEntryTypeMenu {
		if strings.TrimSpace(entry.Label) == "" {
			m.statusMessage = "Label required for menu"
			return
		}
		entry.Command = ""
	} else {
		if entry.Command == "" {
			m.statusMessage = "Command required"
			return
		}
	}

	entry.ID = generateMenuID(labelOrCommand(entry.Label, entry.Command), m.entries)
	insertMenuEntry(m.currentEntries(), entry, m.pendingIdx)
	m.selected = m.pendingIdx
	m.mode = modeList
	m.statusMessage = fmt.Sprintf("Added %s", entry.Label)
	m.persistEntries()
}

func (m *menuModel) beginEdit() {
	entry := m.currentEntry()
	if entry == nil {
		m.statusMessage = "Nothing to edit"
		return
	}
	m.pendingIdx = m.selected
	m.pending = *entry
	m.pendingType = entry.Type
	if entry.Type == config.MenuEntryTypeCommand {
		m.labelOptional = true
		m.mode = modeEditCommand
		m.textInput = newTextInput(entry.Command, "Command")
	} else {
		m.labelOptional = false
		m.mode = modeEditLabel
		m.textInput = newTextInput(entry.Label, "Label")
	}
	m.statusMessage = ""
}

func (m *menuModel) finishEditEntry() {
	entries := m.currentEntries()
	if m.pendingIdx < 0 || m.pendingIdx >= len(*entries) {
		return
	}
	if m.pendingType == config.MenuEntryTypeCommand {
		if m.pending.Command == "" {
			m.statusMessage = "Command required"
			return
		}
		if m.pending.Label == "" {
			m.pending.Label = m.pending.Command
		}
	} else {
		if strings.TrimSpace(m.pending.Label) == "" {
			m.statusMessage = "Label required for menu"
			return
		}
		m.pending.Command = ""
	}
	(*entries)[m.pendingIdx].Label = m.pending.Label
	(*entries)[m.pendingIdx].Command = m.pending.Command
	(*entries)[m.pendingIdx].Type = m.pendingType
	m.mode = modeList
	m.statusMessage = fmt.Sprintf("Updated %s", m.pending.Label)
	m.persistEntries()
}

func (m *menuModel) beginDelete() {
	if m.currentEntry() == nil {
		m.statusMessage = "Nothing to delete"
		return
	}
	m.mode = modeConfirmDelete
	m.statusMessage = ""
}

func (m *menuModel) deleteSelected() {
	entries := m.currentEntries()
	if m.currentEntry() == nil {
		m.mode = modeList
		return
	}
	removeMenuEntry(entries, m.selected)
	if m.selected >= len(*entries) && m.selected > 0 {
		m.selected--
	}
	m.mode = modeList
	m.statusMessage = "Entry removed"
	m.persistEntries()
}

func (m *menuModel) beginMove() {
	if m.currentEntry() == nil {
		m.statusMessage = "Nothing to move"
		return
	}
	m.moveOriginal = cloneEntries(m.entries)
	m.moveOriginalIdx = m.selected
	m.mode = modeMove
	m.statusMessage = ""
}

func (m *menuModel) moveSelection(delta int, moveEntry bool) {
	levelEntries := m.currentEntriesVal()
	max := len(levelEntries)
	if len(m.path) > 0 {
		max++
	}
	if max == 0 {
		m.selected = 0
		return
	}
	newIdx := m.selected + delta
	if newIdx < 0 {
		newIdx = 0
	}
	if newIdx >= max {
		newIdx = max - 1
	}
	if moveEntry {
		if len(levelEntries) == 0 {
			return
		}
		if newIdx >= len(levelEntries) {
			newIdx = len(levelEntries) - 1
		}
		if newIdx < 0 {
			newIdx = 0
		}
		moveMenuEntry(m.currentEntries(), m.selected, newIdx)
		m.persistEntries()
	}
	m.selected = newIdx
}

func (m *menuModel) activateSelection() bool {
	if m.isUpSelected() {
		if len(m.path) > 0 {
			m.leaveSubmenu()
		}
		return false
	}
	entry := m.currentEntry()
	if entry == nil {
		return false
	}
	if entry.Type == config.MenuEntryTypeMenu {
		m.enterSubmenu()
		return false
	}
	m.runCommand = entry.Command
	return true
}

func (m *menuModel) enterSubmenu() bool {
	entry := m.currentEntry()
	if entry == nil || entry.Type != config.MenuEntryTypeMenu {
		return false
	}
	ensureChildrenSlice(entry)
	parentIndex := m.selected
	m.path = append(m.path, parentIndex)
	m.selected = 0
	return true
}

func (m *menuModel) leaveSubmenu() bool {
	if len(m.path) == 0 {
		return false
	}
	parentIndex := m.path[len(m.path)-1]
	m.path = m.path[:len(m.path)-1]
	entries := m.currentEntries()
	if len(*entries) == 0 {
		m.selected = 0
		return true
	}
	if parentIndex >= len(*entries) {
		parentIndex = len(*entries) - 1
	}
	if parentIndex < 0 {
		parentIndex = 0
	}
	m.selected = parentIndex
	return true
}

func (m *menuModel) currentEntries() *[]config.TmuxMenuEntry {
	entries := &m.entries
	for _, idx := range m.path {
		if idx < 0 || idx >= len(*entries) {
			return entries
		}
		child := &(*entries)[idx]
		ensureChildrenSlice(child)
		entries = &child.Children
	}
	return entries
}

func (m *menuModel) currentEntriesVal() []config.TmuxMenuEntry {
	return *m.currentEntries()
}

func (m *menuModel) currentEntry() *config.TmuxMenuEntry {
	entries := m.currentEntries()
	if m.selected < 0 || m.selected >= len(*entries) {
		return nil
	}
	return &(*entries)[m.selected]
}

func (m *menuModel) isUpSelected() bool {
	return len(m.path) > 0 && m.selected == len(m.currentEntriesVal())
}

func (m *menuModel) insertIndex() int {
	if m.isUpSelected() {
		return len(m.currentEntriesVal())
	}
	return m.selected
}

func (m *menuModel) breadcrumb() string {
	labels := []string{}
	entries := &m.entries
	for _, idx := range m.path {
		if idx < 0 || idx >= len(*entries) {
			break
		}
		entry := &(*entries)[idx]
		labels = append(labels, labelOrCommand(entry.Label, entry.Command))
		ensureChildrenSlice(entry)
		entries = &entry.Children
	}
	return strings.Join(labels, " / ")
}

func (m *menuModel) persistEntries() {
	if err := m.ctx.saveMenuEntries(cloneEntries(m.entries)); err != nil {
		m.err = err
	}
}

func ensureChildrenSlice(entry *config.TmuxMenuEntry) {
	if entry.Children == nil {
		entry.Children = []config.TmuxMenuEntry{}
	}
}

func insertMenuEntry(entries *[]config.TmuxMenuEntry, entry config.TmuxMenuEntry, idx int) {
	slice := *entries
	if idx < 0 {
		idx = 0
	}
	if idx > len(slice) {
		idx = len(slice)
	}
	slice = append(slice, config.TmuxMenuEntry{})
	copy(slice[idx+1:], slice[idx:])
	slice[idx] = entry
	*entries = slice
}

func removeMenuEntry(entries *[]config.TmuxMenuEntry, idx int) {
	slice := *entries
	if idx < 0 || idx >= len(slice) {
		return
	}
	*entries = append(slice[:idx], slice[idx+1:]...)
}

func moveMenuEntry(entries *[]config.TmuxMenuEntry, from, to int) {
	slice := *entries
	if from < 0 || from >= len(slice) || to < 0 || to >= len(slice) {
		return
	}
	entry := slice[from]
	slice = append(slice[:from], slice[from+1:]...)
	if to > len(slice) {
		to = len(slice)
	}
	slice = append(slice[:to], append([]config.TmuxMenuEntry{entry}, slice[to:]...)...)
	*entries = slice
}

func cloneEntries(entries []config.TmuxMenuEntry) []config.TmuxMenuEntry {
	cloned := make([]config.TmuxMenuEntry, len(entries))
	for i := range entries {
		cloned[i] = entries[i]
		if len(entries[i].Children) > 0 {
			cloned[i].Children = cloneEntries(entries[i].Children)
		}
	}
	return cloned
}

func truncate(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	if limit <= 1 {
		return text[:limit]
	}
	return text[:limit-1] + "…"
}

func newTextInput(value, placeholder string) textinput.Model {
	ti := textinput.New()
	ti.Prompt = ""
	ti.Placeholder = placeholder
	ti.CharLimit = 256
	ti.SetValue(value)
	ti.Focus()
	return ti
}

func wrapText(text string, width int) string {
	if width <= 0 {
		width = 70
	}
	words := strings.Fields(text)
	if len(words) == 0 {
		return ""
	}
	lines := make([]string, 0)
	current := words[0]
	for _, word := range words[1:] {
		if len(current)+1+len(word) > width {
			lines = append(lines, current)
			current = word
		} else {
			current += " " + word
		}
	}
	lines = append(lines, current)
	return strings.Join(lines, "\n")
}
