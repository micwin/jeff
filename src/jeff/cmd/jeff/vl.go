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
	args = defaultVaultlineSecretStore(args)
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
	case "version", "--version", "help", "--help", "-h", "completion", "daemon", "daemon-stop":
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

func defaultVaultlineSecretStore(args []string) []string {
	if len(args) < 2 || args[0] != "secret" {
		return defaultVaultlineFromSecretFlag(args)
	}
	out := append([]string(nil), args...)
	switch args[1] {
	case "set", "get", "delete", "delete-prefix", "glob":
		defaultFirstSecretArg(out, 2)
	case "copy", "move":
		defaultFirstSecretArg(out, 2)
		defaultFirstSecretArg(out, 3)
	}
	return defaultVaultlineSecretFlags(out)
}

func defaultFirstSecretArg(args []string, start int) {
	for i := start; i < len(args); i++ {
		if strings.HasPrefix(args[i], "-") {
			if vaultlineFlagTakesValue(args[i]) {
				i++
			}
			continue
		}
		args[i] = defaultVaultlineStore(args[i])
		return
	}
}

func defaultVaultlineFromSecretFlag(args []string) []string {
	out := append([]string(nil), args...)
	return defaultVaultlineFromSecretFlagInPlace(out)
}

func defaultVaultlineSecretFlags(args []string) []string {
	out := append([]string(nil), args...)
	out = defaultVaultlineFromSecretFlagInPlace(out)
	for i := 0; i < len(out); i++ {
		switch {
		case out[i] == "--name" && i+1 < len(out):
			out[i+1] = defaultVaultlineStore(out[i+1])
			i++
		case strings.HasPrefix(out[i], "--name="):
			value := strings.TrimPrefix(out[i], "--name=")
			out[i] = "--name=" + defaultVaultlineStore(value)
		}
	}
	return out
}

func defaultVaultlineFromSecretFlagInPlace(out []string) []string {
	for i := 0; i < len(out); i++ {
		switch {
		case out[i] == "--from-secret" && i+1 < len(out):
			out[i+1] = defaultVaultlineStore(out[i+1])
			i++
		case strings.HasPrefix(out[i], "--from-secret="):
			value := strings.TrimPrefix(out[i], "--from-secret=")
			out[i] = "--from-secret=" + defaultVaultlineStore(value)
		}
	}
	return out
}

func defaultVaultlineStore(value string) string {
	if value == "" || strings.Contains(value, ":") || strings.HasPrefix(value, "-") {
		return value
	}
	return "jeff:" + value
}

func vaultlineFlagTakesValue(flag string) bool {
	if strings.Contains(flag, "=") {
		return false
	}
	switch flag {
	case "--value", "--file", "--out", "--output", "--name":
		return true
	default:
		return false
	}
}
