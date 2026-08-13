package main

import (
	"errors"
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
		var partial archivePartialError
		if errors.As(err, &partial) {
			os.Exit(archivePartialExitCode)
		}
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	ver := version.Current()
	longDesc := fmt.Sprintf(`jeff - your friendly CLI-pal.

Version: %s

Commands:
  chat        Start the interactive Jeff chat
  init        Initialize Jeff data defaults
  codex       Configure Jeff's Codex-backed session
  tmux        Launch the Jeff tmux overlay (requires tmux)
  menu        Manage Jeff's interactive shortcut menu
  vl          Run embedded Vaultline commands
  tmpl        Manage Jeff templates
  migrate     Run Jeff user-data migrations
  agent       Manage Jeff's persistent personal agent memory
  memcastle   Inspect Jeff's persistent memory castle (alias: mc)
  execute     Execute stored Jeff Bash commands
  room        Coordinate deterministic specialist rooms
  specialists List and call registered specialists
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
	defaultHelp := cmd.HelpTemplate()
	cmd.SetHelpTemplate("{{if .Version}}Jeff CLI {{.Version}}\n\n{{end}}" + defaultHelp)

	cmd.PersistentFlags().StringVar(&configDir, "config", "", "Override config directory (defaults to XDG config home)")
	cmd.PersistentFlags().BoolVar(&showStatus, "status", false, "Print Codex status header before answers")
	cmd.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
		return setCommandContext(cmd, configDir, showStatus)
	}

	cmd.AddCommand(
		newChatCmd(),
		newInitCmd(),
		newCodexCmd(),
		newTmuxCmd(),
		newMenuCmd(),
		newVaultlineCmd(),
		newTmplCmd(),
		newMigrateCmd(),
		newAgentCmd(),
		newMemcastleCmd(),
		newExecuteCmd(),
		newRoomCmd(),
		newSpecialistsCmd(),
		newCompletionCmd(cmd),
		newVersionCmd(),
	)

	return cmd
}
