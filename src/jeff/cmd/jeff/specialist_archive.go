// specialist_archive.go preserves specialist identities, local transcripts, and
// non-reproducible repository state for account-independent continuity.
package main

import (
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
)

const archivePartialExitCode = 2

type archiveRegistry struct {
	GeneratedAt  string                    `json:"generated_at"`
	ShortAliases map[string]string         `json:"short_aliases,omitempty"`
	Specialists  []archiveSpecialistRecord `json:"specialists"`
}

type archiveSpecialistRecord struct {
	Alias        string         `json:"alias"`
	Description  string         `json:"description,omitempty"`
	Enabled      bool           `json:"enabled"`
	SessionKey   string         `json:"session_key,omitempty"`
	SessionID    string         `json:"session_id,omitempty"`
	Policy       map[string]any `json:"policy"`
	Transcript   string         `json:"transcript,omitempty"`
	Repositories []string       `json:"repositories,omitempty"`
	Error        string         `json:"error,omitempty"`
}

type archiveRepositoryRecord struct {
	ID       string `json:"id"`
	Root     string `json:"root"`
	Remote   string `json:"remote,omitempty"`
	Branch   string `json:"branch,omitempty"`
	Head     string `json:"head"`
	Dirty    bool   `json:"dirty"`
	RemoteOK bool   `json:"remote_reproducible"`
	Snapshot string `json:"snapshot,omitempty"`
}

type archiveStatus struct {
	GeneratedAt        string   `json:"generated_at"`
	Specialists        int      `json:"specialists"`
	Sessions           int      `json:"sessions"`
	Repositories       int      `json:"repositories"`
	MemoryCastleDirty  bool     `json:"memory_castle_git_dirty"`
	MissingTranscripts []string `json:"missing_transcripts,omitempty"`
	Errors             []string `json:"errors,omitempty"`
}

type archiveRefreshOptions struct {
	Briefs       bool
	CodexHome    string
	ResumeConfig string
}

func newSpecialistsArchiveCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "archive", Short: "Archive specialist continuity data"}
	cmd.AddCommand(
		newSpecialistsArchiveRefreshCmd(),
		newSpecialistsArchiveStatusCmd(),
		newSpecialistsArchiveSearchCmd(),
		newSpecialistsArchiveSuccessorCmd(),
	)
	return cmd
}

func newSpecialistsArchiveRefreshCmd() *cobra.Command {
	var options archiveRefreshOptions
	cmd := &cobra.Command{
		Use:   "refresh",
		Short: "Refresh specialist identities, transcripts, and repository state",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, err := commandContextFrom(cmd)
			if err != nil {
				return err
			}
			status, err := refreshSpecialistArchive(ctx, options)
			if err != nil {
				return err
			}
			fmt.Fprintf(ctx.stdout, "Archived %d specialist(s), %d session(s), %d repository state(s).\n", status.Specialists, status.Sessions, status.Repositories)
			if len(status.Errors) > 0 || len(status.MissingTranscripts) > 0 {
				return archivePartialError{status: status}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&options.Briefs, "briefs", false, "Ask enabled specialists for current continuity briefs")
	cmd.Flags().StringVar(&options.CodexHome, "codex-home", "", "Override Codex home (defaults to CODEX_HOME or ~/.codex)")
	cmd.Flags().StringVar(&options.ResumeConfig, "resume-config", "", "Override codex-resume session config")
	_ = cmd.RegisterFlagCompletionFunc("codex-home", completeDirectories)
	_ = cmd.RegisterFlagCompletionFunc("resume-config", completeFiles)
	return cmd
}

type archivePartialError struct{ status archiveStatus }

func (e archivePartialError) Error() string {
	return fmt.Sprintf("specialist archive is partial: %d error(s), %d missing transcript(s); see status.json", len(e.status.Errors), len(e.status.MissingTranscripts))
}

func newSpecialistsArchiveStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use: "status", Short: "Show specialist archive completeness", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, err := commandContextFrom(cmd)
			if err != nil {
				return err
			}
			root, err := specialistArchiveRoot(ctx)
			if err != nil {
				return err
			}
			data, err := os.ReadFile(filepath.Join(root, "status.json"))
			if err != nil {
				return fmt.Errorf("read specialist archive status: %w", err)
			}
			fmt.Fprintln(ctx.stdout, string(data))
			return nil
		},
	}
}

func newSpecialistsArchiveSuccessorCmd() *cobra.Command {
	return &cobra.Command{
		Use: "successor", Short: "Print the successor recovery prompt", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, err := commandContextFrom(cmd)
			if err != nil {
				return err
			}
			root, err := specialistArchiveRoot(ctx)
			if err != nil {
				return err
			}
			data, err := os.ReadFile(filepath.Join(root, "successor-prompt.md"))
			if err != nil {
				return fmt.Errorf("read successor prompt: %w", err)
			}
			fmt.Fprint(ctx.stdout, string(data))
			return nil
		},
	}
}

func newSpecialistsArchiveSearchCmd() *cobra.Command {
	var alias, scope string
	var regexMode bool
	var limit int
	cmd := &cobra.Command{
		Use: "search <query>", Short: "Search archived specialist transcripts offline", Args: cobra.ExactArgs(1),
		ValidArgsFunction: func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := commandContextFrom(cmd)
			if err != nil {
				return err
			}
			return searchSpecialistArchive(ctx, args[0], alias, scope, regexMode, limit)
		},
	}
	cmd.Flags().StringVar(&alias, "alias", "", "Restrict search to one specialist alias")
	cmd.Flags().StringVar(&scope, "scope", "messages", "Search messages or all transcript events")
	cmd.Flags().BoolVar(&regexMode, "regex", false, "Interpret query as a regular expression")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum matching events")
	_ = cmd.RegisterFlagCompletionFunc("alias", completeArchivedSpecialistAliases)
	_ = cmd.RegisterFlagCompletionFunc("scope", cobra.FixedCompletions([]string{"messages", "all"}, cobra.ShellCompDirectiveNoFileComp))
	return cmd
}

func completeDirectories(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return nil, cobra.ShellCompDirectiveFilterDirs
}

func completeFiles(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return nil, cobra.ShellCompDirectiveDefault
}

func completeArchivedSpecialistAliases(cmd *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	ctx, err := commandContextFrom(cmd)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	root, err := specialistArchiveRoot(ctx)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	data, err := os.ReadFile(filepath.Join(root, "registry.json"))
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	var registry archiveRegistry
	if json.Unmarshal(data, &registry) != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	aliases := make([]string, 0, len(registry.Specialists))
	for _, specialist := range registry.Specialists {
		if strings.HasPrefix(specialist.Alias, toComplete) {
			aliases = append(aliases, specialist.Alias)
		}
	}
	sort.Strings(aliases)
	return aliases, cobra.ShellCompDirectiveNoFileComp
}

func refreshSpecialistArchive(ctx *commandContext, options archiveRefreshOptions) (archiveStatus, error) {
	root, err := specialistArchiveRoot(ctx)
	if err != nil {
		return archiveStatus{}, err
	}
	privateRoot := filepath.Join(root, "private")
	for _, dir := range []string{root, filepath.Join(privateRoot, "sessions"), filepath.Join(privateRoot, "repositories")} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return archiveStatus{}, err
		}
	}
	if err := ensureArchiveSupportFiles(root); err != nil {
		return archiveStatus{}, err
	}

	aliases, err := codexResumeAliases(options.ResumeConfig)
	if err != nil {
		return archiveStatus{}, err
	}
	codexHome, err := resolveCodexHome(options.CodexHome)
	if err != nil {
		return archiveStatus{}, err
	}

	registry := archiveRegistry{GeneratedAt: time.Now().Format(time.RFC3339), ShortAliases: configuredShortAliases(options.ResumeConfig)}
	status := archiveStatus{GeneratedAt: registry.GeneratedAt}
	memoryCastleRoot := filepath.Dir(filepath.Dir(root))
	if memoryStatus, statusErr := gitOutput(memoryCastleRoot, "status", "--porcelain=v1", "--untracked-files=normal"); statusErr == nil {
		status.MemoryCastleDirty = memoryStatus != ""
	}
	repositories := map[string]archiveRepositoryRecord{}
	sessions := map[string]string{}
	for _, alias := range aliases {
		record := archiveSpecialistRecord{Alias: alias, Enabled: true, Policy: map[string]any{}}
		policyText, policyErr := codexResumePolicy(alias, options.ResumeConfig)
		if policyErr != nil {
			record.Error = policyErr.Error()
			status.Errors = append(status.Errors, alias+": "+record.Error)
			registry.Specialists = append(registry.Specialists, record)
			continue
		}
		record.Policy = parsePolicy(policyText)
		record.Description = stringPolicy(record.Policy, "description")
		record.SessionKey = stringPolicy(record.Policy, "session_key")
		if enabled, ok := record.Policy["enabled"].(bool); ok {
			record.Enabled = enabled
		}
		record.SessionID, err = resolveSessionID(ctx, record.SessionKey)
		if err != nil {
			record.Error = err.Error()
			status.Errors = append(status.Errors, alias+": "+record.Error)
		} else if record.SessionID != "" {
			if transcript, ok := sessions[record.SessionID]; ok {
				record.Transcript = transcript
			} else {
				transcript, archiveErr := archiveTranscript(codexHome, privateRoot, record.SessionID)
				if archiveErr != nil {
					status.MissingTranscripts = append(status.MissingTranscripts, alias)
				} else {
					sessions[record.SessionID] = transcript
					record.Transcript = transcript
				}
			}
		}
		if cwd := stringPolicy(record.Policy, "cd"); cwd != "" {
			if pathWithin(cwd, memoryCastleRoot) {
				registry.Specialists = append(registry.Specialists, record)
				continue
			}
			repos, repoErr := archiveRepositories(cwd, privateRoot)
			if repoErr == nil {
				for _, repo := range repos {
					repositories[repo.ID] = repo
					record.Repositories = append(record.Repositories, repo.ID)
				}
			} else {
				status.Errors = append(status.Errors, alias+" repository: "+repoErr.Error())
			}
		}
		registry.Specialists = append(registry.Specialists, record)
	}

	if options.Briefs {
		briefErrors := refreshContinuityBriefs(root, registry.Specialists)
		status.Errors = append(status.Errors, briefErrors...)
	}
	status.Specialists = len(registry.Specialists)
	status.Sessions = len(sessions)
	status.Repositories = len(repositories)
	if err := installArchiveSkillLink(root, codexHome); err != nil {
		status.Errors = append(status.Errors, err.Error())
	}
	if err := writeJSON(filepath.Join(root, "registry.json"), registry); err != nil {
		return archiveStatus{}, err
	}
	repoList := make([]archiveRepositoryRecord, 0, len(repositories))
	for _, repo := range repositories {
		repoList = append(repoList, repo)
	}
	sort.Slice(repoList, func(i, j int) bool { return repoList[i].Root < repoList[j].Root })
	if err := pruneArchivePayloads(root, registry.Specialists, repoList); err != nil {
		return archiveStatus{}, err
	}
	if err := writeJSON(filepath.Join(root, "repositories.json"), repoList); err != nil {
		return archiveStatus{}, err
	}
	if err := writeJSON(filepath.Join(root, "status.json"), status); err != nil {
		return archiveStatus{}, err
	}
	if err := writeSpecialistRooms(ctx, registry.Specialists); err != nil {
		return archiveStatus{}, err
	}
	return status, nil
}

func specialistArchiveRoot(ctx *commandContext) (string, error) {
	agentDir, err := ctx.store.AgentDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(agentDir, "codex", "specialist-archive"), nil
}

func pathWithin(path, root string) bool {
	path, pathErr := filepath.Abs(path)
	root, rootErr := filepath.Abs(root)
	if pathErr != nil || rootErr != nil {
		return false
	}
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func resolveCodexHome(override string) (string, error) {
	if override != "" {
		return filepath.Abs(override)
	}
	if value := os.Getenv("CODEX_HOME"); value != "" {
		return filepath.Abs(value)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".codex"), nil
}

func archiveCodexResumeArgs(config string, args ...string) []string {
	if config == "" {
		return args
	}
	return append([]string{"--config", config}, args...)
}

func codexResumeAliases(config string) ([]string, error) {
	if configured := configuredAliasNames(config); len(configured) > 0 {
		return configured, nil
	}
	out, err := exec.Command("codex-resume", archiveCodexResumeArgs(config, "--list")...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("list codex-resume aliases: %w: %s", err, strings.TrimSpace(string(out)))
	}
	aliases := strings.Fields(string(out))
	sort.Strings(aliases)
	return aliases, nil
}

func codexResumePolicy(alias, config string) (string, error) {
	out, err := exec.Command("codex-resume", archiveCodexResumeArgs(config, "--show-policy", alias)...).CombinedOutput()
	if err == nil {
		return string(out), nil
	}
	if policy := configuredAliasPolicy(config, alias); policy != "" {
		return policy, nil
	}
	return "", fmt.Errorf("show codex-resume policy: %w: %s", err, strings.TrimSpace(string(out)))
}

func resumeConfigPath(config string) string {
	if config != "" {
		return config
	}
	if root := os.Getenv("XDG_CONFIG_HOME"); root != "" {
		return filepath.Join(root, "codex-resume", "sessions.toml")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "codex-resume", "sessions.toml")
}

func configuredAliasNames(config string) []string {
	data, err := os.ReadFile(resumeConfigPath(config))
	if err != nil {
		return nil
	}
	pattern := regexp.MustCompile(`(?m)^\[aliases\."([^"]+)"\]$`)
	matches := pattern.FindAllStringSubmatch(string(data), -1)
	aliases := make([]string, 0, len(matches))
	for _, match := range matches {
		aliases = append(aliases, match[1])
	}
	sort.Strings(aliases)
	return aliases
}

func configuredShortAliases(config string) map[string]string {
	data, err := os.ReadFile(resumeConfigPath(config))
	if err != nil {
		return nil
	}
	aliases := map[string]string{}
	inside := false
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") {
			inside = trimmed == "[short_aliases]"
			continue
		}
		if !inside || trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		name, target, ok := strings.Cut(trimmed, "=")
		if ok {
			aliases[strings.TrimSpace(name)] = strings.Trim(strings.TrimSpace(target), `"`)
		}
	}
	return aliases
}

func configuredAliasPolicy(config, alias string) string {
	data, err := os.ReadFile(resumeConfigPath(config))
	if err != nil {
		return ""
	}
	header := `[aliases."` + alias + `"]`
	lines := strings.Split(string(data), "\n")
	inside := false
	var output strings.Builder
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") {
			if inside {
				break
			}
			inside = trimmed == header
			continue
		}
		if !inside || trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		key, value, ok := strings.Cut(trimmed, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(key), "session."))
		value = strings.Trim(strings.TrimSpace(value), `"`)
		switch key {
		case "description", "key", "sandbox", "approval", "cd", "enabled", "git_write", "delegate", "allow_extra_args":
			if key == "key" {
				key = "session_key"
			}
			fmt.Fprintf(&output, "%s: %s\n", key, value)
		}
	}
	return output.String()
}

func parsePolicy(input string) map[string]any {
	result := map[string]any{}
	var listKey string
	for _, line := range strings.Split(input, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- ") && listKey != "" {
			result[listKey] = append(result[listKey].([]string), strings.TrimSpace(strings.TrimPrefix(trimmed, "- ")))
			continue
		}
		key, value, ok := strings.Cut(trimmed, ":")
		if !ok {
			continue
		}
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		if value == "" {
			listKey = key
			result[key] = []string{}
			continue
		}
		listKey = ""
		if parsed, err := strconv.ParseBool(value); err == nil {
			result[key] = parsed
		} else {
			result[key] = value
		}
	}
	return result
}

func stringPolicy(policy map[string]any, key string) string {
	value, _ := policy[key].(string)
	return value
}

func boolPolicy(policy map[string]any, key string) bool {
	value, _ := policy[key].(bool)
	return value
}

func resolveSessionID(ctx *commandContext, key string) (string, error) {
	if key == "" {
		return "", errors.New("policy has no session key")
	}
	executable, err := os.Executable()
	if err != nil {
		return "", err
	}
	out, err := exec.Command(executable, "--config", ctx.store.Dir(), "vl", "secret", "get", key).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("read %s: %w: %s", key, err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

func archiveTranscript(codexHome, privateRoot, sessionID string) (string, error) {
	var source string
	err := filepath.WalkDir(filepath.Join(codexHome, "sessions"), func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), sessionID+".jsonl") {
			source = path
			return io.EOF
		}
		return nil
	})
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	if source == "" {
		return "", os.ErrNotExist
	}
	rel := filepath.Join("private", "sessions", sessionID+".jsonl.gz")
	destination := filepath.Join(filepath.Dir(privateRoot), rel)
	if err := gzipFile(source, destination); err != nil {
		return "", err
	}
	return rel, nil
}

func gzipFile(source, destination string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := destination + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	writer, err := gzip.NewWriterLevel(out, gzip.BestCompression)
	if err == nil {
		_, err = io.Copy(writer, in)
	}
	if closeErr := writer.Close(); err == nil {
		err = closeErr
	}
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, destination)
}

func archiveRepositories(cwd, privateRoot string) ([]archiveRepositoryRecord, error) {
	root, err := gitOutput(cwd, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, nil
	}
	roots := []string{root}
	output, _ := gitOutput(root, "submodule", "foreach", "--recursive", "--quiet", "pwd")
	for _, candidate := range strings.Split(output, "\n") {
		candidate = strings.TrimSpace(candidate)
		if candidate != "" {
			roots = append(roots, candidate)
		}
	}
	seen := map[string]bool{}
	var records []archiveRepositoryRecord
	for _, candidate := range roots {
		canonical, resolveErr := filepath.EvalSymlinks(candidate)
		if resolveErr != nil {
			return nil, resolveErr
		}
		if seen[canonical] {
			continue
		}
		seen[canonical] = true
		record, archiveErr := archiveRepositoryRoot(canonical, privateRoot)
		if archiveErr != nil {
			return nil, archiveErr
		}
		records = append(records, record)
	}
	return records, nil
}

func archiveRepositoryRoot(root, privateRoot string) (archiveRepositoryRecord, error) {
	head, err := gitOutput(root, "rev-parse", "HEAD")
	if err != nil {
		return archiveRepositoryRecord{}, err
	}
	branch, _ := gitOutput(root, "branch", "--show-current")
	remote, _ := gitOutput(root, "remote", "get-url", "origin")
	remote = sanitizeRemoteURL(remote)
	status, err := gitOutput(root, "status", "--porcelain=v1", "--untracked-files=normal")
	if err != nil {
		return archiveRepositoryRecord{}, err
	}
	dirty := status != ""
	remoteOK := false
	if remote != "" && !dirty {
		refs, refsErr := gitOutput(root, "ls-remote", "origin")
		remoteOK = refsErr == nil && strings.Contains(refs, head)
	}
	idHash := sha256.Sum256([]byte(root))
	record := archiveRepositoryRecord{ID: hex.EncodeToString(idHash[:12]), Root: root, Remote: remote, Branch: branch, Head: head, Dirty: dirty, RemoteOK: remoteOK}
	if remoteOK {
		return record, nil
	}
	snapshot, err := snapshotRepository(root, privateRoot)
	if err != nil {
		return archiveRepositoryRecord{}, err
	}
	record.Snapshot = snapshot
	return record, nil
}

func gitOutput(root string, args ...string) (string, error) {
	all := append([]string{"-C", root}, args...)
	out, err := exec.Command("git", all...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

func sanitizeRemoteURL(remote string) string {
	if strings.Contains(remote, "://") {
		prefix, rest, _ := strings.Cut(remote, "://")
		if at := strings.LastIndex(rest, "@"); at >= 0 {
			return prefix + "://" + rest[at+1:]
		}
	}
	return remote
}

func snapshotRepository(root, privateRoot string) (string, error) {
	filesRaw, err := exec.Command("git", "-C", root, "ls-files", "-co", "--exclude-standard", "-z").Output()
	if err != nil {
		return "", err
	}
	files := strings.Split(strings.TrimSuffix(string(filesRaw), "\x00"), "\x00")
	sort.Strings(files)
	hash := sha256.New()
	existingFiles := make([]string, 0, len(files))
	for _, name := range files {
		if name == "" {
			continue
		}
		info, statErr := os.Lstat(filepath.Join(root, name))
		if errors.Is(statErr, os.ErrNotExist) {
			continue
		}
		if statErr != nil {
			return "", statErr
		}
		existingFiles = append(existingFiles, name)
		fmt.Fprintf(hash, "%s\x00%d\x00", name, info.Mode())
		if info.Mode().IsRegular() {
			file, openErr := os.Open(filepath.Join(root, name))
			if openErr != nil {
				return "", openErr
			}
			_, copyErr := io.Copy(hash, file)
			_ = file.Close()
			if copyErr != nil {
				return "", copyErr
			}
		} else if info.Mode()&os.ModeSymlink != 0 {
			target, linkErr := os.Readlink(filepath.Join(root, name))
			if linkErr != nil {
				return "", linkErr
			}
			_, _ = io.WriteString(hash, target)
		}
	}
	contentID := hex.EncodeToString(hash.Sum(nil))
	rel := filepath.Join("private", "repositories", contentID+".tar.gz")
	destination := filepath.Join(filepath.Dir(privateRoot), rel)
	if _, statErr := os.Stat(destination); statErr == nil {
		return rel, nil
	}
	cmd := exec.Command("tar", "-czf", destination, "--null", "-T", "-")
	cmd.Dir = root
	cmd.Stdin = strings.NewReader(strings.Join(existingFiles, "\x00") + "\x00")
	if out, runErr := cmd.CombinedOutput(); runErr != nil {
		return "", fmt.Errorf("snapshot repository: %w: %s", runErr, strings.TrimSpace(string(out)))
	}
	return rel, nil
}

func ensureArchiveSupportFiles(root string) error {
	files := map[string]string{
		"SKILL.md": `---
name: specialist-archive
description: Preserve, inspect, search, and rebuild Jeff specialist identities, roles, transcripts, policies, and non-reproducible repository state. Use when specialist continuity may be lost, a Codex account changes, an archive refresh is requested, or a successor agent must recover prior specialist knowledge.
---

# Specialist Archive

Run ` + "`jeff specialists archive refresh`" + ` to update deterministic archive data. Add ` + "`--briefs`" + ` when current role and open-work summaries should be requested from enabled specialists.

Use ` + "`jeff specialists archive status`" + ` before relying on completeness and ` + "`jeff specialists archive search`" + ` for targeted offline transcript search. Never copy transcript content into tracked files without reviewing it for secrets and private data.

Use ` + "`jeff specialists archive successor`" + ` when rebuilding Jeff under another account. Preserve historical session ids as identity records; never overwrite persistent bindings without explicit human approval.
`,
		"agents/openai.yaml": `interface:
  display_name: "Specialist Archive"
  short_description: "Archive and recover Jeff specialists"
  default_prompt: "Refresh or inspect the Jeff specialist continuity archive."
`,
		"successor-prompt.md": `# Jeff Successor Recovery Prompt

You are rebuilding Jeff's specialist team from its persistent Memory Castle.

1. Run ` + "`~/.local/share/jeff/memcastle/codex/prepare-context.sh --quiet`" + `.
2. Read ` + "`codex/index.md`" + `, this archive's ` + "`registry.json`" + `, ` + "`repositories.json`" + `, and each ` + "`codex/specialists/<alias>/`" + ` room.
3. Treat every archived session id as historical identity and continuity. Attempt resume first, but do not assume another account can resume it.
4. If replacement sessions are required, present the old and proposed new ids and obtain explicit human approval before changing any Vaultline binding.
5. Clone remote-reproducible repositories at the recorded commit. Restore only repositories with a ` + "`snapshot`" + ` field from the Git-excluded private archive.
6. Search archived transcripts with ` + "`jeff specialists archive search`" + ` only when role files and current-state briefs are insufficient.
7. Validate the rebuilt registry with ` + "`codex-resume --list`" + ` and bounded smoke calls before declaring recovery complete.
`,
		"private/.gitignore": "*\n!.gitignore\n",
	}
	for relative, content := range files {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return err
		}
		if _, err := os.Stat(path); err == nil {
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			return err
		}
	}
	return nil
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o600)
}

func pruneArchivePayloads(root string, specialists []archiveSpecialistRecord, repositories []archiveRepositoryRecord) error {
	keepSessions := map[string]bool{}
	for _, specialist := range specialists {
		if specialist.Transcript != "" {
			keepSessions[filepath.Clean(filepath.Join(root, specialist.Transcript))] = true
		}
	}
	keepRepositories := map[string]bool{}
	for _, repository := range repositories {
		if repository.Snapshot != "" {
			keepRepositories[filepath.Clean(filepath.Join(root, repository.Snapshot))] = true
		}
	}
	for _, target := range []struct {
		dir       string
		extension string
		keep      map[string]bool
	}{
		{filepath.Join(root, "private", "sessions"), ".jsonl.gz", keepSessions},
		{filepath.Join(root, "private", "repositories"), ".tar.gz", keepRepositories},
	} {
		entries, err := os.ReadDir(target.dir)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			path := filepath.Join(target.dir, entry.Name())
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), target.extension) || target.keep[path] {
				continue
			}
			if err := os.Remove(path); err != nil {
				return fmt.Errorf("remove stale archive payload %s: %w", path, err)
			}
		}
	}
	return nil
}

func writeSpecialistRooms(ctx *commandContext, specialists []archiveSpecialistRecord) error {
	agentDir, err := ctx.store.AgentDir()
	if err != nil {
		return err
	}
	root := filepath.Join(agentDir, "codex", "specialists")
	if err := os.MkdirAll(root, 0o700); err != nil {
		return err
	}
	var index strings.Builder
	index.WriteString("# Specialist Rooms\n\nGenerated pointers to archived specialist identities and roles.\n\n")
	for _, specialist := range specialists {
		dir := filepath.Join(root, specialist.Alias)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
		fmt.Fprintf(&index, "- [%s](%s/index.md): %s\n", specialist.Alias, specialist.Alias, specialist.Description)
		content := fmt.Sprintf("# %s Specialist\n\n- Enabled: %t\n- Session id: `%s`\n- Session key: `%s`\n- Role: %s\n- Archive record: `../../specialist-archive/registry.json`\n", specialist.Alias, specialist.Enabled, specialist.SessionID, specialist.SessionKey, specialist.Description)
		if err := os.WriteFile(filepath.Join(dir, "index.md"), []byte(content), 0o600); err != nil {
			return err
		}
		if err := writeJSON(filepath.Join(dir, "runtime.json"), specialist); err != nil {
			return err
		}
		rolePath := filepath.Join(dir, "role.md")
		if _, err := os.Stat(rolePath); errors.Is(err, os.ErrNotExist) {
			role := fmt.Sprintf("# Role\n\n%s\n", specialist.Description)
			if writeErr := os.WriteFile(rolePath, []byte(role), 0o600); writeErr != nil {
				return writeErr
			}
		}
	}
	return os.WriteFile(filepath.Join(root, "index.md"), []byte(index.String()), 0o600)
}

func installArchiveSkillLink(root, codexHome string) error {
	skillsDir := filepath.Join(codexHome, "skills")
	if err := os.MkdirAll(skillsDir, 0o700); err != nil {
		return err
	}
	link := filepath.Join(skillsDir, "specialist-archive")
	if target, err := os.Readlink(link); err == nil {
		if target == root {
			return nil
		}
		return fmt.Errorf("specialist archive skill link points to %s", target)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("specialist archive skill path exists and is not a symlink")
	}
	return os.Symlink(root, link)
}

func refreshContinuityBriefs(root string, specialists []archiveSpecialistRecord) []string {
	type job struct {
		alias     string
		sessionID string
		aliases   []string
	}
	jobs := make(chan job)
	errorsChannel := make(chan string, len(specialists))
	seen := map[string]bool{}
	var workers sync.WaitGroup
	for worker := 0; worker < 4; worker++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for item := range jobs {
				prompt := "Goal: create a concise continuity brief for a future replacement agent.\nNeed: exact role, durable knowledge locations, current work, open risks, and restart instructions.\nRules: English; no secrets or secret values; do not change files; high semantic density."
				command := exec.Command("codex-resume", "exec", item.alias, "--", prompt)
				var stdout strings.Builder
				var stderr strings.Builder
				command.Stdout = &stdout
				command.Stderr = &stderr
				if err := command.Run(); err != nil {
					message := strings.TrimSpace(stderr.String())
					if message == "" {
						message = strings.TrimSpace(stdout.String())
					}
					errorsChannel <- item.alias + " brief: " + message
					continue
				}
				brief := cleanCodexResumeOutput(stdout.String())
				for _, alias := range item.aliases {
					path := filepath.Join(filepath.Dir(root), "specialists", alias, "current-state.md")
					if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
						errorsChannel <- alias + " brief: " + err.Error()
						continue
					}
					content := fmt.Sprintf("# Current State\n\nUpdated: %s\nShared session: `%s`\nCollected through alias: `%s`\n\n%s\n", time.Now().Format(time.RFC3339), item.sessionID, item.alias, brief)
					if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
						errorsChannel <- alias + " brief: " + err.Error()
					}
				}
			}
		}()
	}
	groups := map[string][]archiveSpecialistRecord{}
	for _, specialist := range specialists {
		if !specialist.Enabled || specialist.SessionID == "" {
			continue
		}
		groups[specialist.SessionID] = append(groups[specialist.SessionID], specialist)
	}
	for sessionID, group := range groups {
		if seen[sessionID] {
			continue
		}
		seen[sessionID] = true
		representative := group[0].Alias
		aliases := make([]string, 0, len(group))
		for _, specialist := range group {
			aliases = append(aliases, specialist.Alias)
			if boolPolicy(specialist.Policy, "exec_enabled") {
				representative = specialist.Alias
			}
		}
		sort.Strings(aliases)
		jobs <- job{alias: representative, sessionID: sessionID, aliases: aliases}
	}
	close(jobs)
	workers.Wait()
	close(errorsChannel)
	var result []string
	for value := range errorsChannel {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func searchSpecialistArchive(ctx *commandContext, query, alias, scope string, regexMode bool, limit int) error {
	if scope != "messages" && scope != "all" {
		return errors.New("scope must be messages or all")
	}
	if limit < 1 {
		return errors.New("limit must be positive")
	}
	root, err := specialistArchiveRoot(ctx)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(filepath.Join(root, "registry.json"))
	if err != nil {
		return err
	}
	var registry archiveRegistry
	if err := json.Unmarshal(data, &registry); err != nil {
		return err
	}
	pattern := regexp.QuoteMeta(query)
	if regexMode {
		pattern = query
	}
	matcher, err := regexp.Compile("(?i)" + pattern)
	if err != nil {
		return fmt.Errorf("invalid regex: %w", err)
	}
	count := 0
	seenSessions := map[string]bool{}
	for _, specialist := range registry.Specialists {
		if alias != "" && specialist.Alias != alias {
			continue
		}
		if specialist.Transcript == "" || seenSessions[specialist.SessionID] {
			continue
		}
		seenSessions[specialist.SessionID] = true
		path := filepath.Join(root, filepath.FromSlash(specialist.Transcript))
		matches, searchErr := searchTranscript(path, matcher, scope, limit-count)
		if searchErr != nil {
			return searchErr
		}
		for _, match := range matches {
			fmt.Fprintf(ctx.stdout, "%s\t%s\n", specialist.Alias, match)
			count++
			if count >= limit {
				return nil
			}
		}
	}
	return nil
}

func searchTranscript(path string, matcher *regexp.Regexp, scope string, limit int) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	reader, err := gzip.NewReader(file)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	var matches []string
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if scope == "messages" && !isMessageEvent(line) {
			continue
		}
		if matcher.MatchString(line) {
			matches = append(matches, shortenArchiveMatch(line))
			if len(matches) >= limit {
				break
			}
		}
	}
	return matches, scanner.Err()
}

func isMessageEvent(line string) bool {
	return strings.Contains(line, `"type":"message"`) || strings.Contains(line, `"type":"user_message"`) || strings.Contains(line, `"type":"agent_message"`)
}

func shortenArchiveMatch(line string) string {
	line = strings.ReplaceAll(line, "\t", " ")
	if len(line) > 500 {
		return line[:500] + "..."
	}
	return line
}
