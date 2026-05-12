package agent

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
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
		filepath.Join(agentDir, "castle.md"),
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
