package main

import (
	"fmt"
	"os"
	"path/filepath"

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
	output := cmd.OutOrStdout()

	switch shell {
	case "bash":
		return rootCmd.GenBashCompletion(output)
	case "zsh":
		return rootCmd.GenZshCompletion(output)
	case "fish":
		dir, err := os.MkdirTemp("", "jeff-completion")
		if err != nil {
			return err
		}
		defer os.RemoveAll(dir)

		path := filepath.Join(dir, "jeff.fish")
		if err := rootCmd.GenFishCompletionFile(path, true); err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		_, err = output.Write(data)
		return err
	default:
		return fmt.Errorf("unbekannte shell %q", shell)
	}
}
