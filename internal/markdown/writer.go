package markdown

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rishi/claude-watch/internal/claude"
)

func WriteSession(dataDir string, session *claude.Session) (string, error) {
	dir := filepath.Join(dataDir, "sessions", session.ProjectName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	mdPath := filepath.Join(dir, session.SessionID+".md")

	var b strings.Builder
	writeFrontmatter(&b, session)

	for _, msg := range session.Messages {
		writeMessage(&b, &msg)
	}

	if err := os.WriteFile(mdPath, []byte(b.String()), 0o644); err != nil {
		return "", err
	}
	return mdPath, nil
}

func AppendMessages(mdPath string, session *claude.Session) error {
	existing, err := os.ReadFile(mdPath)
	if err != nil {
		return err
	}

	// Find the last seq in the existing file by counting ## headers
	lastSeq := countMessages(string(existing))

	var b strings.Builder
	for _, msg := range session.Messages {
		if msg.Seq <= lastSeq {
			continue
		}
		writeMessage(&b, &msg)
	}

	if b.Len() == 0 {
		// Update frontmatter for last_active_at
		return rewriteFrontmatter(mdPath, session, string(existing))
	}

	// Update frontmatter and append new messages
	if err := rewriteFrontmatter(mdPath, session, string(existing)); err != nil {
		return err
	}

	f, err := os.OpenFile(mdPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(b.String())
	return err
}

func writeFrontmatter(b *strings.Builder, session *claude.Session) {
	b.WriteString("---\n")
	fmt.Fprintf(b, "session_id: %s\n", session.SessionID)
	fmt.Fprintf(b, "project: %s\n", session.ProjectName)
	fmt.Fprintf(b, "project_path: %s\n", session.ProjectPath)
	if session.Slug != "" {
		fmt.Fprintf(b, "slug: %s\n", session.Slug)
	}
	if session.GitBranch != "" {
		fmt.Fprintf(b, "git_branch: %s\n", session.GitBranch)
	}
	fmt.Fprintf(b, "started_at: %s\n", session.StartedAt.UTC().Format("2006-01-02T15:04:05Z"))
	fmt.Fprintf(b, "last_active_at: %s\n", session.LastActiveAt.UTC().Format("2006-01-02T15:04:05Z"))
	if session.Model != "" {
		fmt.Fprintf(b, "model: %s\n", session.Model)
	}
	fmt.Fprintf(b, "has_compaction: %v\n", session.HasCompaction)
	b.WriteString("---\n\n")
}

func writeMessage(b *strings.Builder, msg *claude.ParsedMessage) {
	ts := msg.Timestamp.UTC().Format("2006-01-02 15:04:05")

	switch msg.MsgType {
	case "user":
		fmt.Fprintf(b, "## User · %s\n", ts)
		b.WriteString(msg.ContentText)
		b.WriteString("\n\n")
	case "assistant":
		fmt.Fprintf(b, "## Assistant · %s\n", ts)
		b.WriteString(msg.ContentText)
		b.WriteString("\n\n")
		// Write tool blocks from ContentJSON if present
		writeToolBlocks(b, msg.ContentJSON)
	case "compact_summary":
		b.WriteString("<details>\n<summary>Compaction Summary</summary>\n\n")
		b.WriteString(msg.ContentText)
		b.WriteString("\n</details>\n\n")
	case "compact_boundary":
		b.WriteString("---\n")
		fmt.Fprintf(b, "> COMPACTION · %s", ts)
		if msg.CompactTrigger != "" {
			fmt.Fprintf(b, " · trigger: %s", msg.CompactTrigger)
		}
		if msg.CompactTokens > 0 {
			fmt.Fprintf(b, " · tokens: %s", formatNumber(msg.CompactTokens))
		}
		b.WriteString("\n---\n\n")
	}
}

func writeToolBlocks(b *strings.Builder, contentJSON string) {
	if contentJSON == "" {
		return
	}
	// Parse content blocks to find tool_use entries
	var blocks []claude.ContentBlock
	if err := parseJSON(contentJSON, &blocks); err != nil {
		return
	}
	for _, block := range blocks {
		if block.Type == "tool_use" && block.Name != "" {
			fmt.Fprintf(b, "**Tool: %s**\n", block.Name)
			if len(block.Input) > 0 && string(block.Input) != "null" {
				b.WriteString("```json\n")
				b.WriteString(string(block.Input))
				b.WriteString("\n```\n\n")
			}
		}
	}
}

func parseJSON(s string, v interface{}) error {
	return json.Unmarshal([]byte(s), v)
}

func countMessages(content string) int {
	count := 0
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "## User ") || strings.HasPrefix(line, "## Assistant ") {
			count++
		}
		if strings.HasPrefix(line, "> COMPACTION") {
			count++
		}
		if strings.HasPrefix(line, "<details>") {
			// Check if it's a compaction summary
			count++
		}
	}
	return count
}

func rewriteFrontmatter(mdPath string, session *claude.Session, existing string) error {
	// Find end of frontmatter
	idx := strings.Index(existing[3:], "\n---\n")
	if idx < 0 {
		return nil
	}
	body := existing[idx+3+5:] // skip past closing ---\n

	var b strings.Builder
	writeFrontmatter(&b, session)
	b.WriteString(body)

	return os.WriteFile(mdPath, []byte(b.String()), 0o644)
}

func formatNumber(n int64) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	var result []byte
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(c))
	}
	return string(result)
}
