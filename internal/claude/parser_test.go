package claude

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeJSONL(t *testing.T, entries ...map[string]any) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "transcript.jsonl")
	f, err := os.Create(p)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer f.Close()
	for _, e := range entries {
		b, err := json.Marshal(e)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if _, err := f.Write(append(b, '\n')); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	return p
}

func userEntry(i int, sessionID string, ts time.Time) map[string]any {
	return map[string]any{
		"type":       "user",
		"uuid":       fmt.Sprintf("u-%d", i),
		"sessionId":  sessionID,
		"cwd":        "/tmp/proj",
		"timestamp":  ts.Format(time.RFC3339Nano),
		"parentUuid": "",
		"message": map[string]any{
			"role":    "user",
			"content": fmt.Sprintf("hello %d", i),
		},
	}
}

func assistantEntry(i int, sessionID string, ts time.Time) map[string]any {
	return map[string]any{
		"type":      "assistant",
		"uuid":      fmt.Sprintf("a-%d", i),
		"sessionId": sessionID,
		"cwd":       "/tmp/proj",
		"timestamp": ts.Format(time.RFC3339Nano),
		"message": map[string]any{
			"role":    "assistant",
			"model":   "claude-opus-4-7",
			"content": []map[string]any{{"type": "text", "text": fmt.Sprintf("reply %d", i)}},
		},
	}
}

func TestParseJSONLFromFullFile(t *testing.T) {
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	path := writeJSONL(t,
		userEntry(1, "sess-a", base),
		assistantEntry(1, "sess-a", base.Add(1*time.Second)),
		userEntry(2, "sess-a", base.Add(2*time.Second)),
	)

	sess, offset, err := ParseJSONLFrom(path, 0, 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got, want := len(sess.Messages), 3; got != want {
		t.Errorf("messages: got %d want %d", got, want)
	}
	if sess.SessionID != "sess-a" {
		t.Errorf("session id: got %q want sess-a", sess.SessionID)
	}
	if sess.Model != "claude-opus-4-7" {
		t.Errorf("model: got %q", sess.Model)
	}
	fi, _ := os.Stat(path)
	if offset != fi.Size() {
		t.Errorf("offset: got %d want %d", offset, fi.Size())
	}
	if sess.Messages[2].Seq != 3 {
		t.Errorf("seq: last msg got %d want 3", sess.Messages[2].Seq)
	}
}

func TestParseJSONLFromResumesFromOffsetAndSeq(t *testing.T) {
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	path := writeJSONL(t,
		userEntry(1, "sess-a", base),
		assistantEntry(1, "sess-a", base.Add(1*time.Second)),
		userEntry(2, "sess-a", base.Add(2*time.Second)),
		assistantEntry(2, "sess-a", base.Add(3*time.Second)),
	)

	// First pass: parse only the first 2 entries by stopping at their offset.
	head, headOff, err := ParseJSONLFrom(path, 0, 0)
	if err != nil {
		t.Fatalf("parse full: %v", err)
	}
	if len(head.Messages) != 4 {
		t.Fatalf("setup: full parse should return 4 messages, got %d", len(head.Messages))
	}

	// Compute offset after the first 2 lines.
	f, _ := os.Open(path)
	buf := make([]byte, headOff)
	f.Read(buf)
	f.Close()
	var twoLinesEnd int64
	newlines := 0
	for i, b := range buf {
		if b == '\n' {
			newlines++
			if newlines == 2 {
				twoLinesEnd = int64(i + 1)
				break
			}
		}
	}
	if twoLinesEnd == 0 {
		t.Fatalf("could not locate offset after two lines")
	}

	// Second pass: resume from that offset with seq seeded at 2.
	tail, tailOff, err := ParseJSONLFrom(path, twoLinesEnd, 2)
	if err != nil {
		t.Fatalf("parse tail: %v", err)
	}
	if got, want := len(tail.Messages), 2; got != want {
		t.Errorf("tail messages: got %d want %d", got, want)
	}
	if tail.Messages[0].Seq != 3 || tail.Messages[1].Seq != 4 {
		t.Errorf("tail seqs: got %d,%d want 3,4",
			tail.Messages[0].Seq, tail.Messages[1].Seq)
	}
	if tailOff != headOff {
		t.Errorf("tail offset: got %d want %d", tailOff, headOff)
	}
}

func TestParseJSONLFromDoesNotConsumePartialTrailingLine(t *testing.T) {
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	path := writeJSONL(t,
		userEntry(1, "sess-a", base),
		userEntry(2, "sess-a", base.Add(1*time.Second)),
	)
	fi, _ := os.Stat(path)
	completeEnd := fi.Size()

	// Append a partial line (no trailing newline) — a writer mid-flush.
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	f.WriteString(`{"type":"user","sessionId":"sess-a","cwd":"/tmp/proj","uuid":"partia`)
	f.Close()

	sess, offset, err := ParseJSONLFrom(path, 0, 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got, want := len(sess.Messages), 2; got != want {
		t.Errorf("messages: got %d want %d (partial line must not be consumed)", got, want)
	}
	if offset != completeEnd {
		t.Errorf("offset: got %d want %d (must stop at last complete newline)", offset, completeEnd)
	}
}

func TestParseJSONLFromEmptyAndBlankLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "blank.jsonl")
	// Blank lines mixed with an entry.
	os.WriteFile(path, []byte("\n\n"), 0o644)
	sess, offset, err := ParseJSONLFrom(path, 0, 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(sess.Messages) != 0 {
		t.Errorf("blank-only file: expected 0 messages, got %d", len(sess.Messages))
	}
	if offset != 2 {
		t.Errorf("blank-only file: expected offset 2 (both newlines), got %d", offset)
	}
}

func TestParseJSONLPreservesLegacySignature(t *testing.T) {
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	path := writeJSONL(t,
		userEntry(1, "sess-a", base),
		assistantEntry(1, "sess-a", base.Add(1*time.Second)),
	)
	// The legacy wrapper is used by main.go's rebuild/list/export paths;
	// verify it still returns the full session.
	sess, err := ParseJSONL(path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(sess.Messages) != 2 {
		t.Errorf("got %d messages, want 2", len(sess.Messages))
	}
}
