package migrations

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"jeff/internal/config"
)

func TestRunMovesLegacyFinanceDataAndStampsConfig(t *testing.T) {
	home := t.TempDir()
	configRoot := filepath.Join(home, "config")
	dataRoot := filepath.Join(home, "data")
	t.Setenv("XDG_CONFIG_HOME", configRoot)
	t.Setenv("XDG_DATA_HOME", dataRoot)

	store, err := config.NewStore("")
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(store.Dir(), "finance"), 0o755); err != nil {
		t.Fatalf("create legacy finance dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(store.Dir(), "finance", "ledger.json"), []byte("{}\n"), 0o600); err != nil {
		t.Fatalf("write legacy finance file: %v", err)
	}
	if err := os.WriteFile(store.ConfigPath(), []byte(`{"codex_binary":"codex"}`+"\n"), 0o600); err != nil {
		t.Fatalf("write legacy config: %v", err)
	}

	result, err := Run(store, nil)
	if err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	if len(result.Moved) != 1 {
		t.Fatalf("expected one moved path, got %+v", result.Moved)
	}

	if _, err := os.Stat(filepath.Join(store.Dir(), "finance")); !os.IsNotExist(err) {
		t.Fatalf("legacy finance dir still exists or stat failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dataRoot, "jeff", "finance", "ledger.json")); err != nil {
		t.Fatalf("migrated finance file missing: %v", err)
	}

	var raw struct {
		SchemaVersion int `json:"schema_version"`
	}
	data, err := os.ReadFile(store.ConfigPath())
	if err != nil {
		t.Fatalf("read migrated config: %v", err)
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("parse migrated config: %v", err)
	}
	if raw.SchemaVersion != config.CurrentSchemaVersion {
		t.Fatalf("schema version mismatch: %d", raw.SchemaVersion)
	}
}

func TestRunDoesNotCreateConfigForEmptyInstall(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))

	store, err := config.NewStore("")
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	if _, err := Run(store, nil); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	if _, err := os.Stat(store.ConfigPath()); !os.IsNotExist(err) {
		t.Fatalf("config should not be created for empty install: %v", err)
	}
}

func TestRunFlattensMemcastleLayout(t *testing.T) {
	home := t.TempDir()
	configRoot := filepath.Join(home, "config")
	dataRoot := filepath.Join(home, "data")
	t.Setenv("XDG_CONFIG_HOME", configRoot)
	t.Setenv("XDG_DATA_HOME", dataRoot)

	store, err := config.NewStore("")
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	oldRoom := filepath.Join(dataRoot, "jeff", "memcastle", "wings", "finance", "floors", "banking", "rooms")
	if err := os.MkdirAll(oldRoom, 0o755); err != nil {
		t.Fatalf("create old room: %v", err)
	}
	if err := os.WriteFile(filepath.Join(oldRoom, "api-access.md"), []byte("# API\n"), 0o600); err != nil {
		t.Fatalf("write old room file: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(store.ConfigPath()), 0o755); err != nil {
		t.Fatalf("create config dir: %v", err)
	}
	if err := os.WriteFile(store.ConfigPath(), []byte(`{"schema_version":1}`+"\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	result, err := Run(store, nil)
	if err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	if len(result.Moved) != 1 {
		t.Fatalf("expected one moved path, got %+v", result.Moved)
	}
	if _, err := os.Stat(filepath.Join(dataRoot, "jeff", "memcastle", "finance", "banking", "api-access.md")); err != nil {
		t.Fatalf("flattened room missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dataRoot, "jeff", "memcastle", "wings")); !os.IsNotExist(err) {
		t.Fatalf("old wings directory should be removed when empty: %v", err)
	}
}
