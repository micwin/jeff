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
	longDesc := fmt.Sprintf(`jeff – Kommandozeilen-Assistent.

Version: %s

Kommandos:
  init        Session-ID setzen oder letzte Session wiederverwenden
  ask         Eine Frage stellen (Optionen: --session, --codex-binary, --timeout, --show-token-cost; global: --status)
  chat        Interaktive Codex-Session (nutzt aktuelle Session-ID)
  completion  Shell-Completions erzeugen`, ver)

	cmd := &cobra.Command{
		Use:   "jeff",
		Short: "Kommandozeilen-Assistent mit Codex-Anbindung",
		Long:  longDesc,
		SilenceUsage:  false,
		SilenceErrors: false,
	}
	cmd.Version = ver
	cmd.SetVersionTemplate("jeff version: {{.Version}}\n")

	cmd.PersistentFlags().StringVar(&configDir, "config", "", "Konfigurationsverzeichnis (Standard: XDG)")
	cmd.PersistentFlags().BoolVar(&showStatus, "status", false, "Statusblock vor Antworten anzeigen")
	cmd.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
		return setCommandContext(cmd, configDir, showStatus)
	}

	cmd.AddCommand(
		newInitCmd(),
		newAskCmd(),
		newChatCmd(),
		newCompletionCmd(cmd),
		newVersionCmd(),
	)

	return cmd
}
