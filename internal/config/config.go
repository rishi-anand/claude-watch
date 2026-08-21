package config

import (
	"os"
	"path/filepath"
	"strconv"
)

type Config struct {
	DataDir        string
	ClaudeDir      string
	Port           int
	NoBrowser      bool
	RetentionDays  int
}

// DefaultRetentionDays is the built-in retention window in days. Six months
// of history keeps the DB and MD store manageable — sessions with a
// last_active_at older than this are pruned by serve.
const DefaultRetentionDays = 180

func Load() *Config {
	home, _ := os.UserHomeDir()

	dataDir := os.Getenv("CLAUDE_WATCH_DIR")
	if dataDir == "" {
		dataDir = filepath.Join(home, "claude-watch")
	}

	claudeDir := os.Getenv("CLAUDE_DIR")
	if claudeDir == "" {
		claudeDir = filepath.Join(home, ".claude")
	}

	port := 7823
	if p := os.Getenv("CLAUDE_WATCH_PORT"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			port = v
		}
	}

	retention := DefaultRetentionDays
	if r := os.Getenv("CLAUDE_WATCH_RETENTION_DAYS"); r != "" {
		if v, err := strconv.Atoi(r); err == nil && v > 0 {
			retention = v
		}
	}

	return &Config{
		DataDir:       dataDir,
		ClaudeDir:     claudeDir,
		Port:          port,
		RetentionDays: retention,
	}
}

func (c *Config) SessionsDir() string {
	return filepath.Join(c.DataDir, "sessions")
}

func (c *Config) DBPath() string {
	return filepath.Join(c.DataDir, "claude-watch.db")
}

func (c *Config) HooksDir() string {
	return filepath.Join(c.DataDir, "hooks")
}

func (c *Config) ClaudeProjectsDir() string {
	return filepath.Join(c.ClaudeDir, "projects")
}

func (c *Config) ClaudeSettingsPath() string {
	return filepath.Join(c.ClaudeDir, "settings.json")
}
