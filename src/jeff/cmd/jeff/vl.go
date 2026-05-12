package main

import (
	"context"
	"errors"
	"fmt"
	"io"
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
	args, err := normalizeVaultlineArgs(ctx, args)
	if err != nil {
		return err
	}
	if vaultlineRawPassthrough(args) {
		return runVaultlineSidecar(runCtx, args, ctx)
	}
	if len(args) > 0 && args[0] == "daemon-stop" {
		return stopManagedVaultline(runCtx, ctx)
	}
	managedArgs, err := managedVaultlineArgs(runCtx, ctx, args)
	if err != nil {
		return err
	}
	return runVaultlineSidecar(runCtx, managedArgs, ctx)
}

func runVaultlineSidecar(runCtx context.Context, args []string, ctx *commandContext) error {
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
	ctx, err := commandContextFrom(cmd)
	if err != nil {
		store, storeErr := cachedStore(configDir)
		if storeErr != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		ctx = &commandContext{
			stdin:  nil,
			stdout: io.Discard,
			stderr: io.Discard,
			store:  store,
		}
	}
	args, err = normalizeVaultlineArgs(ctx, args)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	completeArgs := []string{"__complete", toComplete}
	for _, arg := range args {
		if arg != "" {
			completeArgs = append(completeArgs, arg)
		}
	}
	managedArgs, err := managedVaultlineArgs(cmd.Context(), ctx, completeArgs)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	out, err := sidecars.Output(cmd.Context(), "vaultline", managedArgs)
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

func vaultlineRawPassthrough(args []string) bool {
	if len(args) == 0 {
		return true
	}
	switch args[0] {
	case "version", "--version", "help", "--help", "-h", "completion":
		return true
	default:
		return false
	}
}

func normalizeVaultlineArgs(ctx *commandContext, args []string) ([]string, error) {
	normalized := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--config":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--config requires a value")
			}
			store, err := cachedStore(args[i+1])
			if err != nil {
				return nil, err
			}
			ctx.store = store
			i++
		case strings.HasPrefix(arg, "--config="):
			store, err := cachedStore(strings.TrimPrefix(arg, "--config="))
			if err != nil {
				return nil, err
			}
			ctx.store = store
		default:
			normalized = append(normalized, arg)
		}
	}
	return normalized, nil
}
