package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/spf13/cobra"
)

func newTmplCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tmpl",
		Short: "Manage Jeff text templates",
		Long: `Work with Jeff templates stored below ~/.config/jeff/templates.

Each template is a directory with a required main.tmpl file.
Nested names are supported, e.g. finances/report-monthly.`,
	}

	cmd.AddCommand(
		newTmplListCmd(),
		newTmplValidateCmd(),
	)

	return cmd
}

func newTmplListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available template names",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := commandContextFrom(cmd)
			if err != nil {
				return err
			}
			names, err := listTemplateNames(ctx)
			if err != nil {
				return err
			}
			if len(names) == 0 {
				fmt.Fprintln(ctx.stdout, "No templates found.")
				return nil
			}
			for _, name := range names {
				fmt.Fprintln(ctx.stdout, name)
			}
			return nil
		},
	}
	return cmd
}

func newTmplValidateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate <name>",
		Short: "Validate a template package (main.tmpl + syntax)",
		Args:  cobra.ExactArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			ctx, err := commandContextFrom(cmd)
			if err != nil {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			names, err := listTemplateNames(ctx)
			if err != nil {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			filtered := make([]string, 0, len(names))
			for _, n := range names {
				if strings.HasPrefix(n, toComplete) {
					filtered = append(filtered, n)
				}
			}
			return filtered, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := commandContextFrom(cmd)
			if err != nil {
				return err
			}
			tmplDir, err := resolveTemplateDir(ctx, args[0])
			if err != nil {
				return err
			}
			pattern := filepath.Join(tmplDir, "*.tmpl")
			matches, err := filepath.Glob(pattern)
			if err != nil {
				return fmt.Errorf("glob templates: %w", err)
			}
			if len(matches) == 0 {
				return fmt.Errorf("no .tmpl files found in %s", tmplDir)
			}
			mainFile := filepath.Join(tmplDir, "main.tmpl")
			if _, err := os.Stat(mainFile); err != nil {
				return fmt.Errorf("missing main.tmpl in %s", tmplDir)
			}
			_, err = template.ParseFiles(matches...)
			if err != nil {
				return fmt.Errorf("template parse failed: %w", err)
			}
			fmt.Fprintf(ctx.stdout, "Template %q is valid.\n", args[0])
			return nil
		},
	}
	return cmd
}

func listTemplateNames(ctx *commandContext) ([]string, error) {
	root := filepath.Join(ctx.store.Dir(), "templates")
	if _, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	result := make([]string, 0)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if d.Name() != "main.tmpl" {
			return nil
		}
		dir := filepath.Dir(path)
		rel, err := filepath.Rel(root, dir)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "." || rel == "" {
			return nil
		}
		result = append(result, rel)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(result)
	return result, nil
}

func resolveTemplateDir(ctx *commandContext, name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("template name is required")
	}
	if strings.HasPrefix(name, "/") {
		return "", fmt.Errorf("template name must be relative")
	}
	clean := filepath.Clean(name)
	if clean == "." || clean == "" {
		return "", fmt.Errorf("template name is required")
	}
	if strings.HasPrefix(clean, "..") || strings.Contains(clean, string(filepath.Separator)+"..") {
		return "", fmt.Errorf("template name must not contain ..")
	}
	tmplDir := filepath.Join(ctx.store.Dir(), "templates", filepath.FromSlash(clean))
	if _, err := os.Stat(filepath.Join(tmplDir, "main.tmpl")); err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("template %q not found (expected %s)", name, filepath.Join(tmplDir, "main.tmpl"))
		}
		return "", err
	}
	return tmplDir, nil
}
