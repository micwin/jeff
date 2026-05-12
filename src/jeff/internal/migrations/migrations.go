package migrations

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"jeff/internal/config"
)

// Result summarizes what the migration runner changed.
type Result struct {
	FromVersion int
	ToVersion   int
	Moved       []Move
	Skipped     []Move
}

// Move describes a filesystem migration.
type Move struct {
	From   string
	To     string
	Reason string
}

type migration struct {
	version int
	run     func(*config.Store, *Result) error
}

var all = []migration{
	{version: 1, run: migrateDurableDataOutOfConfig},
	{version: 2, run: migrateFlatMemcastleLayout},
}

// Run applies all pending migrations for the provided store.
func Run(store *config.Store, log io.Writer) (*Result, error) {
	if store == nil {
		return nil, errors.New("nil config store")
	}

	exists, err := pathExists(store.ConfigPath())
	if err != nil {
		return nil, err
	}
	hasLegacyData, err := legacyDurableDataExists(store)
	if err != nil {
		return nil, err
	}
	if !exists && !hasLegacyData {
		return &Result{ToVersion: config.CurrentSchemaVersion}, nil
	}

	version, err := readSchemaVersion(store.ConfigPath())
	if err != nil {
		return nil, err
	}
	result := &Result{FromVersion: version, ToVersion: config.CurrentSchemaVersion}
	if version >= config.CurrentSchemaVersion {
		return result, nil
	}

	for _, item := range all {
		if version < item.version {
			if err := item.run(store, result); err != nil {
				return result, err
			}
			version = item.version
		}
	}

	if err := writeSchemaVersion(store.ConfigPath(), config.CurrentSchemaVersion); err != nil {
		return result, err
	}
	report(log, result)
	return result, nil
}

func migrateDurableDataOutOfConfig(store *config.Store, result *Result) error {
	dataDir, err := store.DataDir()
	if err != nil {
		return err
	}

	for _, name := range legacyDurableNames() {
		from := filepath.Join(store.Dir(), name)
		exists, err := pathExists(from)
		if err != nil {
			return err
		}
		if !exists {
			continue
		}

		to := filepath.Join(dataDir, name)
		toExists, err := pathExists(to)
		if err != nil {
			return err
		}
		move := Move{From: from, To: to}
		if toExists {
			move.Reason = "destination exists"
			result.Skipped = append(result.Skipped, move)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
			return fmt.Errorf("create data dir: %w", err)
		}
		if err := os.Rename(from, to); err != nil {
			return fmt.Errorf("move %s to %s: %w", from, to, err)
		}
		result.Moved = append(result.Moved, move)
	}
	return nil
}

func legacyDurableDataExists(store *config.Store) (bool, error) {
	for _, name := range legacyDurableNames() {
		exists, err := pathExists(filepath.Join(store.Dir(), name))
		if err != nil {
			return false, err
		}
		if exists {
			return true, nil
		}
	}
	return false, nil
}

func legacyDurableNames() []string {
	return []string{
		"finance",
		"finance.json",
		"finance.db",
		"banking",
		"banking.json",
		"banking.db",
	}
}

func migrateFlatMemcastleLayout(store *config.Store, result *Result) error {
	agentDir, err := store.AgentDir()
	if err != nil {
		return err
	}
	moves := []Move{
		{
			From: filepath.Join(agentDir, "wings", "codex", "index.md"),
			To:   filepath.Join(agentDir, "codex", "index.md"),
		},
		{
			From: filepath.Join(agentDir, "wings", "codex", "floors", "sessions", "rooms", "session.md"),
			To:   filepath.Join(agentDir, "codex", "sessions.md"),
		},
		{
			From: filepath.Join(agentDir, "wings", "finance", "index.md"),
			To:   filepath.Join(agentDir, "finance", "index.md"),
		},
		{
			From: filepath.Join(agentDir, "wings", "finance", "floors", "banking", "index.md"),
			To:   filepath.Join(agentDir, "finance", "banking", "index.md"),
		},
		{
			From: filepath.Join(agentDir, "wings", "finance", "floors", "banking", "rooms", "api-access.md"),
			To:   filepath.Join(agentDir, "finance", "banking", "api-access.md"),
		},
		{
			From: filepath.Join(agentDir, "wings", "jeff", "index.md"),
			To:   filepath.Join(agentDir, "jeff", "index.md"),
		},
		{
			From: filepath.Join(agentDir, "wings", "jeff", "floors", "architecture", "index.md"),
			To:   filepath.Join(agentDir, "jeff", "architecture", "index.md"),
		},
		{
			From: filepath.Join(agentDir, "wings", "jeff", "floors", "architecture", "rooms", "storage.md"),
			To:   filepath.Join(agentDir, "jeff", "architecture", "storage.md"),
		},
	}
	for _, move := range moves {
		if err := moveIfPresent(move, result); err != nil {
			return err
		}
	}
	_ = removeEmptyDirs(filepath.Join(agentDir, "wings"))
	return nil
}

func moveIfPresent(move Move, result *Result) error {
	exists, err := pathExists(move.From)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	toExists, err := pathExists(move.To)
	if err != nil {
		return err
	}
	if toExists {
		move.Reason = "destination exists"
		result.Skipped = append(result.Skipped, move)
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(move.To), 0o755); err != nil {
		return fmt.Errorf("create memcastle dir: %w", err)
	}
	if err := os.Rename(move.From, move.To); err != nil {
		return fmt.Errorf("move %s to %s: %w", move.From, move.To, err)
	}
	result.Moved = append(result.Moved, move)
	return nil
}

func removeEmptyDirs(root string) error {
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			if err := removeEmptyDirs(filepath.Join(root, entry.Name())); err != nil {
				return err
			}
		}
	}
	entries, err = os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return os.Remove(root)
	}
	return nil
}

func readSchemaVersion(path string) (int, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read config version: %w", err)
	}
	if len(data) == 0 {
		return 0, nil
	}

	var raw struct {
		SchemaVersion int `json:"schema_version"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return 0, fmt.Errorf("parse config version: %w", err)
	}
	return raw.SchemaVersion, nil
}

func writeSchemaVersion(path string, version int) error {
	raw := map[string]any{}
	data, err := os.ReadFile(path)
	if err == nil && len(data) > 0 {
		if err := json.Unmarshal(data, &raw); err != nil {
			return fmt.Errorf("parse config for migration stamp: %w", err)
		}
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read config for migration stamp: %w", err)
	}

	raw["schema_version"] = version
	content, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config migration stamp: %w", err)
	}
	content = append(content, '\n')

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, content, 0o600); err != nil {
		return fmt.Errorf("write config migration stamp: %w", err)
	}
	return os.Rename(tmpPath, path)
}

func report(w io.Writer, result *Result) {
	if w == nil || result == nil {
		return
	}
	for _, move := range result.Moved {
		fmt.Fprintf(w, "migrated %s -> %s\n", move.From, move.To)
	}
	for _, move := range result.Skipped {
		fmt.Fprintf(w, "skipped %s -> %s: %s\n", move.From, move.To, move.Reason)
	}
}

func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, fmt.Errorf("stat %s: %w", path, err)
}
