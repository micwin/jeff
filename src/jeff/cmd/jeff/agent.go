package main

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"jeff/internal/agent"
)

func newAgentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Manage Jeff's persistent personal agent memory",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_ = cmd.Help()
			return errors.New("missing agent subcommand")
		},
	}
	cmd.AddCommand(newAgentStatusCmd(), newAgentRememberCmd())
	return cmd
}

func newAgentStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Print Jeff agent storage paths",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, err := commandContextFrom(cmd)
			if err != nil {
				return err
			}
			dataDir, err := ctx.store.DataDir()
			if err != nil {
				return err
			}
			cacheDir, err := ctx.store.CacheDir()
			if err != nil {
				return err
			}
			agentDir, err := ctx.store.AgentDir()
			if err != nil {
				return err
			}
			vaultlineDir, err := ctx.store.VaultlineDir()
			if err != nil {
				return err
			}
			fmt.Fprintf(ctx.stdout, "config: %s\n", ctx.store.Dir())
			fmt.Fprintf(ctx.stdout, "data: %s\n", dataDir)
			fmt.Fprintf(ctx.stdout, "cache: %s\n", cacheDir)
			fmt.Fprintf(ctx.stdout, "memcastle: %s\n", agentDir)
			fmt.Fprintf(ctx.stdout, "skills: %s\n", filepath.Join(dataDir, "skills"))
			fmt.Fprintf(ctx.stdout, "commands: %s\n", filepath.Join(dataDir, "commands"))
			fmt.Fprintf(ctx.stdout, "reports: %s\n", filepath.Join(dataDir, "reports"))
			fmt.Fprintf(ctx.stdout, "vaultline: %s\n", vaultlineDir)
			return nil
		},
	}
}

func newAgentRememberCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remember <text>",
		Short: "Append a logbook entry to the memory castle",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := commandContextFrom(cmd)
			if err != nil {
				return err
			}
			path, err := agent.RecordLog(ctx.store, joinArgs(args))
			if err != nil {
				return err
			}
			fmt.Fprintf(ctx.stdout, "Remembered in %s\n", path)
			return nil
		},
	}
}
