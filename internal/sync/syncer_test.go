package sync

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rishi/claude-watch/internal/config"
	"github.com/rishi/claude-watch/internal/db"
	"github.com/rishi/claude-watch/internal/store"
)

type env struct {
	cfg      *config.Config
	db       *sql.DB
	jsonlDir string
	tmpDir   string
}

func newEnv(t *testing.T) *env {
	t.Helper()
	tmp := t.TempDir()
	claudeDir := filepath.Join(tmp, "claude")
	dataDir := filepath.Join(tmp, "data")
	jsonlDir := filepath.Join(claudeDir, "projects", "-tmp-proj")
	if err := os.MkdirAll(jsonlDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	cfg := &config.Config{
		DataDir:       dataDir,
		ClaudeDir:     claudeDir,
		Port:          0,
		RetentionDays: 180,
	}
	if err := os.MkdirAll(cfg.SessionsDir(), 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	database, err := db.Open(cfg.DBPath())
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return &env{cfg: cfg, db: database, jsonlDir: jsonlDir, tmpDir: tmp}
}

func appendEntries(t *testing.T, path string, entries []map[string]any) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()
	for _, e := range entries {
		b, _ := json.Marshal(e)
		if _, err := f.Write(append(b, '\n')); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
}

func mkUser(i int, sessionID, cwd string, ts time.Time) map[string]any {
	return map[string]any{
		"type":      "user",
		"uuid":      fmt.Sprintf("u-%d", i),
		"sessionId": sessionID,
		"cwd":       cwd,
		"timestamp": ts.Format(time.RFC3339Nano),
		"message":   map[string]any{"role": "user", "content": fmt.Sprintf("hello %d", i)},
	}
}

func TestSyncFromTranscriptFullThenNoOp(t *testing.T) {
	e := newEnv(t)
	path := filepath.Join(e.jsonlDir, "sess-a.jsonl")
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	appendEntries(t, path, []map[string]any{
		mkUser(1, "sess-a", "/tmp/proj", base),
		mkUser(2, "sess-a", "/tmp/proj", base.Add(1*time.Second)),
	})

	if err := SyncFromTranscript(e.cfg, e.db, path); err != nil {
		t.Fatalf("sync 1: %v", err)
	}

	var count int
	e.db.QueryRow("SELECT COUNT(*) FROM messages WHERE session_id='sess-a'").Scan(&count)
	if count != 2 {
		t.Fatalf("messages after sync 1: got %d want 2", count)
	}
	off, _, sid, err := store.GetJSONLState(e.db, path)
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	if sid != "sess-a" || off == 0 {
		t.Errorf("jsonl_state: got sid=%q off=%d", sid, off)
	}

	// No-op second sync — same file, same size.
	before := off
	if err := SyncFromTranscript(e.cfg, e.db, path); err != nil {
		t.Fatalf("sync 2: %v", err)
	}
	e.db.QueryRow("SELECT COUNT(*) FROM messages WHERE session_id='sess-a'").Scan(&count)
	if count != 2 {
		t.Errorf("no-op sync must not duplicate: got %d want 2", count)
	}
	after, _, _, _ := store.GetJSONLState(e.db, path)
	if after != before {
		t.Errorf("no-op sync must not change offset: before=%d after=%d", before, after)
	}
}

func TestSyncFromTranscriptIncremental(t *testing.T) {
	e := newEnv(t)
	path := filepath.Join(e.jsonlDir, "sess-b.jsonl")
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	appendEntries(t, path, []map[string]any{
		mkUser(1, "sess-b", "/tmp/proj", base),
		mkUser(2, "sess-b", "/tmp/proj", base.Add(1*time.Second)),
	})
	if err := SyncFromTranscript(e.cfg, e.db, path); err != nil {
		t.Fatalf("sync 1: %v", err)
	}

	// Now append two more entries — simulate a live transcript getting extended.
	appendEntries(t, path, []map[string]any{
		mkUser(3, "sess-b", "/tmp/proj", base.Add(2*time.Second)),
		mkUser(4, "sess-b", "/tmp/proj", base.Add(3*time.Second)),
	})
	if err := SyncFromTranscript(e.cfg, e.db, path); err != nil {
		t.Fatalf("sync 2: %v", err)
	}

	var count, maxSeq int
	e.db.QueryRow("SELECT COUNT(*) FROM messages WHERE session_id='sess-b'").Scan(&count)
	e.db.QueryRow("SELECT MAX(seq) FROM messages WHERE session_id='sess-b'").Scan(&maxSeq)
	if count != 4 {
		t.Errorf("messages: got %d want 4", count)
	}
	if maxSeq != 4 {
		t.Errorf("max seq: got %d want 4 (delta must continue seq from prior state)", maxSeq)
	}

	// message_count on the sessions row must equal 4 (initial 2 + delta 2).
	row, err := store.GetSession(e.db, "sess-b")
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if row.MessageCount != 4 {
		t.Errorf("sessions.message_count: got %d want 4", row.MessageCount)
	}

	// Compare against a fresh full parse in a separate env — same file, empty
	// state — should produce identical row counts and max seq.
	e2 := newEnv(t)
	path2 := filepath.Join(e2.jsonlDir, "sess-b.jsonl")
	// Copy the JSONL over so the fresh env sees the same input.
	src, _ := os.ReadFile(path)
	os.WriteFile(path2, src, 0o644)
	if err := SyncFromTranscript(e2.cfg, e2.db, path2); err != nil {
		t.Fatalf("fresh sync: %v", err)
	}
	var fCount, fMax int
	e2.db.QueryRow("SELECT COUNT(*) FROM messages WHERE session_id='sess-b'").Scan(&fCount)
	e2.db.QueryRow("SELECT MAX(seq) FROM messages WHERE session_id='sess-b'").Scan(&fMax)
	if fCount != count || fMax != maxSeq {
		t.Errorf("incremental result diverges from fresh: incremental=%d/%d fresh=%d/%d",
			count, maxSeq, fCount, fMax)
	}
}

func TestSyncFromTranscriptTruncationResets(t *testing.T) {
	e := newEnv(t)
	path := filepath.Join(e.jsonlDir, "sess-c.jsonl")
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	appendEntries(t, path, []map[string]any{
		mkUser(1, "sess-c", "/tmp/proj", base),
		mkUser(2, "sess-c", "/tmp/proj", base.Add(1*time.Second)),
		mkUser(3, "sess-c", "/tmp/proj", base.Add(2*time.Second)),
	})
	if err := SyncFromTranscript(e.cfg, e.db, path); err != nil {
		t.Fatalf("sync 1: %v", err)
	}
	off1, _, _, _ := store.GetJSONLState(e.db, path)
	if off1 == 0 {
		t.Fatalf("expected non-zero offset after sync 1")
	}

	// Truncate: write a shorter file. Offset > new file size must trigger
	// a restart from 0 on the next sync (rotation semantics).
	os.WriteFile(path, []byte{}, 0o644)
	appendEntries(t, path, []map[string]any{
		mkUser(10, "sess-c", "/tmp/proj", base.Add(1*time.Hour)),
	})

	if err := SyncFromTranscript(e.cfg, e.db, path); err != nil {
		t.Fatalf("sync 2: %v", err)
	}

	// After restart, jsonl_state's offset must equal the new (smaller) file size.
	fi, _ := os.Stat(path)
	off2, _, _, _ := store.GetJSONLState(e.db, path)
	if off2 != fi.Size() {
		t.Errorf("truncation restart: got off=%d want %d", off2, fi.Size())
	}
}
