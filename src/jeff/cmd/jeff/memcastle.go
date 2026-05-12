package main

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"jeff/internal/agent"
)

func newMemcastleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "memcastle",
		Aliases: []string{"mc"},
		Short:   "Inspect Jeff's persistent memory castle",
	}
	cmd.AddCommand(newMemcastleInfoCmd(), newMemcastleSearchCmd(), newMemcastleAskCmd(), newMemcastleCleanupCmd())
	return cmd
}

func newMemcastleInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "Print memory castle metadata and structure",
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
			fmt.Fprintf(ctx.stdout, "  Drawers:  %d\n\n", info.Drawers)
			fmt.Fprintln(ctx.stdout, "Structure")
			for _, line := range info.Tree {
				fmt.Fprintf(ctx.stdout, "  %s\n", line)
			}
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
	cmd := exec.Command(codexBinary, codexResumeArgs(sessionID, prompt)...)
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
	args := append([]string{"--sandbox", "danger-full-access"}, codexResumeArgs(sessionID, prompt)...)
	cmd := exec.Command(codexBinary, args...)
	cmd.Stdout = ctx.stdout
	cmd.Stderr = ctx.stderr
	cmd.Stdin = ctx.stdin
	return cmd.Run()
}
