package main

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
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
		newTmplRenderCmd(),
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
		Use:               "validate <name>",
		Short:             "Validate a template package (main.tmpl + syntax)",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeTemplateNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := commandContextFrom(cmd)
			if err != nil {
				return err
			}
			_, tmplName, err := resolveTemplateDir(ctx, args[0])
			if err != nil {
				return err
			}
			if _, err := parseRuntimeTemplates(ctx, tmplName); err != nil {
				return fmt.Errorf("template parse failed: %w", err)
			}
			fmt.Fprintf(ctx.stdout, "Template %q is valid.\n", args[0])
			return nil
		},
	}
	return cmd
}

func newTmplRenderCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "render <name>",
		Short:             "Render template main.tmpl with environment data",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeTemplateNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := commandContextFrom(cmd)
			if err != nil {
				return err
			}
			_, tmplName, err := resolveTemplateDir(ctx, args[0])
			if err != nil {
				return err
			}
			tpl, err := parseRuntimeTemplates(ctx, tmplName)
			if err != nil {
				return fmt.Errorf("template parse failed: %w", err)
			}
			data := map[string]any{"Env": envMap()}
			if err := tpl.ExecuteTemplate(ctx.stdout, runtimeTemplateName(tmplName, "main.tmpl"), data); err != nil {
				return fmt.Errorf("template render failed: %w", err)
			}
			return nil
		},
	}
	return cmd
}

func completeTemplateNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
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
}

func parseRuntimeTemplates(ctx *commandContext, rootTemplate string) (*template.Template, error) {
	names, err := listTemplateNames(ctx)
	if err != nil {
		return nil, err
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("no templates found")
	}

	var tpl *template.Template
	tpl = template.New("runtime-root").Option("missingkey=error")
	tpl = tpl.Funcs(template.FuncMap{
		"include": func(name string, data any) (string, error) {
			resolved, err := resolveRuntimeReference(rootTemplate, name)
			if err != nil {
				return "", err
			}
			var b bytes.Buffer
			if err := tpl.ExecuteTemplate(&b, resolved, data); err != nil {
				return "", err
			}
			return b.String(), nil
		},
	})

	for _, pkg := range names {
		dir := filepath.Join(ctx.store.Dir(), "templates", filepath.FromSlash(pkg))
		if err := loadTemplatePackageFiles(tpl, pkg, dir); err != nil {
			return nil, err
		}
	}

	return tpl, nil
}

var defineOrTemplateRe = regexp.MustCompile(`\{\{\s*(define|template)\s+"([^"]+)"`)

func loadTemplatePackageFiles(tpl *template.Template, packageName, dir string) error {
	pattern := filepath.Join(dir, "*.tmpl")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no .tmpl files found in %s", dir)
	}
	for _, file := range files {
		relFile := filepath.Base(file)
		nsName := runtimeTemplateName(packageName, relFile)
		content, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		rewritten := defineOrTemplateRe.ReplaceAllStringFunc(string(content), func(match string) string {
			parts := defineOrTemplateRe.FindStringSubmatch(match)
			if len(parts) != 3 {
				return match
			}
			resolved, err := resolveRuntimeReference(packageName, parts[2])
			if err != nil {
				return match
			}
			return strings.Replace(match, `"`+parts[2]+`"`, `"`+resolved+`"`, 1)
		})
		if _, err := tpl.New(nsName).Parse(rewritten); err != nil {
			return err
		}
	}
	return nil
}

func runtimeTemplateName(packageName, fileName string) string {
	return "runtime/" + strings.Trim(packageName, "/") + "/" + strings.Trim(fileName, "/")
}

func resolveRuntimeReference(currentPackage, ref string) (string, error) {
	if ref == "" {
		return "", fmt.Errorf("template reference is empty")
	}
	if strings.HasPrefix(ref, "runtime/") {
		return ref, nil
	}
	clean := filepath.ToSlash(filepath.Clean(ref))
	if strings.HasPrefix(clean, "../") || clean == ".." || strings.Contains(clean, "/../") {
		return "", fmt.Errorf("invalid include reference %q", ref)
	}
	if strings.HasPrefix(clean, "/") {
		return "", fmt.Errorf("absolute include reference not allowed: %q", ref)
	}
	if strings.Contains(clean, "/") {
		if strings.HasSuffix(clean, ".tmpl") {
			return "runtime/" + clean, nil
		}
		return "runtime/" + clean + "/main.tmpl", nil
	}
	if strings.HasSuffix(clean, ".tmpl") {
		return runtimeTemplateName(currentPackage, clean), nil
	}
	return "runtime/" + currentPackage + "/" + clean + "/main.tmpl", nil
}

func envMap() map[string]string {
	env := make(map[string]string)
	for _, pair := range os.Environ() {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			continue
		}
		env[parts[0]] = parts[1]
	}
	return env
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

func resolveTemplateDir(ctx *commandContext, name string) (string, string, error) {
	if name == "" {
		return "", "", fmt.Errorf("template name is required")
	}
	if strings.HasPrefix(name, "/") {
		return "", "", fmt.Errorf("template name must be relative")
	}
	clean := filepath.Clean(name)
	if clean == "." || clean == "" {
		return "", "", fmt.Errorf("template name is required")
	}
	if strings.HasPrefix(clean, "..") || strings.Contains(clean, string(filepath.Separator)+"..") {
		return "", "", fmt.Errorf("template name must not contain ..")
	}
	tmplDir := filepath.Join(ctx.store.Dir(), "templates", filepath.FromSlash(clean))
	if _, err := os.Stat(filepath.Join(tmplDir, "main.tmpl")); err != nil {
		if os.IsNotExist(err) {
			return "", "", fmt.Errorf("template %q not found (expected %s)", name, filepath.Join(tmplDir, "main.tmpl"))
		}
		return "", "", err
	}
	return tmplDir, filepath.ToSlash(clean), nil
}
