package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"jeff/internal/agent"
)

func newMemcastleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "memcastle",
		Aliases: []string{"mc"},
		Short:   "Inspect Jeff's persistent memory castle",
	}
	cmd.AddCommand(newMemcastleStatusCmd(), newMemcastleTreeCmd(), newMemcastlePathCmd(), newMemcastleContinuityCmd(), newMemcastleSearchCmd(), newMemcastleAskCmd(), newMemcastleCleanupCmd())
	return cmd
}

func newMemcastlePathCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "path [query]",
		Short: "Print memory castle paths",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := commandContextFrom(cmd)
			if err != nil {
				return err
			}
			agentDir, err := ctx.store.AgentDir()
			if err != nil {
				return err
			}
			if len(args) == 0 {
				fmt.Fprintln(ctx.stdout, agentDir)
				return nil
			}
			matches, err := memcastlePathMatches(agentDir, args[0])
			if err != nil {
				return err
			}
			for _, match := range matches {
				fmt.Fprintln(ctx.stdout, match)
			}
			return nil
		},
	}
}

func memcastlePathMatches(root string, rawQuery string) ([]string, error) {
	query := trimPathQuery(rawQuery)
	if query == "" || query == "." || query == "/" {
		return []string{root}, nil
	}
	if filepath.IsAbs(query) {
		return memcastleExactPath(root, query)
	}
	return memcastleSearchPaths(root, query)
}

func trimPathQuery(query string) string {
	query = strings.TrimSpace(query)
	for len(query) >= 2 {
		first := query[0]
		last := query[len(query)-1]
		if (first == '\'' && last == '\'') || (first == '"' && last == '"') || (first == '`' && last == '`') {
			query = strings.TrimSpace(query[1 : len(query)-1])
			continue
		}
		break
	}
	return query
}

func memcastleExactPath(root string, query string) ([]string, error) {
	cleanRel := strings.TrimPrefix(filepath.Clean(query), string(filepath.Separator))
	if cleanRel == "." || cleanRel == "" {
		return []string{root}, nil
	}
	if strings.HasPrefix(cleanRel, "..") {
		return nil, fmt.Errorf("memory castle path escapes root: %s", query)
	}
	candidate := filepath.Join(root, cleanRel)
	if !strings.HasPrefix(candidate, root+string(filepath.Separator)) {
		return nil, fmt.Errorf("memory castle path escapes root: %s", query)
	}
	info, err := os.Stat(candidate)
	if err != nil {
		return nil, fmt.Errorf("memory castle path not found: %s", query)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("memory castle path is not a directory: %s", query)
	}
	return []string{candidate}, nil
}

func memcastleSearchPaths(root string, query string) ([]string, error) {
	patternSegments, err := cleanPathPatternSegments(query)
	if err != nil {
		return nil, err
	}
	var matches []string
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() {
			return nil
		}
		if path != root && entry.Name() == ".git" {
			return filepath.SkipDir
		}
		if path == root {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		segments := strings.Split(filepath.ToSlash(rel), "/")
		if pathPatternMatches(patternSegments, segments) {
			matches = append(matches, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("search memory castle paths: %w", err)
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("no memory castle paths match: %s", query)
	}
	return matches, nil
}

func cleanPathPatternSegments(query string) ([]string, error) {
	clean := filepath.ToSlash(filepath.Clean(query))
	if clean == "." || clean == "" {
		return nil, fmt.Errorf("empty memory castle path query")
	}
	rawSegments := strings.Split(clean, "/")
	segments := make([]string, 0, len(rawSegments))
	for _, segment := range rawSegments {
		if segment == "" || segment == "." {
			continue
		}
		if segment == ".." {
			return nil, fmt.Errorf("memory castle path query cannot contain '..': %s", query)
		}
		segments = append(segments, segment)
	}
	if len(segments) == 0 {
		return nil, fmt.Errorf("empty memory castle path query")
	}
	return segments, nil
}

func pathPatternMatches(pattern []string, segments []string) bool {
	if len(pattern) == 1 && pattern[0] != "*" && pattern[0] != "**" {
		for _, segment := range segments {
			if segment == pattern[0] {
				return true
			}
		}
		return false
	}
	for start := range segments {
		if pathPatternMatchesFrom(pattern, segments[start:]) {
			return true
		}
	}
	return pathPatternMatchesFrom(pattern, nil)
}

func pathPatternMatchesFrom(pattern []string, segments []string) bool {
	if len(pattern) == 0 {
		return len(segments) == 0
	}
	if pattern[0] == "**" {
		for consumed := 0; consumed <= len(segments); consumed++ {
			if pathPatternMatchesFrom(pattern[1:], segments[consumed:]) {
				return true
			}
		}
		return false
	}
	if len(segments) == 0 {
		return false
	}
	if pattern[0] != "*" && pattern[0] != segments[0] {
		return false
	}
	return pathPatternMatchesFrom(pattern[1:], segments[1:])
}

func newMemcastleStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Print memory castle metadata",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, err := commandContextFrom(cmd)
			if err != nil {
				return err
			}
			info, err := agent.CastleInfoFor(ctx.store)
			if err != nil {
				return err
			}
			fmt.Fprintln(ctx.stdout, "Memory Castle")
			fmt.Fprintf(ctx.stdout, "Root: %s\n\n", info.Root)
			fmt.Fprintln(ctx.stdout, "Summary")
			fmt.Fprintf(ctx.stdout, "  Files:    %d\n", info.Files)
			fmt.Fprintf(ctx.stdout, "  Wings:    %d\n", info.Wings)
			fmt.Fprintf(ctx.stdout, "  Floors:   %d\n", info.Floors)
			fmt.Fprintf(ctx.stdout, "  Rooms:    %d\n", info.Rooms)
			fmt.Fprintf(ctx.stdout, "  Cabinets: %d\n", info.Cabinets)
			fmt.Fprintf(ctx.stdout, "  Drawers:  %d\n", info.Drawers)
			return nil
		},
	}
}

func newMemcastleTreeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tree",
		Short: "Print the memory castle directory tree",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, err := commandContextFrom(cmd)
			if err != nil {
				return err
			}
			tree, err := agent.CastleTreeFor(ctx.store)
			if err != nil {
				return err
			}
			fmt.Fprintln(ctx.stdout, "Structure")
			for _, line := range tree {
				fmt.Fprintf(ctx.stdout, "  %s\n", line)
			}
			return nil
		},
	}
}

func newMemcastleContinuityCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "continuity",
		Short: "Manage Codex continuity notes in the memory castle",
	}
	cmd.AddCommand(newMemcastleContinuityImportCmd())
	return cmd
}

func newMemcastleContinuityImportCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "import [path]",
		Short: "Import a Codex compact note into the memory castle",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := commandContextFrom(cmd)
			if err != nil {
				return err
			}
			sourcePath := ""
			if len(args) > 0 {
				sourcePath = args[0]
			} else {
				sourcePath, err = latestCodexMemoryNote()
				if err != nil {
					return err
				}
			}
			result, err := agent.ImportContinuityNote(ctx.store, sourcePath)
			if err != nil {
				return err
			}
			fmt.Fprintf(ctx.stdout, "Imported continuity note:\n  Source: %s\n  Destination: %s\n", result.Source, result.Destination)
			return nil
		},
	}
}

func newMemcastleSearchCmd() *cobra.Command {
	var regex bool
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search the memory castle offline",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := commandContextFrom(cmd)
			if err != nil {
				return err
			}
			matches, err := agent.SearchCastle(ctx.store, joinArgs(args), regex)
			if err != nil {
				return err
			}
			for _, match := range matches {
				fmt.Fprintf(ctx.stdout, "%s:%d: %s\n", match.Path, match.Line, match.Excerpt)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&regex, "regex", false, "Treat the query as a case-insensitive regular expression")
	return cmd
}

func latestCodexMemoryNote() (string, error) {
	root, err := codexHomeDir()
	if err != nil {
		return "", err
	}
	memoriesDir := filepath.Join(root, "memories")
	var newestPath string
	var newestMod time.Time
	err = filepath.WalkDir(memoriesDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if errors.Is(walkErr, os.ErrNotExist) {
				return nil
			}
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if newestPath == "" || info.ModTime().After(newestMod) {
			newestPath = path
			newestMod = info.ModTime()
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("scan Codex memories: %w", err)
	}
	if newestPath == "" {
		return "", fmt.Errorf("no Codex memory notes found below %s", memoriesDir)
	}
	return newestPath, nil
}

func newMemcastleAskCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ask <question>",
		Short: "Ask Codex a question using only memory castle contents",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := commandContextFrom(cmd)
			if err != nil {
				return err
			}
			return runMemcastleAsk(ctx, joinArgs(args))
		},
	}
}

func newMemcastleCleanupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cleanup",
		Short: "Ask Jeff to sort the memory castle gatehouse",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, err := commandContextFrom(cmd)
			if err != nil {
				return err
			}
			return runMemcastleCleanup(ctx)
		},
	}
}

func runMemcastleAsk(ctx *commandContext, question string) error {
	cfg, err := ctx.loadConfig()
	if err != nil {
		return err
	}
	sessionID := configuredCodexSession(cfg)
	if strings.TrimSpace(sessionID) == "" {
		return errors.New("no active Jeff session - run 'jeff codex init' first")
	}
	codexBinary := cfg.CodexBinary
	if codexBinary == "" {
		codexBinary = "codex"
	}
	document, err := agent.CastleDocument(ctx.store)
	if err != nil {
		return err
	}
	prompt := fmt.Sprintf(`Answer the following question using only the memory castle sources below.

Rules:
- Do not use web search.
- Do not use model memory or prior chat memory.
- If the answer is not present in the sources, say exactly: "No memory castle match."
- Cite matching memory castle paths in the answer.

Question:
%s

Memory castle sources:

%s`, strings.TrimSpace(question), document)
	args := append(codexYoloArgs(), codexResumeArgs(sessionID, prompt)...)
	cmd := exec.Command(codexBinary, args...)
	cmd.Stdout = ctx.stdout
	cmd.Stderr = ctx.stderr
	cmd.Stdin = ctx.stdin
	return cmd.Run()
}

func runMemcastleCleanup(ctx *commandContext) error {
	cfg, err := ctx.loadConfig()
	if err != nil {
		return err
	}
	sessionID := configuredCodexSession(cfg)
	if strings.TrimSpace(sessionID) == "" {
		return errors.New("no active Jeff session - run 'jeff codex init' first")
	}
	codexBinary := cfg.CodexBinary
	if codexBinary == "" {
		codexBinary = "codex"
	}
	agentDir, err := ctx.store.AgentDir()
	if err != nil {
		return err
	}
	document, err := agent.CastleDocument(ctx.store)
	if err != nil {
		return err
	}
	prompt := fmt.Sprintf(`Clean up Jeff's memory castle gatehouse.

Scope:
- Work only below this memory castle root: %s
- Inspect gatehouse/ first, then inspect the rest of the castle.
- Decide whether the castle structure should change before moving information.
- Move durable facts from gatehouse/ into the best matching wing, floor, room, cabinet, or drawer.
- Create or update index.md files when structure changes.
- Preserve useful source references.
- Leave gatehouse/ empty except for index.md when everything has been sorted.
- Add a short logbook entry describing what changed.
- Do not write secrets, banking credentials, private personal data, or company-confidential data into source-controlled defaults or prompts.
- Do not use web search and do not run sudo.

Current memory castle snapshot:

%s`, agentDir, document)
	args := append(codexYoloArgs(), codexResumeArgs(sessionID, prompt)...)
	cmd := exec.Command(codexBinary, args...)
	cmd.Stdout = ctx.stdout
	cmd.Stderr = ctx.stderr
	cmd.Stdin = ctx.stdin
	return cmd.Run()
}
