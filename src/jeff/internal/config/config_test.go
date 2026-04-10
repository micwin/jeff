package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStoreLoadAndSave(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}

	cfg, err := store.Load()
	if err != nil {
		t.Fatalf("load empty config: %v", err)
	}

	if cfg.CodexBinary == "" {
		t.Fatalf("expected default codex binary, got empty string")
	}

	cfg.RecordSession("abc123")
	cfg.CodexBinary = "/usr/local/bin/codex"

	if err := store.Save(cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	data, readErr := os.ReadFile(filepath.Join(tmpDir, "config.json"))
	if readErr != nil {
		t.Fatalf("config not written: %v", readErr)
	}
	if len(data) == 0 {
		t.Fatalf("config file empty")
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}

	if loaded.ActiveSession != "abc123" || loaded.LastSession != "abc123" {
		t.Fatalf("session not persisted: %+v", loaded)
	}
	if loaded.CodexBinary != "/usr/local/bin/codex" {
		t.Fatalf("binary mismatch: %s", loaded.CodexBinary)
	}

	if len(loaded.SessionHistory) != 1 || loaded.SessionHistory[0] != "abc123" {
		t.Fatalf("history mismatch: %+v", loaded.SessionHistory)
	}
}

func TestRecordSessionDedup(t *testing.T) {
	cfg := &Config{}

	cfg.RecordSession("one")
	cfg.RecordSession("one")
	cfg.RecordSession("two")

	if cfg.ActiveSession != "two" {
		t.Fatalf("active session mismatch: %s", cfg.ActiveSession)
	}
	if len(cfg.SessionHistory) != 2 {
		t.Fatalf("expected two history entries, got %d", len(cfg.SessionHistory))
	}
}
