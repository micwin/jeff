// specialists.go exposes Jeff's generic specialist registry and call bridge.
// It keeps specialist identity data outside the transport implementation so
// future local LLM transports can use the same Jeff-facing command surface.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const (
	specialistTransportCodexCtl = "codex-ctl"
	specialistTransportEcho     = "echo"
)

type specialistConfig struct {
	Alias       string   `json:"alias"`
	DisplayName string   `json:"display_name,omitempty"`
	Description string   `json:"description,omitempty"`
	Transport   string   `json:"transport"`
	Target      string   `json:"target,omitempty"`
	CWD         string   `json:"cwd,omitempty"`
	Enabled     *bool    `json:"enabled,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

func newSpecialistsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "specialists",
		Short:   "List and call registered specialists",
		Aliases: []string{"sp"},
	}
	cmd.AddCommand(
		newSpecialistsListCmd(),
		newSpecialistsShowCmd(),
		newSpecialistsCallCmd(),
		newSpecialistsArchiveCmd(),
	)
	return cmd
}

func newSpecialistsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List registered specialists",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, err := commandContextFrom(cmd)
			if err != nil {
				return err
			}
			specialists, err := listSpecialists(ctx)
			if err != nil {
				return err
			}
			for _, specialist := range specialists {
				state := "enabled"
				if !specialist.isEnabled() {
					state = "disabled"
				}
				fmt.Fprintf(ctx.stdout, "%s\t%s\t%s\t%s\n", specialist.Alias, specialist.Transport, state, specialist.Description)
			}
			return nil
		},
	}
}

func newSpecialistsShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:               "show <alias>",
		Short:             "Show one specialist registry entry",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeSpecialistAliases,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := commandContextFrom(cmd)
			if err != nil {
				return err
			}
			specialist, err := loadSpecialist(ctx, args[0])
			if err != nil {
				return err
			}
			content, err := json.MarshalIndent(specialist, "", "  ")
			if err != nil {
				return err
			}
			fmt.Fprintln(ctx.stdout, string(content))
			return nil
		},
	}
}

func newSpecialistsCallCmd() *cobra.Command {
	var filePath string
	cmd := &cobra.Command{
		Use:               "call <alias> -- <message>",
		Short:             "Call one registered specialist",
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: completeSpecialistAliases,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := commandContextFrom(cmd)
			if err != nil {
				return err
			}
			message := strings.TrimSpace(joinArgs(args[1:]))
			if filePath != "" {
				data, err := os.ReadFile(filePath)
				if err != nil {
					return fmt.Errorf("read specialist request file: %w", err)
				}
				message = strings.TrimSpace(string(data))
			}
			if message == "" {
				return errors.New("specialist message must not be empty")
			}
			return callSpecialistByAlias(ctx, args[0], message)
		},
	}
	cmd.Flags().StringVar(&filePath, "file", "", "Read specialist request from a file")
	return cmd
}

func callSpecialistByAlias(ctx *commandContext, alias, message string) error {
	specialist, err := loadSpecialist(ctx, alias)
	if err != nil {
		return err
	}
	if !specialist.isEnabled() {
		return fmt.Errorf("specialist %q is disabled", alias)
	}
	output, err := callSpecialist(ctx, specialist, message)
	if err != nil {
		return err
	}
	if err := appendSpecialistCallLog(ctx, specialist.Alias, message, output); err != nil {
		return err
	}
	fmt.Fprintln(ctx.stdout, strings.TrimSpace(output))
	return nil
}

func callSpecialist(ctx *commandContext, specialist specialistConfig, message string) (string, error) {
	transport := specialist.Transport
	if transport == "" {
		transport = specialistTransportCodexCtl
	}
	prompt := buildSpecialistPrompt(specialist, message)
	switch transport {
	case specialistTransportCodexCtl:
		return callCodexCtlSpecialist(specialist, prompt)
	case specialistTransportEcho:
		return fmt.Sprintf("echo specialist %s\n%s", specialist.Alias, prompt), nil
	default:
		return "", fmt.Errorf("unsupported specialist transport %q", transport)
	}
}

func callCodexCtlSpecialist(specialist specialistConfig, prompt string) (string, error) {
	target := specialist.Target
	if target == "" {
		target = specialist.Alias
	}
	cmd := exec.Command("codex-ctl", "exec", target, "--", prompt)
	if specialist.CWD != "" {
		cmd.Dir = specialist.CWD
	}
	cmd.Env = append(os.Environ(), "JEFF_AGENT_ALIAS="+specialist.Alias)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return "", fmt.Errorf("codex-ctl exec %s failed: %w: %s", target, err, msg)
		}
		return "", fmt.Errorf("codex-ctl exec %s failed: %w", target, err)
	}
	return cleanCodexCtlOutput(stdout.String()), nil
}

func buildSpecialistPrompt(specialist specialistConfig, message string) string {
	lines := []string{
		"Goal: Jeff specialist call.",
		"Specialist alias: " + specialist.Alias,
		"Rules:",
		"- Identify yourself as a Jeff-called specialist when coordination context matters.",
		"- Follow the project-local `.agents/` harness if one exists in your active project.",
		"- Keep high semantic density.",
		"- Answer in English unless the task explicitly requires another language.",
		"Task:",
		message,
	}
	return strings.Join(lines, "\n")
}

func listSpecialists(ctx *commandContext) ([]specialistConfig, error) {
	root, err := ctx.store.SpecialistsDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read specialists: %w", err)
	}
	specialists := make([]specialistConfig, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		specialist, err := loadSpecialistFile(filepath.Join(root, entry.Name()))
		if err != nil {
			return nil, err
		}
		specialists = append(specialists, specialist)
	}
	sort.Slice(specialists, func(i, j int) bool {
		return specialists[i].Alias < specialists[j].Alias
	})
	return specialists, nil
}

func loadSpecialist(ctx *commandContext, alias string) (specialistConfig, error) {
	if !commandNamePattern.MatchString(alias) {
		return specialistConfig{}, fmt.Errorf("invalid specialist alias %q", alias)
	}
	root, err := ctx.store.SpecialistsDir()
	if err != nil {
		return specialistConfig{}, err
	}
	path := filepath.Join(root, alias+".json")
	specialist, err := loadSpecialistFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return specialistConfig{}, fmt.Errorf("specialist %q does not exist", alias)
		}
		return specialistConfig{}, err
	}
	if specialist.Alias != alias {
		return specialistConfig{}, fmt.Errorf("specialist file %q has alias %q", alias, specialist.Alias)
	}
	return specialist, nil
}

func loadSpecialistFile(path string) (specialistConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return specialistConfig{}, err
	}
	var specialist specialistConfig
	if err := json.Unmarshal(data, &specialist); err != nil {
		return specialistConfig{}, fmt.Errorf("parse specialist %s: %w", path, err)
	}
	if specialist.Alias == "" {
		return specialistConfig{}, fmt.Errorf("specialist %s has no alias", path)
	}
	if specialist.Transport == "" {
		specialist.Transport = specialistTransportCodexCtl
	}
	return specialist, nil
}

func appendSpecialistCallLog(ctx *commandContext, alias, message, output string) error {
	dataDir, err := ctx.store.DataDir()
	if err != nil {
		return err
	}
	logDir := filepath.Join(dataDir, "specialist-calls", time.Now().UTC().Format("2006-01-02"))
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return fmt.Errorf("create specialist log dir: %w", err)
	}
	stamp := time.Now().UTC().Format("150405")
	path := filepath.Join(logDir, stamp+"-"+alias+".md")
	content := fmt.Sprintf("# Specialist Call: %s\n\n## Request\n\n%s\n\n## Response\n\n%s\n", alias, strings.TrimSpace(message), strings.TrimSpace(output))
	return os.WriteFile(path, []byte(content), 0o600)
}

func (s specialistConfig) isEnabled() bool {
	return s.Enabled == nil || *s.Enabled
}

func completeSpecialistAliases(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	ctx, err := commandContextFrom(cmd)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	specialists, err := listSpecialists(ctx)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	names := make([]string, 0, len(specialists))
	for _, specialist := range specialists {
		if strings.HasPrefix(specialist.Alias, toComplete) {
			names = append(names, specialist.Alias)
		}
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}
