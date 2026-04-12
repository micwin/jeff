package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"jeff/internal/version"
)

var (
	configDir  string
	showStatus bool
)

func main() {
	rootCmd := newRootCommand()
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	ver := version.Current()
	longDesc := fmt.Sprintf(`jeff - your friendly CLI-pal.

Version: %s

Commands:
  codex       Manage Codex integration (init, ask, tui)
  completion  Generate shell completions`, ver)

	cmd := &cobra.Command{
		Use:           "jeff",
		Short:         "Command-line assistant with optional Codex integration",
		Long:          longDesc,
		SilenceUsage:  false,
		SilenceErrors: false,
	}
	cmd.Version = ver
	cmd.SetVersionTemplate("jeff version: {{.Version}}\n")

	cmd.PersistentFlags().StringVar(&configDir, "config", "", "Override config directory (defaults to XDG config home)")
	cmd.PersistentFlags().BoolVar(&showStatus, "status", false, "Print Codex status header before answers")
	cmd.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
		return setCommandContext(cmd, configDir, showStatus)
	}

	cmd.AddCommand(
		newCodexCmd(),
		newCompletionCmd(cmd),
		newVersionCmd(),
	)

	return cmd
}
