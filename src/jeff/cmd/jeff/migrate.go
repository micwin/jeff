package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"jeff/internal/migrations"
)

func newMigrateCmd() *cobra.Command {
	var quiet bool

	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Run Jeff user-data migrations",
		Long: `Run pending Jeff user-data migrations.

Migrations are idempotent. They stamp config.json with a schema version and
move legacy durable user data out of the config directory when needed.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, err := commandContextFrom(cmd)
			if err != nil {
				return err
			}

			var out = ctx.stdout
			if quiet {
				out = nil
			}
			result, err := migrations.Run(ctx.store, out)
			if err != nil {
				return err
			}
			if !quiet && len(result.Moved) == 0 && len(result.Skipped) == 0 {
				fmt.Fprintln(ctx.stdout, "No migrations needed.")
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&quiet, "quiet", false, "Suppress migration status output")
	return cmd
}
