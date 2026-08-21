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
	fi, err := os.Stat(transcriptPath)
	if err != nil {
		return err
	}
	fileSize := fi.Size()
	curMtime := float64(fi.ModTime().UnixMilli()) / 1000.0

	prevOffset, _, prevSessionID, err := store.GetJSONLState(db, transcriptPath)
	if err != nil {
		return fmt.Errorf("read jsonl state: %w", err)
	}
	// File shrank (rotated / truncated) — restart from the beginning.
	if prevOffset > fileSize {
		prevOffset = 0
		prevSessionID = ""
	}
	// Nothing new since the last sync.
	if prevOffset > 0 && prevOffset == fileSize {
		return nil
	}

	if prevOffset == 0 || prevSessionID == "" {
		return syncFull(cfg, db, transcriptPath, curMtime)
	}
	return syncIncremental(cfg, db, transcriptPath, prevOffset, prevSessionID, curMtime)
}

func syncFull(cfg *config.Config, db *sql.DB, transcriptPath string, mtime float64) error {
	session, newOffset, err := claude.ParseJSONLFrom(transcriptPath, 0, 0)
	if err != nil {
		return fmt.Errorf("parse JSONL: %w", err)
	}
	if session.SessionID == "" {
		return nil
	}

	mdPath := filepath.Join(cfg.SessionsDir(), session.ProjectName, session.SessionID+".md")
	if _, err := os.Stat(mdPath); err == nil {
		if err := markdown.AppendMessages(mdPath, session); err != nil {
			return fmt.Errorf("append messages: %w", err)
		}
	} else {
		mdPath, err = markdown.WriteSession(cfg.DataDir, session)
		if err != nil {
			return fmt.Errorf("write session: %w", err)
		}
	}

	mdInfo, err := os.Stat(mdPath)
	if err != nil {
		return err
	}
	mdMtime := float64(mdInfo.ModTime().UnixMilli()) / 1000.0
	if err := store.UpsertSession(db, session, mdPath, mdMtime); err != nil {
		return fmt.Errorf("upsert session: %w", err)
	}
	if err := store.UpsertMessagesFast(db, session.SessionID, session.Messages); err != nil {
		return fmt.Errorf("upsert messages: %w", err)
	}
	_ = store.UpsertJSONLState(db, transcriptPath, session.SessionID, newOffset, mtime)
	return nil
}

func syncIncremental(cfg *config.Config, db *sql.DB, transcriptPath string, prevOffset int64, sessionID string, mtime float64) error {
	lastSeq, _ := store.GetLastSeq(db, sessionID)
	delta, newOffset, err := claude.ParseJSONLFrom(transcriptPath, prevOffset, lastSeq)
	if err != nil {
		return fmt.Errorf("parse JSONL tail: %w", err)
	}
	if newOffset == prevOffset {
		return nil
	}
	if delta.SessionID == "" {
		delta.SessionID = sessionID
	}
	if delta.ProjectName == "" {
		row, err := store.GetSession(db, sessionID)
		if err != nil || row == nil {
			// Sessions row is missing — force a full resync next time.
			_ = store.UpsertJSONLState(db, transcriptPath, sessionID, 0, mtime)
			return nil
		}
		delta.ProjectName = row.ProjectName
		delta.ProjectPath = row.ProjectPath
	}

	if len(delta.Messages) == 0 {
		// No indexable rows in the delta — still advance the offset so we
		// don't rescan these bytes next time.
		_ = store.UpsertJSONLState(db, transcriptPath, sessionID, newOffset, mtime)
		return nil
	}

	mdPath := filepath.Join(cfg.SessionsDir(), delta.ProjectName, delta.SessionID+".md")
	if _, err := os.Stat(mdPath); err != nil {
		// MD file was removed out from under us — start over.
		_ = store.UpsertJSONLState(db, transcriptPath, sessionID, 0, mtime)
		return syncFull(cfg, db, transcriptPath, mtime)
	}
	if err := markdown.AppendMessages(mdPath, delta); err != nil {
		return fmt.Errorf("append messages: %w", err)
	}
	mdInfo, err := os.Stat(mdPath)
	if err != nil {
		return err
	}
	mdMtime := float64(mdInfo.ModTime().UnixMilli()) / 1000.0
	if err := store.UpsertMessagesFast(db, delta.SessionID, delta.Messages); err != nil {
		return fmt.Errorf("upsert messages: %w", err)
	}
	if err := store.UpdateSessionIncremental(db, delta, mdPath, mdMtime); err != nil {
		return fmt.Errorf("update session: %w", err)
	}
	_ = store.UpsertJSONLState(db, transcriptPath, delta.SessionID, newOffset, mtime)
	return nil
}

func SyncAll(cfg *config.Config, db *sql.DB) error {
	jsonlMtimes, err := store.GetAllJSONLMtimes(db)
	if err != nil {
		jsonlMtimes = make(map[string]float64)
	}
	changed, err := claude.ScanAll(cfg, jsonlMtimes)
	if err != nil {
		return err
	}

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

// RebuildIndex is kept for API compatibility. Rebuilding is handled by the
// rebuild command via SyncFromTranscript + store.RebuildFTS.
func RebuildIndex(cfg *config.Config, db *sql.DB) error {
	return nil
}
