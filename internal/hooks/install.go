package hooks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rishi/claude-watch/internal/config"
)

var hookEvents = []struct {
	event    string
	filename string
	cliArg   string
}{
	{"SessionStart", "session-start.sh", "session-start"},
	{"UserPromptSubmit", "prompt.sh", "prompt"},
	{"Stop", "stop.sh", "stop"},
	{"PreCompact", "compact.sh", "compact"},
	{"SessionEnd", "session-end.sh", "session-end"},
}

func Install(cfg *config.Config) error {
	hooksDir := cfg.HooksDir()
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		return err
	}

	binaryPath, err := os.Executable()
	if err != nil {
		return err
	}

	// Write hook scripts
	for _, h := range hookEvents {
		scriptPath := filepath.Join(hooksDir, h.filename)
		script := fmt.Sprintf("#!/usr/bin/env bash\ncat | %s hook %s\n", binaryPath, h.cliArg)
		if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
			return err
		}
	}

	// Merge into settings.json
	return mergeSettings(cfg)
}

func mergeSettings(cfg *config.Config) error {
	settingsPath := cfg.ClaudeSettingsPath()
	hooksDir := cfg.HooksDir()

	// Read existing settings
	var settings map[string]interface{}
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			settings = make(map[string]interface{})
		} else {
			return err
		}
	} else {
		if err := json.Unmarshal(data, &settings); err != nil {
			return err
		}
	}

	// Get or create hooks map
	hooksMap, ok := settings["hooks"].(map[string]interface{})
	if !ok {
		hooksMap = make(map[string]interface{})
	}

	// Add our hooks (don't replace existing ones, append)
	for _, h := range hookEvents {
		scriptPath := filepath.Join(hooksDir, h.filename)
		ourHook := map[string]interface{}{
			"type":    "command",
			"command": scriptPath,
		}

		existing, ok := hooksMap[h.event].([]interface{})
		if !ok {
			existing = nil
		}

		// Check if we already have this hook installed
		alreadyInstalled := false
		for _, entry := range existing {
			entryMap, ok := entry.(map[string]interface{})
			if !ok {
				continue
			}
			hooks, ok := entryMap["hooks"].([]interface{})
			if !ok {
				continue
			}
			for _, hook := range hooks {
				hookMap, ok := hook.(map[string]interface{})
				if !ok {
					continue
				}
				if hookMap["command"] == scriptPath {
					alreadyInstalled = true
					break
				}
			}
			if alreadyInstalled {
				break
			}
		}

		if !alreadyInstalled {
			newEntry := map[string]interface{}{
				"hooks": []interface{}{ourHook},
			}
			existing = append(existing, newEntry)
			hooksMap[h.event] = existing
		}
	}

	settings["hooks"] = hooksMap

	// Write atomically
	newData, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}

	tmpPath := settingsPath + ".tmp"
	if err := os.WriteFile(tmpPath, newData, 0o644); err != nil {
		return err
	}
	return os.Rename(tmpPath, settingsPath)
}
