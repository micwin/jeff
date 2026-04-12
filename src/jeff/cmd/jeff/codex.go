package main

import (
	"github.com/spf13/cobra"
)

func newCodexCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "codex",
		Short: "Codex integration commands",
		Long:  "Manage Codex sessions, ask questions, or jump into the full Codex TUI.",
	}

	cmd.AddCommand(
		newCodexInitCmd(),
		newCodexAskCmd(),
		newCodexTuiCmd(),
	)

	return cmd
}
