package claude

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"regexp"
	"strings"
	"time"
)

// System XML tags injected by Claude Code into user messages
var systemTagsRe = regexp.MustCompile(`(?s)<system-reminder>.*?</system-reminder>\s*|<task-notification>.*?</task-notification>\s*|<available-deferred-tools>.*?</available-deferred-tools>\s*|<user-prompt-submit-hook>.*?</user-prompt-submit-hook>\s*`)

func ParseJSONL(path string) (*Session, error) {
	s, _, err := ParseJSONLFrom(path, 0, 0)
	return s, err
}

// ParseJSONLFrom parses the JSONL file starting at startOffset, seeding the
// sequence counter with startSeq (so the first indexable message gets
// startSeq+1). Only complete newline-terminated lines are consumed — a
// trailing partial line at EOF is left for the next call. Returns the parsed
// session (populated only from the consumed lines), the byte offset just past
// the last consumed newline, and any read error.
func ParseJSONLFrom(path string, startOffset int64, startSeq int) (*Session, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, startOffset, err
	}
	defer f.Close()

	if startOffset > 0 {
		if _, err := f.Seek(startOffset, io.SeekStart); err != nil {
			return nil, startOffset, err
		}
	}

	r := bufio.NewReaderSize(f, 1<<20)
	session := &Session{}
	seq := startSeq
	offset := startOffset

	for {
		line, err := r.ReadBytes('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			return session, offset, err
		}
		offset += int64(len(line))
		trimmed := bytes.TrimRight(line, "\r\n")
		if len(trimmed) == 0 {
			continue
		}
		processJSONLLine(session, trimmed, &seq)
	}
	return session, offset, nil
}

func processJSONLLine(session *Session, line []byte, seq *int) {
	var entry Entry
	if err := json.Unmarshal(line, &entry); err != nil {
		return
	}

	// Set session ID from first entry that has one
	if session.SessionID == "" && entry.SessionID != "" {
		session.SessionID = entry.SessionID
	}

	// Set project path from first entry with cwd
	if session.ProjectPath == "" && entry.CWD != "" {
		session.ProjectPath = entry.CWD
		session.ProjectName = ProjectNameFromCWD(entry.CWD)
	}

	ts, _ := time.Parse(time.RFC3339Nano, entry.Timestamp)
	if !ts.IsZero() {
		if session.StartedAt.IsZero() || ts.Before(session.StartedAt) {
			session.StartedAt = ts
		}
		if ts.After(session.LastActiveAt) {
			session.LastActiveAt = ts
		}
	}

	switch entry.Type {
	case "progress", "queue-operation", "file-history-snapshot":
		return
	case "system":
		if entry.Subtype == "turn_duration" {
			return
		}
		if entry.Subtype == "compact_boundary" {
			session.HasCompaction = true
			*seq++
			session.Messages = append(session.Messages, ParsedMessage{
				UUID:      entry.UUID,
				MsgType:   "compact_boundary",
				Timestamp: ts,
				Seq:       *seq,
			})
		}
	case "user":
		if entry.Message == nil {
			return
		}
		*seq++
		msgType := "user"
		if entry.Message.IsCompactSummary {
			msgType = "compact_summary"
		}
		session.Messages = append(session.Messages, ParsedMessage{
			UUID:             entry.UUID,
			ParentUUID:       entry.ParentUUID,
			MsgType:          msgType,
			Role:             "user",
			ContentText:      ExtractText(entry.Message.Content),
			ContentJSON:      rawContentJSON(entry.Message.Content),
			IsCompactSummary: entry.Message.IsCompactSummary,
			IsSidechain:      entry.IsSidechain,
			Timestamp:        ts,
			Seq:              *seq,
		})
	case "assistant":
		if entry.Message == nil {
			return
		}
		if session.Model == "" && entry.Message.Model != "" {
			session.Model = entry.Message.Model
		}
		*seq++
		session.Messages = append(session.Messages, ParsedMessage{
			UUID:        entry.UUID,
			ParentUUID:  entry.ParentUUID,
			MsgType:     "assistant",
			Role:        "assistant",
			ContentText: ExtractText(entry.Message.Content),
			ContentJSON: rawContentJSON(entry.Message.Content),
			IsSidechain: entry.IsSidechain,
			Timestamp:   ts,
			Seq:         *seq,
		})
	}
}

func ExtractText(content ContentValue) string {
	if content.Text != "" {
		return stripSystemTags(content.Text)
	}
	var parts []string
	for _, block := range content.Blocks {
		if block.Type == "text" && block.Text != "" {
			parts = append(parts, block.Text)
		}
	}
	return stripSystemTags(strings.Join(parts, "\n"))
}

// stripSystemTags removes Claude Code internal XML tags from message content.
func stripSystemTags(s string) string {
	return strings.TrimSpace(systemTagsRe.ReplaceAllString(s, ""))
}

// stripSystemTagsFromRaw strips system tags from a json.RawMessage that may be
// a JSON string or an array of content blocks.
func stripSystemTagsFromRaw(raw json.RawMessage) json.RawMessage {
	// Try as string first
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		cleaned := stripSystemTags(s)
		if cleaned != s {
			out, _ := json.Marshal(cleaned)
			return out
		}
		return raw
	}
	// Try as array of {type, text} blocks
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &blocks); err == nil {
		changed := false
		for i := range blocks {
			if blocks[i].Text != "" {
				c := stripSystemTags(blocks[i].Text)
				if c != blocks[i].Text {
					blocks[i].Text = c
					changed = true
				}
			}
		}
		if changed {
			out, _ := json.Marshal(blocks)
			return out
		}
	}
	return raw
}

func rawContentJSON(content ContentValue) string {
	if len(content.Blocks) > 0 {
		// Strip system tags from text and tool_result blocks before serializing
		cleaned := make([]ContentBlock, len(content.Blocks))
		copy(cleaned, content.Blocks)
		for i := range cleaned {
			if cleaned[i].Type == "text" && cleaned[i].Text != "" {
				cleaned[i].Text = stripSystemTags(cleaned[i].Text)
			}
			if len(cleaned[i].Content) > 0 {
				cleaned[i].Content = stripSystemTagsFromRaw(cleaned[i].Content)
			}
		}
		data, err := json.Marshal(cleaned)
		if err == nil {
			return string(data)
		}
	}
	return ""
}

func ProjectNameFromCWD(cwd string) string {
	if cwd == "" {
		return ""
	}
	// Remove trailing slash
	cwd = strings.TrimRight(cwd, "/")
	idx := strings.LastIndex(cwd, "/")
	if idx >= 0 {
		return cwd[idx+1:]
	}
	return cwd
}
