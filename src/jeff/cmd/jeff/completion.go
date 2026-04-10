package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newCompletionCmd(rootCmd *cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion",
		Short: "Shell-Completions über Cobra erzeugen",
		Long: `Generiert Shell-Completion Skripte.

Beispiele:
  jeff completion bash > /etc/bash_completion.d/jeff
  jeff completion zsh  > "${fpath[1]}/_jeff"`,
	}

	shells := []string{"bash", "zsh", "fish"}
	for _, shell := range shells {
		s := shell
		cmd.AddCommand(&cobra.Command{
			Use:   s,
			Short: fmt.Sprintf("%s-completion erzeugen", s),
			RunE: func(cmd *cobra.Command, args []string) error {
				return runCompletion(rootCmd, s, cmd)
			},
		})
	}

	return cmd
}

func runCompletion(rootCmd *cobra.Command, shell string, cmd *cobra.Command) error {
	switch shell {
	case "bash":
		return rootCmd.GenBashCompletion(cmd.OutOrStdout())
	case "zsh":
		return rootCmd.GenZshCompletion(cmd.OutOrStdout())
	case "fish":
		return rootCmd.GenFishCompletion(cmd.OutOrStdout(), true)
	default:
		return fmt.Errorf("unbekannte shell %q", shell)
	}
}
