package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"jeff/internal/agent"
)

func newInitCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize Jeff data defaults",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, err := commandContextFrom(cmd)
			if err != nil {
				return err
			}
			result, err := agent.Bootstrap(ctx.store, force)
			if err != nil {
				return err
			}
			fmt.Fprintf(ctx.stdout, "Jeff initialized: %d written, %d skipped.\n", len(result.Created), len(result.Skipped))
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing Jeff default files")
	return cmd
}
