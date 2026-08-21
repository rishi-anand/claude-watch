package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rishi/claude-watch/internal/claude"
	"github.com/rishi/claude-watch/internal/db"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

func makeSession(id, projectName string, startedAt, lastActive time.Time, msgs int) *claude.Session {
	s := &claude.Session{
		SessionID:    id,
		ProjectPath:  "/tmp/" + projectName,
		ProjectName:  projectName,
		StartedAt:    startedAt,
		LastActiveAt: lastActive,
	}
	for i := 0; i < msgs; i++ {
		s.Messages = append(s.Messages, claude.ParsedMessage{
			UUID:        fmt.Sprintf("u-%s-%d", id, i),
			MsgType:     "user",
			Role:        "user",
			ContentText: fmt.Sprintf("body %d", i),
			Timestamp:   lastActive,
			Seq:         i + 1,
		})
	}
	return s
}

func TestJSONLStateRoundTrip(t *testing.T) {
	database := openTestDB(t)

	off, mt, sid, err := GetJSONLState(database, "/tmp/no-such-file")
	if err != nil || off != 0 || mt != 0 || sid != "" {
		t.Fatalf("missing row must be zero-valued: off=%d mt=%v sid=%q err=%v", off, mt, sid, err)
	}

	if err := UpsertJSONLState(database, "/tmp/x.jsonl", "sess-1", 4096, 1700000000.5); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	off, mt, sid, err = GetJSONLState(database, "/tmp/x.jsonl")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if off != 4096 || sid != "sess-1" || mt != 1700000000.5 {
		t.Errorf("round-trip: got off=%d mt=%v sid=%q", off, mt, sid)
	}

	// Overwrite with a bigger offset — INSERT OR UPDATE should replace.
	if err := UpsertJSONLState(database, "/tmp/x.jsonl", "sess-1", 8192, 1700000001.0); err != nil {
		t.Fatalf("upsert 2: %v", err)
	}
	off, _, _, _ = GetJSONLState(database, "/tmp/x.jsonl")
	if off != 8192 {
		t.Errorf("overwrite: got off=%d want 8192", off)
	}
}

func TestGetLastSeq(t *testing.T) {
	database := openTestDB(t)
	if seq, err := GetLastSeq(database, "sess-empty"); err != nil || seq != 0 {
		t.Errorf("empty session: got seq=%d err=%v", seq, err)
	}
	if err := UpsertMessagesFast(database, "sess-1", []claude.ParsedMessage{
		{UUID: "a", MsgType: "user", Role: "user", ContentText: "hi", Timestamp: time.Now(), Seq: 1},
		{UUID: "b", MsgType: "user", Role: "user", ContentText: "there", Timestamp: time.Now(), Seq: 5},
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	seq, err := GetLastSeq(database, "sess-1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if seq != 5 {
		t.Errorf("got seq=%d want 5", seq)
	}
}

func TestUpsertMessagesFastSkipsFTS(t *testing.T) {
	database := openTestDB(t)
	err := UpsertMessagesFast(database, "sess-1", []claude.ParsedMessage{
		{UUID: "a", MsgType: "user", Role: "user", ContentText: "hello world", Timestamp: time.Now(), Seq: 1},
	})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	var msgCount, ftsCount int
	database.QueryRow("SELECT COUNT(*) FROM messages WHERE session_id='sess-1'").Scan(&msgCount)
	database.QueryRow("SELECT COUNT(*) FROM messages_fts WHERE session_id='sess-1'").Scan(&ftsCount)
	if msgCount != 1 {
		t.Errorf("messages: got %d want 1", msgCount)
	}
	if ftsCount != 0 {
		t.Errorf("fts: got %d want 0 (fast path must skip FTS)", ftsCount)
	}
}

func TestCatchUpFTSIndexesMissingRows(t *testing.T) {
	database := openTestDB(t)
	msgs := []claude.ParsedMessage{
		{UUID: "a", MsgType: "user", Role: "user", ContentText: "alpha bravo", Timestamp: time.Now(), Seq: 1},
		{UUID: "b", MsgType: "user", Role: "user", ContentText: "charlie delta", Timestamp: time.Now(), Seq: 2},
		{UUID: "c", MsgType: "user", Role: "user", ContentText: "", Timestamp: time.Now(), Seq: 3},
	}
	if err := UpsertMessagesFast(database, "sess-1", msgs); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if err := CatchUpFTS(database); err != nil {
		t.Fatalf("catch-up: %v", err)
	}
	var ftsCount int
	database.QueryRow("SELECT COUNT(*) FROM messages_fts WHERE session_id='sess-1'").Scan(&ftsCount)
	if ftsCount != 2 {
		t.Errorf("fts: got %d want 2 (empty content rows must be skipped)", ftsCount)
	}

	// Idempotent second call: no duplicates because rows are LEFT-JOIN filtered.
	if err := CatchUpFTS(database); err != nil {
		t.Fatalf("catch-up 2: %v", err)
	}
	database.QueryRow("SELECT COUNT(*) FROM messages_fts WHERE session_id='sess-1'").Scan(&ftsCount)
	if ftsCount != 2 {
		t.Errorf("fts after second catch-up: got %d want 2 (must not duplicate)", ftsCount)
	}
}

func TestUpdateSessionIncremental(t *testing.T) {
	database := openTestDB(t)
	// Seed a session with 2 messages and last_active_at at t0.
	t0 := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	initial := makeSession("sess-1", "proj", t0, t0.Add(1*time.Minute), 2)
	initial.Messages[0].ContentText = "first message body"
	if err := UpsertSession(database, initial, "/tmp/proj/sess-1.md", 1234.0); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := UpsertMessagesFast(database, "sess-1", initial.Messages); err != nil {
		t.Fatalf("seed msgs: %v", err)
	}

	// Now apply an incremental delta with 3 new messages, later last_active_at,
	// and a compaction boundary.
	delta := &claude.Session{
		SessionID:     "sess-1",
		ProjectName:   "proj",
		LastActiveAt:  t0.Add(1 * time.Hour),
		HasCompaction: true,
		Messages: []claude.ParsedMessage{
			{UUID: "u-new-1", MsgType: "user", Role: "user", ContentText: "new one", Timestamp: t0.Add(1 * time.Hour), Seq: 3},
			{UUID: "u-new-2", MsgType: "assistant", Role: "assistant", ContentText: "new two", Timestamp: t0.Add(1 * time.Hour), Seq: 4},
			{UUID: "u-new-3", MsgType: "user", Role: "user", ContentText: "new three", Timestamp: t0.Add(1 * time.Hour), Seq: 5},
		},
	}
	if err := UpdateSessionIncremental(database, delta, "/tmp/proj/sess-1.md", 2345.0); err != nil {
		t.Fatalf("update: %v", err)
	}

	row, err := GetSession(database, "sess-1")
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	// message_count should be seeded 2 + delta 3 = 5.
	if row.MessageCount != 5 {
		t.Errorf("message_count: got %d want 5", row.MessageCount)
	}
	if !row.HasCompaction {
		t.Error("has_compaction: expected true after delta")
	}
	if row.LastActiveAt.Before(t0.Add(59 * time.Minute)) {
		t.Errorf("last_active_at not advanced: got %v", row.LastActiveAt)
	}
	// StartedAt is one-time — must be untouched.
	if !row.StartedAt.Equal(t0) {
		t.Errorf("started_at: got %v want %v (must not be clobbered)", row.StartedAt, t0)
	}
	if row.LastMessage != "new three" {
		t.Errorf("last_message: got %q want %q", row.LastMessage, "new three")
	}
	// First message must be preserved from the initial upsert.
	if row.FirstMessage != "first message body" {
		t.Errorf("first_message: got %q want %q", row.FirstMessage, "first message body")
	}
}

func TestPruneOlderThan(t *testing.T) {
	database := openTestDB(t)
	dir := t.TempDir()

	// Old session — last_active_at 400 days ago.
	old := makeSession("old-1", "proj", time.Now().AddDate(-2, 0, 0), time.Now().AddDate(0, 0, -400), 3)
	oldMD := filepath.Join(dir, "old-1.md")
	os.WriteFile(oldMD, []byte("stale"), 0o644)
	if err := UpsertSession(database, old, oldMD, 0); err != nil {
		t.Fatalf("seed old: %v", err)
	}
	if err := UpsertMessagesFast(database, old.SessionID, old.Messages); err != nil {
		t.Fatalf("seed old msgs: %v", err)
	}
	if err := UpsertJSONLState(database, "/tmp/old.jsonl", old.SessionID, 100, 0); err != nil {
		t.Fatalf("seed old jsonl_state: %v", err)
	}
	CatchUpFTS(database)

	// Fresh session — active yesterday.
	fresh := makeSession("fresh-1", "proj", time.Now().AddDate(0, 0, -10), time.Now().AddDate(0, 0, -1), 2)
	freshMD := filepath.Join(dir, "fresh-1.md")
	os.WriteFile(freshMD, []byte("keep"), 0o644)
	if err := UpsertSession(database, fresh, freshMD, 0); err != nil {
		t.Fatalf("seed fresh: %v", err)
	}
	if err := UpsertMessagesFast(database, fresh.SessionID, fresh.Messages); err != nil {
		t.Fatalf("seed fresh msgs: %v", err)
	}
	if err := UpsertJSONLState(database, "/tmp/fresh.jsonl", fresh.SessionID, 200, 0); err != nil {
		t.Fatalf("seed fresh jsonl_state: %v", err)
	}
	CatchUpFTS(database)

	cutoff := time.Now().AddDate(0, 0, -180)
	pruned, err := PruneOlderThan(database, cutoff)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if pruned != 1 {
		t.Errorf("pruned count: got %d want 1", pruned)
	}

	// Old is fully gone.
	var n int
	database.QueryRow("SELECT COUNT(*) FROM sessions WHERE session_id='old-1'").Scan(&n)
	if n != 0 {
		t.Error("old session row not deleted")
	}
	database.QueryRow("SELECT COUNT(*) FROM messages WHERE session_id='old-1'").Scan(&n)
	if n != 0 {
		t.Error("old messages not deleted")
	}
	database.QueryRow("SELECT COUNT(*) FROM messages_fts WHERE session_id='old-1'").Scan(&n)
	if n != 0 {
		t.Error("old FTS not deleted")
	}
	database.QueryRow("SELECT COUNT(*) FROM jsonl_state WHERE session_id='old-1'").Scan(&n)
	if n != 0 {
		t.Error("old jsonl_state not deleted")
	}
	if _, err := os.Stat(oldMD); !os.IsNotExist(err) {
		t.Errorf("old MD file not deleted: %v", err)
	}

	// Fresh is fully retained.
	database.QueryRow("SELECT COUNT(*) FROM sessions WHERE session_id='fresh-1'").Scan(&n)
	if n != 1 {
		t.Error("fresh session was pruned")
	}
	if _, err := os.Stat(freshMD); err != nil {
		t.Errorf("fresh MD file was deleted: %v", err)
	}
}
