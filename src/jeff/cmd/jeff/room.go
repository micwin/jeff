// room.go coordinates file-backed specialist rooms. It owns room config,
// transcripts, deterministic speaker routing, and session calls; it does not
// manage specialist session policies themselves.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var roomNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

type roomConfig struct {
	Name    string       `json:"name"`
	Experts []roomExpert `json:"experts"`
}

type roomExpert struct {
	Alias string `json:"alias"`
}

type roomTurnAnswer struct {
	Alias  string
	Output string
}

func newRoomCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "room",
		Short:   "Coordinate deterministic specialist rooms",
		Aliases: []string{"rooms"},
	}
	cmd.AddCommand(
		newRoomNewCmd(),
		newRoomEnterCmd(),
		newRoomListCmd(),
	)
	return cmd
}

func newRoomNewCmd() *cobra.Command {
	var expertsCSV string
	cmd := &cobra.Command{
		Use:   "new <name> --experts <alias>[,<alias>...]",
		Short: "Create a file-backed specialist room",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := commandContextFrom(cmd)
			if err != nil {
				return err
			}
			experts := splitRoomExperts(expertsCSV)
			return createRoom(ctx, args[0], experts)
		},
	}
	cmd.Flags().StringVar(&expertsCSV, "experts", "", "Comma-separated codex-ctl aliases")
	_ = cmd.MarkFlagRequired("experts")
	return cmd
}

func newRoomEnterCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "enter <name> [message]",
		Short:             "Enter a specialist room or send one room turn",
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: completeRoomNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := commandContextFrom(cmd)
			if err != nil {
				return err
			}
			room, err := loadRoom(ctx, args[0])
			if err != nil {
				return err
			}
			if len(args) > 1 {
				return runRoomTurn(ctx, room, joinArgs(args[1:]))
			}
			return runRoomLoop(ctx, room)
		},
	}
	return cmd
}

func newRoomListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured specialist rooms",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, err := commandContextFrom(cmd)
			if err != nil {
				return err
			}
			names, err := listRoomNames(ctx)
			if err != nil {
				return err
			}
			for _, name := range names {
				fmt.Fprintln(ctx.stdout, name)
			}
			return nil
		},
	}
}

func splitRoomExperts(raw string) []string {
	fields := strings.Split(raw, ",")
	experts := make([]string, 0, len(fields))
	for _, field := range fields {
		trimmed := strings.TrimSpace(field)
		if trimmed != "" {
			experts = append(experts, trimmed)
		}
	}
	return experts
}

func createRoom(ctx *commandContext, name string, requestedExperts []string) error {
	if !roomNamePattern.MatchString(name) {
		return fmt.Errorf("invalid room name %q", name)
	}
	if len(requestedExperts) == 0 {
		return errors.New("at least one expert is required")
	}
	experts := make([]roomExpert, 0, len(requestedExperts))
	seen := make(map[string]struct{})
	for _, requested := range requestedExperts {
		alias, err := resolveCodexCtlAlias(requested)
		if err != nil {
			return err
		}
		if _, exists := seen[alias]; exists {
			continue
		}
		seen[alias] = struct{}{}
		experts = append(experts, roomExpert{Alias: alias})
	}
	room := roomConfig{Name: name, Experts: experts}
	if err := saveRoom(ctx, room); err != nil {
		return err
	}
	fmt.Fprintf(ctx.stdout, "Room %q created with %d expert(s).\n", name, len(experts))
	return nil
}

func runRoomLoop(ctx *commandContext, room roomConfig) error {
	scanner := bufio.NewScanner(ctx.stdin)
	for {
		fmt.Fprintf(ctx.stdout, "%s> ", room.Name)
		if !scanner.Scan() {
			fmt.Fprintln(ctx.stdout)
			return scanner.Err()
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if err := runRoomTurn(ctx, room, line); err != nil {
			return err
		}
	}
}

func runRoomTurn(ctx *commandContext, room roomConfig, message string) error {
	if strings.TrimSpace(message) == "" {
		return errors.New("message must not be empty")
	}
	aliasMap, err := loadCodexCtlShortAliases()
	if err != nil {
		return err
	}
	body, order := resolveRoomTurn(room, aliasMap, message)
	if len(order) == 0 {
		return errors.New("room has no experts")
	}
	if strings.TrimSpace(body) == "" {
		return errors.New("message must not be empty")
	}
	if err := appendRoomTranscript(ctx, room.Name, "user", body); err != nil {
		return err
	}
	answers := make([]roomTurnAnswer, 0, len(order))
	for _, alias := range order {
		output, err := askRoomExpert(alias, room.Name, body, answers)
		if err != nil {
			return err
		}
		answer := roomTurnAnswer{Alias: alias, Output: strings.TrimSpace(output)}
		answers = append(answers, answer)
		if err := appendRoomTranscript(ctx, room.Name, alias, answer.Output); err != nil {
			return err
		}
		if !isPassAnswer(answer.Output) {
			fmt.Fprintf(ctx.stdout, "%s:\n%s\n", alias, answer.Output)
		}
	}
	return nil
}

func resolveRoomTurn(room roomConfig, aliasMap map[string]string, rawMessage string) (string, []string) {
	body := strings.TrimSpace(rawMessage)
	token, rest, hasSelector := splitRoomSelector(body)
	roomExperts := roomExpertSet(room)
	if hasSelector {
		canonical := token
		if target, ok := aliasMap[token]; ok {
			canonical = target
		}
		if _, exists := roomExperts[canonical]; exists {
			body = rest
			return body, rotateRoomExperts(room, canonical)
		}
	}
	return body, roomExpertAliases(room)
}

func splitRoomSelector(message string) (string, string, bool) {
	for _, sep := range []string{":", ","} {
		before, after, ok := strings.Cut(message, sep)
		if !ok {
			continue
		}
		token := strings.TrimSpace(before)
		if roomNamePattern.MatchString(token) {
			return token, strings.TrimSpace(after), true
		}
	}
	return "", message, false
}

func rotateRoomExperts(room roomConfig, first string) []string {
	aliases := roomExpertAliases(room)
	ordered := make([]string, 0, len(aliases))
	ordered = append(ordered, first)
	for _, alias := range aliases {
		if alias != first {
			ordered = append(ordered, alias)
		}
	}
	return ordered
}

func roomExpertSet(room roomConfig) map[string]struct{} {
	set := make(map[string]struct{}, len(room.Experts))
	for _, expert := range room.Experts {
		set[expert.Alias] = struct{}{}
	}
	return set
}

func roomExpertAliases(room roomConfig) []string {
	aliases := make([]string, 0, len(room.Experts))
	for _, expert := range room.Experts {
		aliases = append(aliases, expert.Alias)
	}
	return aliases
}

func askRoomExpert(alias, roomName, message string, previous []roomTurnAnswer) (string, error) {
	prompt := buildRoomPrompt(alias, roomName, message, previous)
	cmd := exec.Command("codex-ctl", "exec", alias, "--", prompt)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return "", fmt.Errorf("codex-ctl exec %s failed: %w: %s", alias, err, msg)
		}
		return "", fmt.Errorf("codex-ctl exec %s failed: %w", alias, err)
	}
	return cleanCodexCtlOutput(stdout.String()), nil
}

func buildRoomPrompt(alias, roomName, message string, previous []roomTurnAnswer) string {
	lines := []string{
		"Goal: Jeff room turn.",
		"Room: " + roomName,
		"Speaker: " + alias,
		"Rules:",
		"- Answer only from your specialist perspective.",
		"- If you have no clarification, correction, risk, or useful addition, answer exactly PASS.",
		"- Do not impersonate another room participant.",
		"- Keep the answer concise and evidence-based.",
		"User message:",
		message,
	}
	if len(previous) > 0 {
		lines = append(lines, "Previous room answers:")
		for _, answer := range previous {
			lines = append(lines, answer.Alias+": "+answer.Output)
		}
	}
	lines = append(lines, "Need: concise answer or PASS.")
	return strings.Join(lines, "\n")
}

func isPassAnswer(output string) bool {
	trimmed := strings.TrimSpace(output)
	lines := strings.Fields(trimmed)
	return len(lines) == 1 && strings.EqualFold(lines[0], "PASS")
}

func cleanCodexCtlOutput(raw string) string {
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	start := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "codex" {
			start = i + 1
		}
	}
	if start < 0 {
		return strings.TrimSpace(raw)
	}
	answerLines := make([]string, 0, len(lines)-start)
	for _, line := range lines[start:] {
		if strings.TrimSpace(line) == "tokens used" {
			break
		}
		answerLines = append(answerLines, line)
	}
	return dedupeTrailingRepeatedLine(strings.TrimSpace(strings.Join(answerLines, "\n")))
}

func dedupeTrailingRepeatedLine(raw string) string {
	lines := strings.Split(raw, "\n")
	if len(lines) < 2 {
		return raw
	}
	last := strings.TrimSpace(lines[len(lines)-1])
	prev := strings.TrimSpace(lines[len(lines)-2])
	if last != "" && last == prev {
		return strings.TrimSpace(strings.Join(lines[:len(lines)-1], "\n"))
	}
	return raw
}

func resolveCodexCtlAlias(alias string) (string, error) {
	cmd := exec.Command("codex-ctl", "show-policy", alias)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return "", fmt.Errorf("resolve codex-ctl alias %q: %w: %s", alias, err, msg)
		}
		return "", fmt.Errorf("resolve codex-ctl alias %q: %w", alias, err)
	}
	for _, line := range strings.Split(stdout.String(), "\n") {
		if value, ok := strings.CutPrefix(line, "alias: "); ok {
			value = strings.TrimSpace(value)
			if value != "" {
				return value, nil
			}
		}
	}
	return "", fmt.Errorf("resolve codex-ctl alias %q: policy output has no alias field", alias)
}

func loadCodexCtlShortAliases() (map[string]string, error) {
	cmd := exec.Command("codex-ctl", "list-short-aliases")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return nil, fmt.Errorf("list codex-ctl short aliases: %w: %s", err, msg)
		}
		return nil, fmt.Errorf("list codex-ctl short aliases: %w", err)
	}
	aliases := make(map[string]string)
	for _, line := range strings.Split(stdout.String(), "\n") {
		shortName, target, ok := strings.Cut(line, " -> ")
		if !ok {
			continue
		}
		aliases[strings.TrimSpace(shortName)] = strings.TrimSpace(target)
	}
	return aliases, nil
}

func saveRoom(ctx *commandContext, room roomConfig) error {
	roomDir, err := roomDir(ctx, room.Name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(roomDir, 0o755); err != nil {
		return fmt.Errorf("create room dir: %w", err)
	}
	content, err := json.MarshalIndent(room, "", "  ")
	if err != nil {
		return fmt.Errorf("encode room: %w", err)
	}
	path := filepath.Join(roomDir, "room.json")
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, content, 0o600); err != nil {
		return fmt.Errorf("write temp room: %w", err)
	}
	return os.Rename(tmpPath, path)
}

func loadRoom(ctx *commandContext, name string) (roomConfig, error) {
	roomDir, err := roomDir(ctx, name)
	if err != nil {
		return roomConfig{}, err
	}
	data, err := os.ReadFile(filepath.Join(roomDir, "room.json"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return roomConfig{}, fmt.Errorf("room %q does not exist", name)
		}
		return roomConfig{}, fmt.Errorf("read room: %w", err)
	}
	var room roomConfig
	if err := json.Unmarshal(data, &room); err != nil {
		return roomConfig{}, fmt.Errorf("parse room: %w", err)
	}
	if room.Name == "" {
		room.Name = name
	}
	if len(room.Experts) == 0 {
		return roomConfig{}, fmt.Errorf("room %q has no experts", name)
	}
	return room, nil
}

func roomDir(ctx *commandContext, name string) (string, error) {
	if !roomNamePattern.MatchString(name) {
		return "", fmt.Errorf("invalid room name %q", name)
	}
	root, err := ctx.store.RoomsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, name), nil
}

func appendRoomTranscript(ctx *commandContext, roomName, speaker, text string) error {
	roomDir, err := roomDir(ctx, roomName)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(roomDir, 0o755); err != nil {
		return fmt.Errorf("create room dir: %w", err)
	}
	file, err := os.OpenFile(filepath.Join(roomDir, "transcript.md"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("open room transcript: %w", err)
	}
	defer file.Close()
	timestamp := time.Now().UTC().Format(time.RFC3339)
	_, err = fmt.Fprintf(file, "## %s %s\n\n%s\n\n", timestamp, speaker, strings.TrimSpace(text))
	return err
}

func listRoomNames(ctx *commandContext) ([]string, error) {
	root, err := ctx.store.RoomsDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read rooms: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

func completeRoomNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	ctx, err := commandContextFrom(cmd)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	names, err := listRoomNames(ctx)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	filtered := make([]string, 0, len(names))
	for _, name := range names {
		if strings.HasPrefix(name, toComplete) {
			filtered = append(filtered, name)
		}
	}
	return filtered, cobra.ShellCompDirectiveNoFileComp
}
