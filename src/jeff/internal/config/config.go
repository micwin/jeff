package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
)

const (
	appName          = "jeff"
	configFileName   = "config.json"
	completionFolder = "completions"
	defaultCodexBin  = "codex"
)

// Config represents the persisted CLI configuration.
type Config struct {
	ActiveSession  string   `json:"active_session"`
	LastSession    string   `json:"last_session"`
	SessionHistory []string `json:"session_history,omitempty"`
	CompletionDir  string   `json:"completion_dir,omitempty"`
	CodexBinary    string   `json:"codex_binary,omitempty"`
}

// Store keeps configuration on disk.
type Store struct {
	dir        string
	configPath string
}

// NewStore builds a new configuration store at the provided directory or the default XDG config directory.
func NewStore(customDir string) (*Store, error) {
	dir := customDir
	if dir == "" {
		var err error
		dir, err = defaultConfigDir()
		if err != nil {
			return nil, err
		}
	} else {
		var err error
		dir, err = filepath.Abs(dir)
		if err != nil {
			return nil, err
		}
	}

	return &Store{
		dir:        dir,
		configPath: filepath.Join(dir, configFileName),
	}, nil
}

// Dir reports the directory that contains the config file.
func (s *Store) Dir() string {
	return s.dir
}

// CompletionDir determines the directory where completion scripts should live.
func (s *Store) CompletionDir(cfg *Config) string {
	if cfg != nil && cfg.CompletionDir != "" {
		return cfg.CompletionDir
	}
	return filepath.Join(s.dir, completionFolder)
}

// Load reads the configuration from disk, returning defaults when the file does not exist.
func (s *Store) Load() (*Config, error) {
	data, err := os.ReadFile(s.configPath)
	if errors.Is(err, os.ErrNotExist) {
		cfg := defaultConfig()
		return cfg, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if cfg.CodexBinary == "" {
		cfg.CodexBinary = defaultCodexBin
	}

	return &cfg, nil
}

// Save writes the configuration back to disk.
func (s *Store) Save(cfg *Config) error {
	if cfg == nil {
		return errors.New("nil config")
	}

	if cfg.CodexBinary == "" {
		cfg.CodexBinary = defaultCodexBin
	}

	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	content, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}

	tmpPath := s.configPath + ".tmp"
	if err := os.WriteFile(tmpPath, content, 0o600); err != nil {
		return fmt.Errorf("write temp config: %w", err)
	}

	return os.Rename(tmpPath, s.configPath)
}

// RecordSession stores the provided session as the active and last-used value, keeping a deduped history.
func (c *Config) RecordSession(session string) {
	if session == "" {
		return
	}
	c.ActiveSession = session
	c.LastSession = session

	if !slices.Contains(c.SessionHistory, session) {
		c.SessionHistory = append(c.SessionHistory, session)
	}
}

func defaultConfig() *Config {
	return &Config{
		CodexBinary: defaultCodexBin,
	}
}

func defaultConfigDir() (string, error) {
	if val := os.Getenv("XDG_CONFIG_HOME"); val != "" {
		return filepath.Join(val, appName), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home: %w", err)
	}

	return filepath.Join(home, ".config", appName), nil
}
