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
	appName              = "jeff"
	CurrentSchemaVersion = 1
	configFileName       = "config.json"
	menuFileName         = "menu.json"
	completionFolder     = "completions"
	defaultCodexBin      = "codex"
	defaultInterval      = 5
	defaultLeftCommand   = "$(pwd)"
	defaultCenterCommand = "$(date '+%H:%M:%S')"
	defaultRightCommand  = "$(id -un)@$(hostname)"
	MenuEntryTypeCommand = "command"
	MenuEntryTypeMenu    = "menu"
)

// Config represents the persisted CLI configuration.
type Config struct {
	SchemaVersion  int               `json:"schema_version,omitempty"`
	ActiveSession  string            `json:"active_session"`
	LastSession    string            `json:"last_session"`
	SessionHistory []string          `json:"session_history,omitempty"`
	CompletionDir  string            `json:"completion_dir,omitempty"`
	CodexBinary    string            `json:"codex_binary,omitempty"`
	ShellStatus    ShellStatusConfig `json:"shell_status,omitempty"`
}

type TmuxMenuEntry struct {
	ID       string          `json:"id"`
	Label    string          `json:"label"`
	Command  string          `json:"command,omitempty"`
	Type     string          `json:"type,omitempty"`
	Children []TmuxMenuEntry `json:"children,omitempty"`
}

type ShellStatusConfig struct {
	Left   ShellStatusRegion `json:"left"`
	Center ShellStatusRegion `json:"center"`
	Right  ShellStatusRegion `json:"right"`
	Layout ShellStatusLayout `json:"layout"`
}

type ShellStatusRegion struct {
	Command  string `json:"command"`
	Interval int    `json:"interval"`
}

type ShellStatusLayout struct {
	Left   int `json:"left"`
	Center int `json:"center"`
	Right  int `json:"right"`
}

// Store keeps configuration on disk.
type Store struct {
	dir        string
	configPath string
	menuPath   string
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
		menuPath:   filepath.Join(dir, menuFileName),
	}, nil
}

// Dir reports the directory that contains the config file.
func (s *Store) Dir() string {
	return s.dir
}

// ConfigPath returns the absolute path to the main configuration file.
func (s *Store) ConfigPath() string {
	return s.configPath
}

// DataDir reports the XDG data directory Jeff uses for durable user data.
func (s *Store) DataDir() (string, error) {
	return defaultDataDir()
}

// CacheDir reports the XDG cache directory Jeff uses for rebuildable data.
func (s *Store) CacheDir() (string, error) {
	return defaultCacheDir()
}

// CompletionDir determines the directory where completion scripts should live.
func (s *Store) CompletionDir(cfg *Config) string {
	if cfg != nil && cfg.CompletionDir != "" {
		return cfg.CompletionDir
	}
	return filepath.Join(s.dir, completionFolder)
}

// MenuPath returns the absolute path to the menu configuration file.
func (s *Store) MenuPath() string {
	return s.menuPath
}

// Load reads the configuration from disk, returning defaults when the file does not exist.
func (s *Store) Load() (*Config, error) {
	data, err := os.ReadFile(s.configPath)
	if errors.Is(err, os.ErrNotExist) {
		cfg := defaultConfig()
		cfg.ensureShellDefaults()
		return cfg, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	cfg.applyDefaults()

	return &cfg, nil
}

// Save writes the configuration back to disk.
func (s *Store) Save(cfg *Config) error {
	if cfg == nil {
		return errors.New("nil config")
	}

	cfg.applyDefaults()

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

// LoadMenu reads the menu configuration from disk, returning nil when no menu file exists.
func (s *Store) LoadMenu() ([]TmuxMenuEntry, error) {
	data, err := os.ReadFile(s.menuPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read menu: %w", err)
	}
	if len(data) == 0 {
		return nil, nil
	}

	var entries []TmuxMenuEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parse menu: %w", err)
	}
	ensureMenuEntryDefaults(entries)
	used := make(map[string]struct{})
	assignMenuIDs(entries, used)
	return entries, nil
}

// SaveMenu writes the provided menu entries to disk.
func (s *Store) SaveMenu(entries []TmuxMenuEntry) error {
	if entries == nil {
		entries = []TmuxMenuEntry{}
	}
	ensureMenuEntryDefaults(entries)
	used := make(map[string]struct{})
	assignMenuIDs(entries, used)

	content, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("encode menu: %w", err)
	}

	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	tmpPath := s.menuPath + ".tmp"
	if err := os.WriteFile(tmpPath, content, 0o600); err != nil {
		return fmt.Errorf("write temp menu: %w", err)
	}
	return os.Rename(tmpPath, s.menuPath)
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
		SchemaVersion: CurrentSchemaVersion,
		CodexBinary:   defaultCodexBin,
		ShellStatus:   defaultShellStatus(),
	}
}

func defaultShellStatus() ShellStatusConfig {
	return ShellStatusConfig{
		Left: ShellStatusRegion{
			Command:  defaultLeftCommand,
			Interval: defaultInterval,
		},
		Center: ShellStatusRegion{
			Command:  defaultCenterCommand,
			Interval: defaultInterval,
		},
		Right: ShellStatusRegion{
			Command:  defaultRightCommand,
			Interval: defaultInterval,
		},
		Layout: ShellStatusLayout{
			Left:   3,
			Center: 4,
			Right:  3,
		},
	}
}

func DefaultShellStatusConfig() ShellStatusConfig {
	return defaultShellStatus()
}

func (c *Config) applyDefaults() {
	if c.SchemaVersion <= 0 {
		c.SchemaVersion = CurrentSchemaVersion
	}
	if c.CodexBinary == "" {
		c.CodexBinary = defaultCodexBin
	}
	c.ensureShellDefaults()
}

func (c *Config) ensureShellDefaults() {
	if c.ShellStatus.Left.Command == "" {
		c.ShellStatus.Left.Command = defaultLeftCommand
	}
	if c.ShellStatus.Left.Interval <= 0 {
		c.ShellStatus.Left.Interval = defaultInterval
	}
	if c.ShellStatus.Center.Command == "" {
		c.ShellStatus.Center.Command = defaultCenterCommand
	}
	if c.ShellStatus.Center.Interval <= 0 {
		c.ShellStatus.Center.Interval = defaultInterval
	}
	if c.ShellStatus.Right.Command == "" {
		c.ShellStatus.Right.Command = defaultRightCommand
	}
	if c.ShellStatus.Right.Interval <= 0 {
		c.ShellStatus.Right.Interval = defaultInterval
	}
	if c.ShellStatus.Layout.Left <= 0 {
		c.ShellStatus.Layout.Left = 3
	}
	if c.ShellStatus.Layout.Center <= 0 {
		c.ShellStatus.Layout.Center = 4
	}
	if c.ShellStatus.Layout.Right <= 0 {
		c.ShellStatus.Layout.Right = 3
	}
}

func ensureMenuEntryDefaults(entries []TmuxMenuEntry) {
	for i := range entries {
		entry := &entries[i]
		if entry.Type == "" {
			if len(entry.Children) > 0 && entry.Command == "" {
				entry.Type = MenuEntryTypeMenu
			} else {
				entry.Type = MenuEntryTypeCommand
			}
		}
		if entry.Type == MenuEntryTypeMenu {
			ensureMenuEntryDefaults(entry.Children)
		} else if entry.Type == MenuEntryTypeCommand {
			ensureMenuEntryDefaults(entry.Children)
		}
	}
}

func assignMenuIDs(entries []TmuxMenuEntry, used map[string]struct{}) {
	for i := range entries {
		entry := &entries[i]
		if entry.ID == "" || idTaken(entry.ID, used) {
			entry.ID = nextMenuID(used)
		}
		used[entry.ID] = struct{}{}
		assignMenuIDs(entry.Children, used)
	}
}

func idTaken(id string, used map[string]struct{}) bool {
	_, exists := used[id]
	return exists
}

func nextMenuID(used map[string]struct{}) string {
	index := len(used) + 1
	for {
		candidate := fmt.Sprintf("entry-%d", index)
		if _, exists := used[candidate]; !exists {
			return candidate
		}
		index++
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

func defaultDataDir() (string, error) {
	if val := os.Getenv("XDG_DATA_HOME"); val != "" {
		return filepath.Join(val, appName), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home: %w", err)
	}

	return filepath.Join(home, ".local", "share", appName), nil
}

func defaultCacheDir() (string, error) {
	if val := os.Getenv("XDG_CACHE_HOME"); val != "" {
		return filepath.Join(val, appName), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home: %w", err)
	}

	return filepath.Join(home, ".cache", appName), nil
}
