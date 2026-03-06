package setup

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rishi/claude-watch/internal/config"
)

// ConfigFile is the path to the persisted user config.
func ConfigFilePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude-watch", "config.json")
}

// IsFirstRun returns true if the config file doesn't exist yet.
func IsFirstRun() bool {
	_, err := os.Stat(ConfigFilePath())
	return os.IsNotExist(err)
}

// Run interactively walks the user through first-time setup.
// It updates cfg in place and saves ~/.claude-watch/config.json.
// Returns whether the user confirmed hook installation.
func Run(cfg *config.Config) (installHooks bool, err error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println()
	fmt.Println("┌─────────────────────────────────────────┐")
	fmt.Println("│        claude-watch  first-run setup     │")
	fmt.Println("└─────────────────────────────────────────┘")
	fmt.Println()

	// --- Step 1: Data directory ---
	home, _ := os.UserHomeDir()
	defaultDataDir := filepath.Join(home, "claude-watch")

	fmt.Printf("Where should claude-watch store session files?\n")
	fmt.Printf("  Press Enter to use the default, or type a custom path.\n")
	fmt.Printf("  Default: %s\n", defaultDataDir)
	fmt.Printf("  Path: ")

	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		line = defaultDataDir
	}
	// Expand ~ manually
	if strings.HasPrefix(line, "~/") {
		line = filepath.Join(home, line[2:])
	}
	cfg.DataDir = line
	fmt.Printf("  ✓ Sessions will be stored in: %s\n", cfg.DataDir)
	fmt.Println()

	// --- Step 2: Show proposed settings.json changes ---
	settingsPath := cfg.ClaudeSettingsPath()
	hooksDir := cfg.HooksDir()
	binaryPath, _ := os.Executable()

	// Build the proposed hook entries
	type hookEntry struct {
		Type    string `json:"type"`
		Command string `json:"command"`
	}
	type hookGroup struct {
		Hooks []hookEntry `json:"hooks"`
	}

	events := []struct{ event, file, arg string }{
		{"SessionStart", "session-start.sh", "session-start"},
		{"UserPromptSubmit", "prompt.sh", "prompt"},
		{"Stop", "stop.sh", "stop"},
		{"PreCompact", "compact.sh", "compact"},
		{"SessionEnd", "session-end.sh", "session-end"},
	}

	proposedHooks := make(map[string][]hookGroup)
	for _, e := range events {
		scriptPath := filepath.Join(hooksDir, e.file)
		proposedHooks[e.event] = []hookGroup{
			{Hooks: []hookEntry{{Type: "command", Command: scriptPath}}},
		}
	}

	// Read existing settings
	fmt.Printf("I need to update your Claude Code settings to install hooks.\n")
	fmt.Printf("File: %s\n\n", settingsPath)

	existingData, readErr := os.ReadFile(settingsPath)
	if readErr == nil {
		fmt.Println("  Current file contents:")
		fmt.Println("  " + strings.ReplaceAll(string(existingData), "\n", "\n  "))
		fmt.Println()
	} else {
		fmt.Println("  (file does not exist yet — will be created)")
		fmt.Println()
	}

	fmt.Println("  I will add the following hooks entries:")
	fmt.Println()
	for _, e := range events {
		scriptPath := filepath.Join(hooksDir, e.file)
		fmt.Printf("  %-20s → %s\n", e.event, scriptPath)
	}
	fmt.Printf("\n  Hook scripts call: %s hook <event>\n", binaryPath)
	fmt.Println()

	// Show the full resulting settings.json
	var existing map[string]interface{}
	if readErr == nil {
		json.Unmarshal(existingData, &existing)
	}
	if existing == nil {
		existing = make(map[string]interface{})
	}
	hooksMap, _ := existing["hooks"].(map[string]interface{})
	if hooksMap == nil {
		hooksMap = make(map[string]interface{})
	}
	for k, v := range proposedHooks {
		raw, _ := json.Marshal(v)
		var arr []interface{}
		json.Unmarshal(raw, &arr)
		hooksMap[k] = arr
	}
	existing["hooks"] = hooksMap
	preview, _ := json.MarshalIndent(existing, "", "  ")
	fmt.Println("  Result after update:")
	fmt.Println("  " + strings.ReplaceAll(string(preview), "\n", "\n  "))
	fmt.Println()

	// --- Step 3: Confirm ---
	fmt.Printf("May I update %s with these changes? [Y/n]: ", settingsPath)
	ans, _ := reader.ReadString('\n')
	ans = strings.TrimSpace(strings.ToLower(ans))
	installHooks = ans == "" || ans == "y" || ans == "yes"

	if !installHooks {
		fmt.Println()
		fmt.Println("  Skipping hook installation.")
		fmt.Println("  You can install hooks later by running: claude-watch serve")
		fmt.Println("  (it will ask again)")
	} else {
		fmt.Println("  ✓ Will update settings.json")
	}
	fmt.Println()

	// --- Step 4: Save config ---
	if err := saveConfig(cfg, installHooks); err != nil {
		return installHooks, fmt.Errorf("save config: %w", err)
	}

	fmt.Printf("  ✓ Saved config to: %s\n", ConfigFilePath())
	fmt.Println()

	return installHooks, nil
}

// LoadSaved reads ~/.claude-watch/config.json and applies saved values to cfg.
// Returns false if the config file does not exist.
func LoadSaved(cfg *config.Config) bool {
	data, err := os.ReadFile(ConfigFilePath())
	if err != nil {
		return false
	}
	var saved savedConfig
	if err := json.Unmarshal(data, &saved); err != nil {
		return false
	}
	if saved.DataDir != "" {
		cfg.DataDir = saved.DataDir
	}
	if saved.Port != 0 {
		cfg.Port = saved.Port
	}
	return true
}

// HooksInstalled returns true if the saved config records hooks as installed
// AND the hook scripts actually exist on disk.
func HooksInstalled(cfg *config.Config) bool {
	data, err := os.ReadFile(ConfigFilePath())
	if err != nil {
		return false
	}
	var saved savedConfig
	if err := json.Unmarshal(data, &saved); err != nil {
		return false
	}
	if !saved.HooksInstalled {
		return false
	}
	// Verify scripts still exist on disk
	hooksDir := cfg.HooksDir()
	scripts := []string{"session-start.sh", "prompt.sh", "stop.sh", "compact.sh", "session-end.sh"}
	for _, s := range scripts {
		if _, err := os.Stat(filepath.Join(hooksDir, s)); os.IsNotExist(err) {
			return false
		}
	}
	return true
}

type savedConfig struct {
	DataDir      string `json:"data_dir"`
	Port         int    `json:"port,omitempty"`
	HooksInstalled bool `json:"hooks_installed"`
}

func saveConfig(cfg *config.Config, hooksInstalled bool) error {
	cfgPath := ConfigFilePath()
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		return err
	}
	saved := savedConfig{
		DataDir:      cfg.DataDir,
		Port:         cfg.Port,
		HooksInstalled: hooksInstalled,
	}
	data, err := json.MarshalIndent(saved, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(cfgPath, data, 0o644)
}
