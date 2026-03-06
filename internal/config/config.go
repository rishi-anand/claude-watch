package config

import (
	"os"
	"path/filepath"
	"strconv"
)

type Config struct {
	DataDir   string
	ClaudeDir string
	Port      int
	NoBrowser bool
}

func Load() *Config {
	home, _ := os.UserHomeDir()

	dataDir := os.Getenv("CLAUDE_WATCH_DIR")
	if dataDir == "" {
		dataDir = filepath.Join(home, "work", "claude-watch")
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

	return &Config{
		DataDir:   dataDir,
		ClaudeDir: claudeDir,
		Port:      port,
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
