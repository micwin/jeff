package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"jeff/internal/sidecars"
)

func newVaultlineCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                "vl",
		Short:              "Run embedded Vaultline commands",
		Long:               "Run the Vaultline CLI embedded in this Jeff binary.",
		DisableFlagParsing: true,
		SilenceUsage:       true,
		Args:               cobra.ArbitraryArgs,
		ValidArgsFunction:  completeVaultline,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := commandContextFrom(cmd)
			if err != nil {
				return err
			}
			return runVaultline(cmd.Context(), ctx, args)
		},
	}
	return cmd
}

func runVaultline(runCtx context.Context, ctx *commandContext, args []string) error {
	err := sidecars.Run(runCtx, "vaultline", args, sidecars.Stdio{
		Stdin:  ctx.stdin,
		Stdout: ctx.stdout,
		Stderr: ctx.stderr,
	})
	if err == nil {
		return nil
	}
	if errors.Is(err, sidecars.ErrNotFound) {
		return fmt.Errorf("vaultline sidecar is not embedded in this Jeff build; run scripts/build.sh to build Jeff with sidecars")
	}
	return err
}

func completeVaultline(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	completeArgs := append([]string{"__complete", toComplete}, args...)
	out, err := sidecars.Output(cmd.Context(), "vaultline", completeArgs)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	items := sidecars.ParseLineCompletions(out, toComplete)
	directive := cobra.ShellCompDirectiveNoFileComp
	for _, item := range items {
		if strings.HasSuffix(item, ":") || strings.HasSuffix(item, ".") {
			directive |= cobra.ShellCompDirectiveNoSpace
			break
		}
	}
	return items, directive
}
