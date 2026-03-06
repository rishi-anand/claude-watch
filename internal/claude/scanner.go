package claude

import (
	"os"
	"path/filepath"

	"github.com/rishi/claude-watch/internal/config"
)

func ScanAll(cfg *config.Config, lastMtimes map[string]float64) ([]string, error) {
	projectsDir := cfg.ClaudeProjectsDir()
	var changed []string

	err := filepath.Walk(projectsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip errors
		}
		if info.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".jsonl" {
			return nil
		}

		mtime := float64(info.ModTime().UnixMilli()) / 1000.0
		if lastMtime, ok := lastMtimes[path]; ok && mtime <= lastMtime {
			return nil // unchanged
		}

		changed = append(changed, path)
		return nil
	})

	return changed, err
}
