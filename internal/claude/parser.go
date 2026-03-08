package claude

import (
	"bufio"
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"time"
)

// System XML tags injected by Claude Code into user messages
var systemTagsRe = regexp.MustCompile(`(?s)<system-reminder>.*?</system-reminder>\s*|<task-notification>.*?</task-notification>\s*|<available-deferred-tools>.*?</available-deferred-tools>\s*|<user-prompt-submit-hook>.*?</user-prompt-submit-hook>\s*`)

func ParseJSONL(path string) (*Session, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 10*1024*1024), 10*1024*1024)

	session := &Session{}
	seq := 0

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var entry Entry
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
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

		// Parse timestamp
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
			continue
		case "system":
			if entry.Subtype == "turn_duration" {
				continue
			}
			if entry.Subtype == "compact_boundary" {
				session.HasCompaction = true
				seq++
				msg := ParsedMessage{
					UUID:      entry.UUID,
					MsgType:   "compact_boundary",
					Timestamp: ts,
					Seq:       seq,
				}
				session.Messages = append(session.Messages, msg)
			}
			continue
		case "user":
			if entry.Message == nil {
				continue
			}
			seq++
			msgType := "user"
			if entry.Message.IsCompactSummary {
				msgType = "compact_summary"
			}
			contentText := ExtractText(entry.Message.Content)
			contentJSON := rawContentJSON(entry.Message.Content)
			msg := ParsedMessage{
				UUID:             entry.UUID,
				ParentUUID:       entry.ParentUUID,
				MsgType:          msgType,
				Role:             "user",
				ContentText:      contentText,
				ContentJSON:      contentJSON,
				IsCompactSummary: entry.Message.IsCompactSummary,
				IsSidechain:      entry.IsSidechain,
				Timestamp:        ts,
				Seq:              seq,
			}
			session.Messages = append(session.Messages, msg)
		case "assistant":
			if entry.Message == nil {
				continue
			}
			// Set model from first assistant message
			if session.Model == "" && entry.Message.Model != "" {
				session.Model = entry.Message.Model
			}
			seq++
			contentText := ExtractText(entry.Message.Content)
			contentJSON := rawContentJSON(entry.Message.Content)
			msg := ParsedMessage{
				UUID:        entry.UUID,
				ParentUUID:  entry.ParentUUID,
				MsgType:     "assistant",
				Role:        "assistant",
				ContentText: contentText,
				ContentJSON: contentJSON,
				IsSidechain: entry.IsSidechain,
				Timestamp:   ts,
				Seq:         seq,
			}
			session.Messages = append(session.Messages, msg)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return session, nil
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
