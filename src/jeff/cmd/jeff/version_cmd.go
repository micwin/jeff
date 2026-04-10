package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"jeff/internal/version"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the current jeff version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintln(cmd.OutOrStdout(), version.Current())
		},
	}
}
