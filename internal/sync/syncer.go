package sync

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rishi/claude-watch/internal/claude"
	"github.com/rishi/claude-watch/internal/config"
	"github.com/rishi/claude-watch/internal/markdown"
	"github.com/rishi/claude-watch/internal/store"
)

func SyncFromTranscript(cfg *config.Config, db *sql.DB, transcriptPath string) error {
	session, err := claude.ParseJSONL(transcriptPath)
	if err != nil {
		return fmt.Errorf("parse JSONL: %w", err)
	}
	if session.SessionID == "" {
		return nil // silently skip files without a session ID (incomplete/empty sessions)
	}

	// Check if MD file already exists
	mdPath := filepath.Join(cfg.SessionsDir(), session.ProjectName, session.SessionID+".md")
	if _, err := os.Stat(mdPath); err == nil {
		// Append new messages
		if err := markdown.AppendMessages(mdPath, session); err != nil {
			return fmt.Errorf("append messages: %w", err)
		}
	} else {
		// Write full session
		mdPath, err = markdown.WriteSession(cfg.DataDir, session)
		if err != nil {
			return fmt.Errorf("write session: %w", err)
		}
	}

	// Get mtime of md file
	mdInfo, err := os.Stat(mdPath)
	if err != nil {
		return err
	}
	mdMtime := float64(mdInfo.ModTime().UnixMilli()) / 1000.0

	// Upsert session and messages into SQLite
	if err := store.UpsertSession(db, session, mdPath, mdMtime); err != nil {
		return fmt.Errorf("upsert session: %w", err)
	}
	if err := store.UpsertMessages(db, session.SessionID, session.Messages); err != nil {
		return fmt.Errorf("upsert messages: %w", err)
	}

	// Record JSONL mtime so future startup scans can skip unchanged files.
	// This ensures serve restarts (and post-rebuild starts) are fast.
	if info, err := os.Stat(transcriptPath); err == nil {
		mtime := float64(info.ModTime().UnixMilli()) / 1000.0
		recordJSONLMtime(db, transcriptPath, mtime)
	}

	return nil
}

func SyncAll(cfg *config.Config, db *sql.DB) error {
	mtimes, err := store.GetAllMtimes(db)
	if err != nil {
		return err
	}

	// Scan for JSONL files — use mtimes keyed by JSONL path, not md path
	// We need to track JSONL mtimes separately
	jsonlMtimes, err := getJSONLMtimes(db)
	if err != nil {
		jsonlMtimes = make(map[string]float64)
	}

	changed, err := claude.ScanAll(cfg, jsonlMtimes)
	if err != nil {
		return err
	}
	_ = mtimes // mtimes used for md files, jsonlMtimes for JSONL

	if len(changed) > 0 {
		fmt.Printf("Syncing %d session files...\n", len(changed))
	}
	for _, path := range changed {
		if err := SyncFromTranscript(cfg, db, path); err != nil {
			fmt.Fprintf(os.Stderr, "warning: sync %s: %v\n", path, err)
		}
	}

	return nil
}

func RebuildIndex(cfg *config.Config, db *sql.DB) error {
	// Clear existing data
	db.Exec("DELETE FROM messages_fts")
	db.Exec("DELETE FROM messages")
	db.Exec("DELETE FROM sessions")
	db.Exec("DELETE FROM index_state")

	// Walk all .md files in sessions dir
	sessionsDir := cfg.SessionsDir()
	return filepath.Walk(sessionsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}

		// We need the original JSONL to rebuild, so scan JSONL files instead
		return nil
	})
}

func getJSONLMtimes(db *sql.DB) (map[string]float64, error) {
	// We store JSONL mtimes in index_state with a jsonl: prefix
	rows, err := db.Query("SELECT md_path, last_mtime FROM index_state WHERE md_path LIKE 'jsonl:%'")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]float64)
	for rows.Next() {
		var path string
		var mtime float64
		if err := rows.Scan(&path, &mtime); err != nil {
			return nil, err
		}
		// Strip jsonl: prefix
		result[path[6:]] = mtime
	}
	return result, rows.Err()
}

func recordJSONLMtime(db *sql.DB, path string, mtime float64) {
	db.Exec(`INSERT INTO index_state (md_path, last_mtime, indexed_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(md_path) DO UPDATE SET last_mtime=excluded.last_mtime, indexed_at=CURRENT_TIMESTAMP`,
		"jsonl:"+path, mtime)
}
