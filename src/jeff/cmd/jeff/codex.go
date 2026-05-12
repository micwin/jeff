package main

import (
	"github.com/spf13/cobra"
)

func newCodexCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "codex",
		Short: "Configure Jeff's Codex-backed session",
		Long:  "Configure the Codex session Jeff uses internally.",
	}

	cmd.AddCommand(
		newCodexInitCmd(),
		newCodexAskCmd(),
	)

	return cmd
}
