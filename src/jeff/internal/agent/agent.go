package agent

import (
	"bufio"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"jeff/internal/config"
)

//go:embed defaults/**
var defaults embed.FS

type BootstrapResult struct {
	Created []string
	Skipped []string
}

type CastleInfo struct {
	Root     string
	Files    int
	Wings    int
	Floors   int
	Rooms    int
	Cabinets int
	Drawers  int
	Tree     []string
}

type SearchMatch struct {
	Path    string
	Line    int
	Excerpt string
}

func Bootstrap(store *config.Store, force bool) (*BootstrapResult, error) {
	if store == nil {
		return nil, errors.New("nil config store")
	}
	result := &BootstrapResult{}
	dataDir, err := store.DataDir()
	if err != nil {
		return nil, err
	}
	for _, dir := range []string{"skills", "reports", "vaultline", "commands"} {
		if err := os.MkdirAll(filepath.Join(dataDir, dir), 0o755); err != nil {
			return nil, fmt.Errorf("create %s dir: %w", dir, err)
		}
	}
	if err := copyDefaults("defaults/memcastle", filepath.Join(dataDir, "memcastle"), force, result); err != nil {
		return nil, err
	}
	if err := copyDefaults("defaults/commands", filepath.Join(dataDir, "commands"), force, result); err != nil {
		return nil, err
	}
	return result, nil
}

func SystemPrompt(store *config.Store) (string, error) {
	agentDir, err := store.AgentDir()
	if err != nil {
		return "", err
	}
	paths := []string{
		filepath.Join(agentDir, "system.md"),
	}
	var parts []string
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("read agent prompt %s: %w", path, err)
		}
		text := strings.TrimSpace(string(data))
		if text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n\n"), nil
}

func CastleInfoFor(store *config.Store) (*CastleInfo, error) {
	agentDir, err := store.AgentDir()
	if err != nil {
		return nil, err
	}
	info := &CastleInfo{Root: agentDir}
	err = filepath.WalkDir(agentDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(agentDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		slashRel := filepath.ToSlash(rel)
		if strings.HasPrefix(slashRel, "state/") || strings.HasPrefix(slashRel, "logbook/") {
			return nil
		}
		parts := strings.Split(slashRel, "/")
		if entry.IsDir() {
			switch len(parts) {
			case 1:
				if entry.Name() == "gatehouse" {
					return nil
				}
				info.Wings++
			case 2:
				info.Floors++
			case 3:
				info.Cabinets++
			case 4:
				info.Drawers++
			}
		} else {
			info.Files++
			if entry.Name() == "index.md" {
				return nil
			}
			if len(parts) >= 2 {
				info.Rooms++
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk memory castle: %w", err)
	}
	info.Tree, err = structureTree(agentDir)
	if err != nil {
		return nil, err
	}
	return info, nil
}

func structureTree(root string) ([]string, error) {
	var lines []string
	lines = append(lines, filepath.Base(root)+"/")
	children, err := treeChildren(root)
	if err != nil {
		return nil, err
	}
	for index, child := range children {
		if err := appendTree(root, child, "", index == len(children)-1, &lines); err != nil {
			return nil, err
		}
	}
	return lines, nil
}

func appendTree(root, rel, prefix string, last bool, lines *[]string) error {
	path := filepath.Join(root, filepath.FromSlash(rel))
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	connector := "|-- "
	nextPrefix := prefix + "|   "
	if last {
		connector = "`-- "
		nextPrefix = prefix + "    "
	}
	name := filepath.Base(rel)
	if info.IsDir() {
		name += "/"
	}
	*lines = append(*lines, prefix+connector+name)
	if !info.IsDir() {
		return nil
	}
	children, err := treeChildren(path)
	if err != nil {
		return err
	}
	for index, child := range children {
		childRel := filepath.ToSlash(filepath.Join(rel, child))
		if err := appendTree(root, childRel, nextPrefix, index == len(children)-1, lines); err != nil {
			return err
		}
	}
	return nil
}

func treeChildren(path string) ([]string, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	var dirs []string
	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, entry.Name())
		} else {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(dirs)
	sort.Strings(files)
	return append(dirs, files...), nil
}

func CastleDocument(store *config.Store) (string, error) {
	agentDir, err := store.AgentDir()
	if err != nil {
		return "", err
	}
	var parts []string
	err = filepath.WalkDir(agentDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || strings.Contains(filepath.ToSlash(path), "/state/") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(agentDir, path)
		if err != nil {
			return err
		}
		parts = append(parts, fmt.Sprintf("## %s\n\n%s", filepath.ToSlash(rel), strings.TrimSpace(string(data))))
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("read memory castle: %w", err)
	}
	sort.Strings(parts)
	return strings.Join(parts, "\n\n---\n\n"), nil
}

func SearchCastle(store *config.Store, query string, regex bool) ([]SearchMatch, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, errors.New("search query must not be empty")
	}
	agentDir, err := store.AgentDir()
	if err != nil {
		return nil, err
	}
	var matcher func(string) bool
	if regex {
		re, err := regexp.Compile("(?i)" + query)
		if err != nil {
			return nil, err
		}
		matcher = re.MatchString
	} else {
		needle := normalizeSearchText(query)
		matcher = func(line string) bool {
			return strings.Contains(normalizeSearchText(line), needle)
		}
	}
	var matches []SearchMatch
	err = filepath.WalkDir(agentDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || strings.Contains(filepath.ToSlash(path), "/state/") {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		rel, err := filepath.Rel(agentDir, path)
		if err != nil {
			return err
		}
		var lines []string
		scanner := bufio.NewScanner(file)
		lineNo := 0
		for scanner.Scan() {
			lineNo++
			line := scanner.Text()
			lines = append(lines, line)
			if matcher(line) {
				matches = append(matches, SearchMatch{
					Path:    filepath.ToSlash(rel),
					Line:    lineNo,
					Excerpt: strings.TrimSpace(line),
				})
			}
		}
		if err := scanner.Err(); err != nil {
			return err
		}
		if !regex && len(lines) > 0 && matcher(strings.Join(lines, " ")) {
			matches = append(matches, SearchMatch{
				Path:    filepath.ToSlash(rel),
				Line:    1,
				Excerpt: "(file-level whitespace-normalized match)",
			})
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("search memory castle: %w", err)
	}
	return matches, nil
}

func normalizeSearchText(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(value)), " ")
}

func PromptInjected(store *config.Store, sessionID string) (bool, error) {
	path, err := promptSemaphorePath(store, sessionID)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

func MarkPromptInjected(store *config.Store, sessionID string) error {
	path, err := promptSemaphorePath(store, sessionID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create prompt semaphore dir: %w", err)
	}
	content := fmt.Sprintf("session=%s\ninjected_at=%s\n", sessionID, time.Now().Format(time.RFC3339))
	return os.WriteFile(path, []byte(content), 0o600)
}

func promptSemaphorePath(store *config.Store, sessionID string) (string, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return "", errors.New("session id must not be empty")
	}
	agentDir, err := store.AgentDir()
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(sessionID))
	name := hex.EncodeToString(sum[:]) + ".prompt-injected"
	return filepath.Join(agentDir, "state", "sessions", name), nil
}

func RecordLog(store *config.Store, text string) (string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", errors.New("memory log entry must not be empty")
	}
	agentDir, err := store.AgentDir()
	if err != nil {
		return "", err
	}
	now := time.Now()
	path := filepath.Join(agentDir, "logbook", now.Format("2006"), now.Format("2006-01-02")+".md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("create logbook dir: %w", err)
	}
	entry := fmt.Sprintf("\n## %s\n\n%s\n", now.Format(time.RFC3339), text)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return "", fmt.Errorf("open logbook: %w", err)
	}
	defer file.Close()
	if _, err := file.WriteString(entry); err != nil {
		return "", fmt.Errorf("write logbook: %w", err)
	}
	return path, nil
}

func copyDefaults(srcRoot, dstRoot string, force bool, result *BootstrapResult) error {
	return fs.WalkDir(defaults, srcRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(srcRoot, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return os.MkdirAll(dstRoot, 0o755)
		}
		dst := filepath.Join(dstRoot, filepath.FromSlash(rel))
		if entry.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		if !force {
			if _, err := os.Stat(dst); err == nil {
				result.Skipped = append(result.Skipped, dst)
				return nil
			} else if !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("stat %s: %w", dst, err)
			}
		}
		data, err := defaults.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.WriteFile(dst, data, 0o600); err != nil {
			return fmt.Errorf("write default %s: %w", dst, err)
		}
		result.Created = append(result.Created, dst)
		return nil
	})
}

func SortedPaths(paths []string) []string {
	out := append([]string(nil), paths...)
	sort.Strings(out)
	return out
}
