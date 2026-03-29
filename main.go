package main

import (
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/rishi/claude-watch/internal/api"
	"github.com/rishi/claude-watch/internal/claude"
	"github.com/rishi/claude-watch/internal/config"
	"github.com/rishi/claude-watch/internal/db"
	"github.com/rishi/claude-watch/internal/hooks"
	"github.com/rishi/claude-watch/internal/setup"
	"github.com/rishi/claude-watch/internal/store"
	cwsync "github.com/rishi/claude-watch/internal/sync"
)

//go:embed static
var staticEmbed embed.FS

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "serve":
		cmdServe()
	case "hook":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: claude-watch hook <event>")
			os.Exit(1)
		}
		cmdHook(os.Args[2])
	case "rebuild":
		cmdRebuild()
	case "list":
		cmdList()
	case "export":
		cmdExport()
	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "Usage: claude-watch <command>")
	fmt.Fprintln(os.Stderr, "  serve     Start HTTP server")
	fmt.Fprintln(os.Stderr, "  hook      Process hook event (reads JSON from stdin)")
	fmt.Fprintln(os.Stderr, "  rebuild   Force rebuild SQLite index")
	fmt.Fprintln(os.Stderr, "  list      List sessions (reads JSONL directly, no SQLite)")
	fmt.Fprintln(os.Stderr, "  export    Export session to detailed markdown (no SQLite)")
}

func cmdServe() {
	cfg := config.Load()
	parseServeFlags(cfg)

	// Apply saved config (overrides env defaults if config file exists)
	setup.LoadSaved(cfg)

	// First-run interactive setup
	if setup.IsFirstRun() {
		if _, err := setup.Run(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "warning: setup: %v\n", err)
		}
	}

	// Ensure directories exist
	os.MkdirAll(cfg.SessionsDir(), 0o755)
	os.MkdirAll(cfg.HooksDir(), 0o755)

	// Install hooks only if not already installed (scripts missing or not confirmed yet)
	if !setup.HooksInstalled(cfg) {
		if err := hooks.Install(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "warning: install hooks: %v\n", err)
		}
	}

	// Open database
	database, err := db.Open(cfg.DBPath())
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: open database: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	// Initial full scan
	fmt.Println("Scanning sessions...")
	if err := cwsync.SyncAll(cfg, database); err != nil {
		fmt.Fprintf(os.Stderr, "warning: sync: %v\n", err)
	}

	// Auto-rebuild FTS if it's out of sync with the messages table
	var msgCount, ftsCount int
	database.QueryRow("SELECT COUNT(*) FROM messages WHERE content_text != ''").Scan(&msgCount)
	database.QueryRow("SELECT COUNT(*) FROM messages_fts").Scan(&ftsCount)
	if ftsCount < msgCount {
		fmt.Printf("Rebuilding search index (%d/%d messages indexed)...\n", ftsCount, msgCount)
		if err := store.RebuildFTS(database); err != nil {
			fmt.Fprintf(os.Stderr, "warning: rebuild FTS: %v\n", err)
		}
	}

	var count int
	database.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&count)
	fmt.Printf("Indexed %d sessions\n", count)

	addr := fmt.Sprintf(":%d", cfg.Port)
	fmt.Printf("Ready at http://localhost:%d\n", cfg.Port)

	// Open browser after short delay
	if !cfg.NoBrowser {
		go func() {
			time.Sleep(500 * time.Millisecond)
			openBrowser(fmt.Sprintf("http://localhost:%d", cfg.Port))
		}()
	}

	staticSub, _ := fs.Sub(staticEmbed, "static")
	server := api.NewServer(cfg, database, staticSub)
	if err := http.ListenAndServe(addr, server); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func parseServeFlags(cfg *config.Config) {
	for i := 2; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--port":
			if i+1 < len(os.Args) {
				i++
				if p, err := strconv.Atoi(os.Args[i]); err == nil {
					cfg.Port = p
				}
			}
		case "--no-browser":
			cfg.NoBrowser = true
		}
	}
}

func cmdHook(event string) {
	cfg := config.Load()

	// Read JSON from stdin
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		// Always exit 0 — never block Claude Code
		os.Exit(0)
	}

	var payload struct {
		SessionID      string `json:"session_id"`
		TranscriptPath string `json:"transcript_path"`
		CWD            string `json:"cwd"`
		HookEventName  string `json:"hook_event_name"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		os.Exit(0)
	}

	if payload.TranscriptPath == "" {
		os.Exit(0)
	}

	// Open database
	database, err := db.Open(cfg.DBPath())
	if err != nil {
		os.Exit(0)
	}
	defer database.Close()

	// Sync this session
	_ = cwsync.SyncFromTranscript(cfg, database, payload.TranscriptPath)

	// Always exit 0
	os.Exit(0)
}

func cmdRebuild() {
	cfg := config.Load()
	setup.LoadSaved(cfg)

	// Open existing DB (keep all messages — including pre-compaction ones
	// no longer in JSONL). Do NOT delete the DB.
	database, err := db.Open(cfg.DBPath())
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: open database: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	// Re-sync from JSONL to pick up any new messages
	fmt.Println("Syncing from JSONL files...")
	if err := rebuildFromJSONL(cfg, database); err != nil {
		fmt.Fprintf(os.Stderr, "error: sync: %v\n", err)
		os.Exit(1)
	}

	// Rebuild FTS from the complete messages table — this preserves
	// pre-compaction messages that are no longer in JSONL files.
	fmt.Println("Rebuilding FTS index from messages table...")
	if err := store.RebuildFTS(database); err != nil {
		fmt.Fprintf(os.Stderr, "warning: rebuild FTS: %v\n", err)
	}

	var count int
	database.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&count)
	var ftsCount int
	database.QueryRow("SELECT COUNT(*) FROM messages_fts").Scan(&ftsCount)
	fmt.Printf("Done: %d sessions, %d messages indexed in FTS\n", count, ftsCount)
}

func rebuildFromJSONL(cfg *config.Config, database *sql.DB) error {
	empty := make(map[string]float64)
	changed, err := claude.ScanAll(cfg, empty)
	if err != nil {
		return err
	}

	for _, path := range changed {
		if err := cwsync.SyncFromTranscript(cfg, database, path); err != nil {
			fmt.Fprintf(os.Stderr, "warning: %s: %v\n", path, err)
		}
	}
	return nil
}

// scanJSONLFiles returns all JSONL paths, optionally filtered by repo path.
func scanJSONLFiles(cfg *config.Config, repoPath string) ([]string, error) {
	projectsDir := cfg.ClaudeProjectsDir()
	var paths []string

	// If repo path given, resolve it and target the specific project dir
	if repoPath != "" {
		abs, err := filepath.Abs(repoPath)
		if err != nil {
			return nil, err
		}
		// Claude encodes /Users/rishi/work/src/foo as -Users-rishi-work-src-foo
		encoded := strings.ReplaceAll(abs, string(filepath.Separator), "-")
		targetDir := filepath.Join(projectsDir, encoded)
		entries, err := os.ReadDir(targetDir)
		if err != nil {
			return nil, fmt.Errorf("no sessions found for repo %s (looked in %s)", repoPath, targetDir)
		}
		for _, e := range entries {
			if !e.IsDir() && filepath.Ext(e.Name()) == ".jsonl" {
				paths = append(paths, filepath.Join(targetDir, e.Name()))
			}
		}
		return paths, nil
	}

	// No filter — scan all
	empty := make(map[string]float64)
	return claude.ScanAll(cfg, empty)
}

func cmdList() {
	cfg := config.Load()

	var repoPath string
	var jsonOutput bool
	for i := 2; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--repo":
			if i+1 < len(os.Args) {
				i++
				repoPath = os.Args[i]
			}
		case "--json":
			jsonOutput = true
		}
	}

	paths, err := scanJSONLFiles(cfg, repoPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if len(paths) == 0 {
		fmt.Fprintln(os.Stderr, "No sessions found.")
		os.Exit(0)
	}

	// Parse each and collect sessions, deduplicate by session ID (keep latest)
	type listEntry struct {
		sessionID    string
		project      string
		startedAt    time.Time
		lastActiveAt time.Time
		messages     int
		toolCalls    int
		oneLiner     string
	}

	seen := make(map[string]int) // sessionID -> index in entries
	var entries []listEntry
	for _, p := range paths {
		session, err := claude.ParseJSONL(p)
		if err != nil || session.SessionID == "" {
			continue
		}

		// Extract first user message as one-liner, count messages and tool calls
		oneLiner := ""
		msgCount := 0
		toolCount := 0
		for _, msg := range session.Messages {
			if msg.Role == "user" || msg.Role == "assistant" {
				msgCount++
			}
			if oneLiner == "" && msg.Role == "user" && !msg.IsCompactSummary {
				oneLiner = msg.ContentText
			}
			// Count tool_use blocks in assistant messages
			if msg.Role == "assistant" && msg.ContentJSON != "" {
				var blocks []claude.ContentBlock
				if err := json.Unmarshal([]byte(msg.ContentJSON), &blocks); err == nil {
					for _, blk := range blocks {
						if blk.Type == "tool_use" {
							toolCount++
						}
					}
				}
			}
		}
		// Truncate and clean up
		oneLiner = strings.ReplaceAll(oneLiner, "\n", " ")
		oneLiner = strings.TrimSpace(oneLiner)
		if len(oneLiner) > 100 {
			oneLiner = oneLiner[:100] + "..."
		}

		e := listEntry{
			sessionID:    session.SessionID,
			project:      session.ProjectName,
			startedAt:    session.StartedAt,
			lastActiveAt: session.LastActiveAt,
			messages:     msgCount,
			toolCalls:    toolCount,
			oneLiner:     oneLiner,
		}

		if idx, ok := seen[session.SessionID]; ok {
			// Keep the one with more recent activity
			if session.LastActiveAt.After(entries[idx].lastActiveAt) {
				entries[idx] = e
			}
		} else {
			seen[session.SessionID] = len(entries)
			entries = append(entries, e)
		}
	}

	// Sort by last active, newest first
	for i := 0; i < len(entries); i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[j].lastActiveAt.After(entries[i].lastActiveAt) {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}

	if jsonOutput {
		type jsonEntry struct {
			SessionID    string `json:"session_id"`
			Project      string `json:"project"`
			StartedAt    string `json:"started_at"`
			LastActiveAt string `json:"last_active_at"`
			Messages     int    `json:"messages"`
			ToolCalls    int    `json:"tool_calls"`
			Summary      string `json:"summary"`
		}
		var out []jsonEntry
		for _, e := range entries {
			out = append(out, jsonEntry{
				SessionID:    e.sessionID,
				Project:      e.project,
				StartedAt:    e.startedAt.UTC().Format(time.RFC3339),
				LastActiveAt: e.lastActiveAt.UTC().Format(time.RFC3339),
				Messages:     e.messages,
				ToolCalls:    e.toolCalls,
				Summary:      e.oneLiner,
			})
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(out)
	} else {
		for _, e := range entries {
			started := e.startedAt.Local().Format("2006-01-02 15:04")
			lastActive := e.lastActiveAt.Local().Format("2006-01-02 15:04")
			fmt.Printf("%-38s  %-20s  %s → %s  [m:%d t:%d]  %s\n", e.sessionID, e.project, started, lastActive, e.messages, e.toolCalls, e.oneLiner)
		}
	}
}

func cmdExport() {
	cfg := config.Load()

	var sessionID, repoPath, outPath string
	var includeToolMsg bool
	for i := 2; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--session-id":
			if i+1 < len(os.Args) {
				i++
				sessionID = os.Args[i]
			}
		case "--repo":
			if i+1 < len(os.Args) {
				i++
				repoPath = os.Args[i]
			}
		case "-o", "--output":
			if i+1 < len(os.Args) {
				i++
				outPath = os.Args[i]
			}
		case "--include-tool-msg":
			includeToolMsg = true
		}
	}

	if sessionID == "" {
		fmt.Fprintln(os.Stderr, "usage: claude-watch export --session-id <id> [--repo <path>] [-o <output.md>] [--include-tool-msg]")
		os.Exit(1)
	}

	paths, err := scanJSONLFiles(cfg, repoPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// Find the JSONL file matching the session ID
	var targetPath string
	for _, p := range paths {
		base := strings.TrimSuffix(filepath.Base(p), ".jsonl")
		if base == sessionID {
			targetPath = p
			break
		}
	}

	if targetPath == "" {
		// Fallback: parse each file looking for matching session ID
		for _, p := range paths {
			session, err := claude.ParseJSONL(p)
			if err != nil {
				continue
			}
			if session.SessionID == sessionID {
				targetPath = p
				break
			}
		}
	}

	if targetPath == "" {
		fmt.Fprintf(os.Stderr, "error: session %s not found\n", sessionID)
		os.Exit(1)
	}

	session, err := claude.ParseJSONL(targetPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: parse %s: %v\n", targetPath, err)
		os.Exit(1)
	}

	md := renderDetailedMarkdown(session, includeToolMsg)

	if outPath != "" {
		if err := os.WriteFile(outPath, []byte(md), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "error: write %s: %v\n", outPath, err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Exported to %s\n", outPath)
	} else {
		fmt.Print(md)
	}
}

func renderDetailedMarkdown(session *claude.Session, includeToolMsg bool) string {
	var b strings.Builder

	// Frontmatter
	b.WriteString("---\n")
	fmt.Fprintf(&b, "session_id: %s\n", session.SessionID)
	fmt.Fprintf(&b, "project: %s\n", session.ProjectName)
	fmt.Fprintf(&b, "project_path: %s\n", session.ProjectPath)
	if session.Slug != "" {
		fmt.Fprintf(&b, "slug: %s\n", session.Slug)
	}
	if session.GitBranch != "" {
		fmt.Fprintf(&b, "git_branch: %s\n", session.GitBranch)
	}
	fmt.Fprintf(&b, "started_at: %s\n", session.StartedAt.UTC().Format("2006-01-02T15:04:05Z"))
	fmt.Fprintf(&b, "last_active_at: %s\n", session.LastActiveAt.UTC().Format("2006-01-02T15:04:05Z"))
	if session.Model != "" {
		fmt.Fprintf(&b, "model: %s\n", session.Model)
	}
	fmt.Fprintf(&b, "has_compaction: %v\n", session.HasCompaction)
	b.WriteString("---\n\n")

	for _, msg := range session.Messages {
		ts := msg.Timestamp.UTC().Format("2006-01-02 15:04:05")

		switch msg.MsgType {
		case "user":
			// Without --include-tool-msg, skip user turns that are tool-related
			if !includeToolMsg {
				if isToolResultOnly(msg.ContentJSON, msg.ContentText) {
					continue
				}
				text := strings.TrimSpace(msg.ContentText)
				if text == "Tool loaded." || text == "" {
					continue
				}
			}
			fmt.Fprintf(&b, "## User · %s\n\n", ts)
			if includeToolMsg && msg.ContentJSON != "" {
				writeDetailedUserBlocks(&b, msg.ContentJSON, msg.ContentText)
			} else {
				b.WriteString(msg.ContentText)
				b.WriteString("\n\n")
			}

		case "assistant":
			if !includeToolMsg {
				// Skip assistant turns with no visible text (tool-only turns)
				text := strings.TrimSpace(msg.ContentText)
				if text == "" {
					continue
				}
			}
			fmt.Fprintf(&b, "## Assistant · %s\n\n", ts)
			if includeToolMsg && msg.ContentJSON != "" {
				writeDetailedBlocks(&b, msg.ContentJSON)
			} else {
				b.WriteString(msg.ContentText)
				b.WriteString("\n\n")
			}

		case "compact_summary":
			b.WriteString("<details>\n<summary>Compaction Summary</summary>\n\n")
			b.WriteString(msg.ContentText)
			b.WriteString("\n</details>\n\n")

		case "compact_boundary":
			b.WriteString("---\n")
			fmt.Fprintf(&b, "> COMPACTION · %s", ts)
			if msg.CompactTrigger != "" {
				fmt.Fprintf(&b, " · trigger: %s", msg.CompactTrigger)
			}
			b.WriteString("\n---\n\n")
		}
	}

	return b.String()
}

// isToolResultOnly returns true if the user message contains only tool_result blocks
// (no actual user text). These are the automatic tool response turns, not real user prompts.
func isToolResultOnly(contentJSON string, contentText string) bool {
	if contentJSON == "" {
		return false
	}
	var blocks []claude.ContentBlock
	if err := json.Unmarshal([]byte(contentJSON), &blocks); err != nil {
		return false
	}
	for _, block := range blocks {
		if block.Type == "text" && strings.TrimSpace(block.Text) != "" {
			return false
		}
	}
	// If we got here, all blocks are tool_result/tool_reference — no real user text
	hasToolResult := false
	for _, block := range blocks {
		if block.Type == "tool_result" || block.Type == "tool_reference" {
			hasToolResult = true
			break
		}
	}
	return hasToolResult
}

func writeDetailedUserBlocks(b *strings.Builder, contentJSON string, fallbackText string) {
	var blocks []claude.ContentBlock
	if err := json.Unmarshal([]byte(contentJSON), &blocks); err != nil {
		b.WriteString(fallbackText)
		b.WriteString("\n\n")
		return
	}

	hasToolResult := false
	for _, block := range blocks {
		if block.Type == "tool_result" {
			hasToolResult = true
			break
		}
	}
	if !hasToolResult {
		b.WriteString(fallbackText)
		b.WriteString("\n\n")
		return
	}

	for _, block := range blocks {
		switch block.Type {
		case "text":
			if block.Text != "" {
				b.WriteString(block.Text)
				b.WriteString("\n\n")
			}
		case "tool_result":
			toolName := extractToolName(block)
			if toolName != "" {
				fmt.Fprintf(b, "### Tool Result: `%s`\n\n", toolName)
			} else {
				b.WriteString("### Tool Result\n\n")
			}
			if block.ToolUseID != "" {
				fmt.Fprintf(b, "**Tool Use ID:** `%s`\n\n", block.ToolUseID)
			}
			if len(block.Content) > 0 && string(block.Content) != "null" {
				writeToolResultContent(b, block.Content)
			}
		case "tool_reference":
			// Skip tool reference markers
		}
	}
}

func extractToolName(block claude.ContentBlock) string {
	if block.Name != "" {
		return block.Name
	}
	if block.ToolName != "" {
		return block.ToolName
	}
	// Look inside content array for tool_reference blocks
	if len(block.Content) > 0 {
		var refs []struct {
			Type     string `json:"type"`
			ToolName string `json:"tool_name"`
		}
		if err := json.Unmarshal(block.Content, &refs); err == nil {
			for _, r := range refs {
				if r.Type == "tool_reference" && r.ToolName != "" {
					return r.ToolName
				}
			}
		}
	}
	return ""
}

func writeToolResultContent(b *strings.Builder, content json.RawMessage) {
	// Try as plain string
	var text string
	if err := json.Unmarshal(content, &text); err == nil {
		b.WriteString("```\n")
		b.WriteString(text)
		b.WriteString("\n```\n\n")
		return
	}
	// Try as array of content blocks
	var resultBlocks []struct {
		Type     string `json:"type"`
		Text     string `json:"text"`
		ToolName string `json:"tool_name"`
	}
	if err := json.Unmarshal(content, &resultBlocks); err == nil {
		for _, rb := range resultBlocks {
			switch rb.Type {
			case "text":
				if rb.Text != "" {
					b.WriteString("```\n")
					b.WriteString(rb.Text)
					b.WriteString("\n```\n\n")
				}
			case "tool_reference":
				// Skip
			default:
				if rb.Text != "" {
					b.WriteString("```\n")
					b.WriteString(rb.Text)
					b.WriteString("\n```\n\n")
				}
			}
		}
		return
	}
	// Fallback: raw JSON
	b.WriteString("```json\n")
	b.Write(content)
	b.WriteString("\n```\n\n")
}

func writeDetailedBlocks(b *strings.Builder, contentJSON string) {
	var blocks []claude.ContentBlock
	if err := json.Unmarshal([]byte(contentJSON), &blocks); err != nil {
		b.WriteString(contentJSON)
		b.WriteString("\n\n")
		return
	}

	for _, block := range blocks {
		switch block.Type {
		case "text":
			if block.Text != "" {
				b.WriteString(block.Text)
				b.WriteString("\n\n")
			}
		case "tool_use":
			fmt.Fprintf(b, "### Tool Call: `%s`\n\n", block.Name)
			if block.ID != "" {
				fmt.Fprintf(b, "**ID:** `%s`\n\n", block.ID)
			}
			if len(block.Input) > 0 && string(block.Input) != "null" {
				b.WriteString("**Input:**\n```json\n")
				// Pretty-print the JSON
				var pretty json.RawMessage
				if err := json.Unmarshal(block.Input, &pretty); err == nil {
					formatted, err := json.MarshalIndent(pretty, "", "  ")
					if err == nil {
						b.Write(formatted)
					} else {
						b.Write(block.Input)
					}
				} else {
					b.Write(block.Input)
				}
				b.WriteString("\n```\n\n")
			}
		case "tool_result":
			b.WriteString("### Tool Result\n\n")
			if block.ID != "" {
				fmt.Fprintf(b, "**Tool Use ID:** `%s`\n\n", block.ID)
			}
			if len(block.Content) > 0 && string(block.Content) != "null" {
				// Content can be a string or array of blocks
				var text string
				if err := json.Unmarshal(block.Content, &text); err == nil {
					b.WriteString("```\n")
					b.WriteString(text)
					b.WriteString("\n```\n\n")
				} else {
					// Try as array of content blocks
					var resultBlocks []struct {
						Type string `json:"type"`
						Text string `json:"text"`
					}
					if err := json.Unmarshal(block.Content, &resultBlocks); err == nil {
						for _, rb := range resultBlocks {
							if rb.Text != "" {
								b.WriteString("```\n")
								b.WriteString(rb.Text)
								b.WriteString("\n```\n\n")
							}
						}
					} else {
						b.WriteString("```json\n")
						b.Write(block.Content)
						b.WriteString("\n```\n\n")
					}
				}
			}
		}
	}
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	default:
		return
	}
	cmd.Run()
}
