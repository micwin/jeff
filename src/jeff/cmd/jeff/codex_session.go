package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"jeff/internal/config"
)

var codexSessionIDPattern = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)

func configuredCodexSession(cfg *config.Config) string {
	if cfg.Codex.SessionID != "" {
		return cfg.Codex.SessionID
	}
	if cfg.ActiveSession != "" {
		return cfg.ActiveSession
	}
	return cfg.LastSession
}

func codexResumeArgs(sessionID string, prompt ...string) []string {
	args := []string{"resume", sessionID}
	return append(args, prompt...)
}

func resolveLastCodexSessionID() (string, error) {
	root, err := codexHomeDir()
	if err != nil {
		return "", err
	}
	sessionsDir := filepath.Join(root, "sessions")
	var newestID string
	var newestMod time.Time
	err = filepath.WalkDir(sessionsDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if errors.Is(walkErr, os.ErrNotExist) {
				return nil
			}
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".jsonl" {
			return nil
		}
		match := codexSessionIDPattern.FindString(filepath.Base(path))
		if match == "" {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if newestID == "" || info.ModTime().After(newestMod) {
			newestID = match
			newestMod = info.ModTime()
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("scan Codex sessions: %w", err)
	}
	if newestID == "" {
		return "", fmt.Errorf("no Codex sessions found below %s", sessionsDir)
	}
	return newestID, nil
}

func codexHomeDir() (string, error) {
	if value := os.Getenv("CODEX_HOME"); value != "" {
		return value, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".codex"), nil
}
