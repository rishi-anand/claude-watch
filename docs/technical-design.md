# claude-watch: Technical Design

## Problem statement

Claude Code's context compaction (`/compact`) is a token-management strategy that summarizes
old conversation turns and discards the originals. This is destructive: once compacted, the
detailed history is gone from Claude's JSONL files and cannot be recovered.

claude-watch solves this by hooking into Claude Code's event system to capture each session
transcript **before** compaction occurs, persisting everything to local Markdown files that
survive the compaction.

---

## Architecture overview

```
┌──────────────────────────────────────────────────────────────┐
│                     Claude Code (claude CLI)                  │
│                                                              │
│  fires hooks → SessionStart, UserPromptSubmit, Stop,        │
│                PreCompact, SessionEnd                        │
└──────────────────┬───────────────────────────────────────────┘
                   │ JSON payload on stdin (includes transcript_path)
                   ▼
┌──────────────────────────────────────────────────────────────┐
│              Hook scripts (~/.claude-watch/hooks/*.sh)        │
│         cat | claude-watch hook <event>                      │
└──────────────────┬───────────────────────────────────────────┘
                   │
                   ▼
┌──────────────────────────────────────────────────────────────┐
│                   claude-watch CLI binary                     │
│                                                              │
│  cmd: hook <event>  →  read transcript_path from JSON stdin  │
│                     →  parse JSONL  →  write/update .md file │
│                     →  rebuild SQLite index for session       │
│                                                              │
│  cmd: serve         →  startup full scan of all JSONL files  │
│                     →  HTTP server on :7823                  │
│                     →  browser UI                            │
│                                                              │
│  cmd: rebuild       →  reindex all .md files → SQLite        │
└──────────────┬───────────────────────────────────────────────┘
               │
       ┌───────┴────────┐
       ▼                ▼
  .md files          SQLite DB
  (source of truth)  (search index)
```

---

## Hook system

### What are Claude Code hooks?

Claude Code supports lifecycle hooks — shell commands invoked at specific points in a
conversation. Each hook receives a JSON payload on stdin with context about the current event.

| Hook event | When it fires |
|------------|---------------|
| `SessionStart` | When a new conversation is created |
| `UserPromptSubmit` | When the user submits a prompt |
| `Stop` | When Claude finishes a response turn |
| `PreCompact` | **Before** context compaction — critical for history preservation |
| `SessionEnd` | When the session ends |

### Hook script pattern

Each hook script at `~/.claude-watch/hooks/<event>.sh`:

```bash
#!/usr/bin/env bash
PAYLOAD=$(cat)
/path/to/claude-watch hook stop <<< "$PAYLOAD"
```

The script:
1. Reads the JSON payload from stdin (`cat`)
2. Passes it directly to the `claude-watch hook <event>` CLI subcommand

**Why CLI invocation instead of HTTP?**
The HTTP server (`claude-watch serve`) does not need to be running for hooks to capture data.
Hook scripts write directly to disk (Markdown files + SQLite) via the CLI binary. This means:
- Capturing works even when the browser UI is closed
- No dependency on a running server process
- No network errors if the server is down

### Hook payload structure

Every hook payload includes `transcript_path` — the exact path to the JSONL file for the
current session:

```json
{
  "session_id": "a1b2c3d4-...",
  "transcript_path": "/Users/you/.claude/projects/-Users-you-work-src-myapp/abc123.jsonl",
  "hook_event_name": "Stop",
  "cwd": "/Users/you/work/src/myapp"
}
```

This eliminates filesystem scanning — no need to search for the right JSONL file.

### Automatic hook installation

When `claude-watch serve` runs for the first time, it:

1. Writes the 5 hook scripts to `~/claude-watch/hooks/`
2. Reads `~/.claude/settings.json`
3. Merges the hook entries (does not overwrite existing hooks)
4. Writes back atomically (temp file + rename)

The merge is additive — existing hooks in `settings.json` are preserved.

---

## Data pipeline

### JSONL → Markdown → SQLite

```
~/.claude/projects/{encoded-path}/{session}.jsonl
        │
        │  ParseJSONL (bufio.Scanner, 10MB buffer)
        ▼
  []ParsedMessage  (UUID, role, type, text, content blocks, timestamp)
        │
        │  markdown.WriteSession / AppendMessages
        ▼
  ~/claude-watch/sessions/{project}/{session-id}.md
        │
        │  store.UpsertMessages + FTS5 delete+reinsert
        ▼
  ~/claude-watch/claude-watch.db  (sessions + messages + messages_fts)
```

### Markdown file format (source of truth)

```markdown
---
session_id: a1b2c3d4-...
project: my-project
project_path: /Users/you/work/src/my-project
slug: helpful-session-name
git_branch: main
started_at: 2026-01-15T10:00:00Z
last_active_at: 2026-01-15T11:30:00Z
has_compaction: true
---

## User · 2026-01-15 10:00:12

Can you implement the SSH tunnel feature?

## Assistant · 2026-01-15 10:02:44

I'll implement the SSH tunnel by...

---
> COMPACTION · 2026-01-15 10:05:00
---

<details>
<summary>Compaction Summary</summary>

[Summary text preserved here — this is what Claude compacted away]

</details>

## User · 2026-01-15 10:06:00

Continue from where we left off...
```

Markdown files are **append-only** — new messages are appended, existing content is never
rewritten. This makes them safe to open in any editor and resilient to partial writes.

### SQLite schema

```sql
-- Session metadata (listing + filtering)
CREATE TABLE sessions (
    session_id      TEXT PRIMARY KEY,
    project_path    TEXT NOT NULL,
    project_name    TEXT NOT NULL,
    slug            TEXT,
    git_branch      TEXT,
    first_message   TEXT,
    last_message    TEXT,
    started_at      DATETIME NOT NULL,
    last_active_at  DATETIME NOT NULL,
    message_count   INTEGER DEFAULT 0,
    has_compaction  INTEGER DEFAULT 0,
    md_path         TEXT NOT NULL,   -- path to .md file (source of truth)
    md_mtime        REAL NOT NULL,   -- mtime at last index (skip if unchanged)
    memory_md       TEXT
);

-- Per-message content (full fidelity, including tool calls as JSON)
CREATE TABLE messages (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id   TEXT NOT NULL,
    msg_uuid     TEXT NOT NULL,
    msg_type     TEXT NOT NULL,   -- user | assistant | compact_boundary | compact_summary
    role         TEXT,
    content_text TEXT,            -- plain text (for FTS)
    content_json TEXT,            -- full content blocks JSON (for UI rendering)
    timestamp    DATETIME NOT NULL,
    seq          INTEGER NOT NULL,
    UNIQUE(session_id, msg_uuid)
);

-- FTS5 full-text search (standalone, not content-linked)
CREATE VIRTUAL TABLE messages_fts USING fts5(
    session_id UNINDEXED,
    msg_uuid UNINDEXED,
    content_text
);

-- Track .md file mtimes to skip re-indexing unchanged files
CREATE TABLE index_state (
    md_path     TEXT PRIMARY KEY,
    last_mtime  REAL NOT NULL
);
```

**FTS5 strategy:** Standalone FTS table (not `content=messages`). On each session sync, all
FTS rows for that session are deleted and reinserted. This avoids the rowid-sync complexity of
content-table FTS and is correct for batch updates.

**SQLite pragmas:** `WAL` journal mode, `NORMAL` synchronous — gives good write throughput
with crash safety.

---

## Search query parsing

The search bar supports a compact query language translated to FTS5 syntax:

| Input | Operator | FTS5 |
|-------|----------|------|
| `ssh tunnel` | exact phrase | `"ssh tunnel"` |
| `ssh,tunnel` | AND | `ssh AND tunnel` |
| `ssh;tunnel` | OR | `ssh OR tunnel` |
| `ssh tunnel,foo` | phrase AND term | `"ssh tunnel" AND foo` |

The `snippet()` FTS5 function returns highlighted excerpts with `<mark>` tags for display
in search results.

---

## Content rendering

### Why content_json?

Many assistant messages consist entirely of tool calls with no plain text. Storing only
`content_text` (which skips tool_use blocks) would make these messages appear empty in the UI.

`content_json` stores the full `[]ContentBlock` array as JSON, including:
- `type: "text"` — prose text
- `type: "tool_use"` — tool invocation with name + input parameters
- `type: "tool_result"` — tool output

The frontend renders each block type distinctly:
- Text blocks → rendered as Markdown (headers, lists, bold, code, etc.)
- Tool use → collapsible `<details>` showing tool name + input JSON
- Tool result → collapsible `<details>` showing output (truncated at 4000 chars)

### Markdown rendering

All message text (user and assistant) is passed through a client-side Markdown renderer that
handles:
- Fenced code blocks with language class
- ATX headings (`#` through `######`)
- Ordered and unordered lists (with nesting)
- Blockquotes
- Horizontal rules
- Inline: bold, bold+italic, italic, strikethrough, inline code, links (rendered as non-clickable spans — read-only app)

---

## JSONL parsing details

Claude Code stores conversation history as JSONL files at:
```
~/.claude/projects/{encoded-project-path}/{session-id}.jsonl
```

The encoded project path uses `-` for `/` (lossy). The authoritative project path is
extracted from the `cwd` field in the first entry of the JSONL file.

**Entry types:**

| type | subtype | Action |
|------|---------|--------|
| `user` | — | Store as `msg_type=user` |
| `user` | `isCompactSummary:true` | Store as `msg_type=compact_summary` |
| `assistant` | — | Store as `msg_type=assistant` |
| `system` | `compact_boundary` | Store as `msg_type=compact_boundary` |
| `system` | `turn_duration` | Skip |
| `progress` | — | Skip |
| `queue-operation` | — | Skip |
| `file-history-snapshot` | — | Skip |

**Large file handling:** `bufio.Scanner` with a 10 MB token buffer. Observed JSONL files up
to 20+ MB in production.

**Multi-file sessions:** Claude Code creates a new JSONL file when resuming a session
(via `--resume`). Multiple JSONL files share the same `session_id`. All are merged and
ordered by `timestamp, seq` (not seq alone, since each file restarts seq at 1).

---

## Startup sequence

```
claude-watch serve
  │
  ├── 1. Ensure ~/claude-watch/{sessions,hooks}/ exist
  ├── 2. Install hook scripts to ~/claude-watch/hooks/
  ├── 3. Merge hooks into ~/.claude/settings.json (atomic rename)
  ├── 4. Open SQLite, run schema migrations (ALTER TABLE if columns missing)
  ├── 5. Full scan: ~/.claude/projects/**/*.jsonl → .md files → SQLite
  │       (skips JSONL files whose mtime hasn't changed since last index)
  ├── 6. Start HTTP server on :7823
  └── 7. Open browser after 500ms
```

After startup, new sessions are captured entirely via hooks — no polling, no background timer.

---

## API endpoints

```
GET  /api/conversations?page=1&limit=50&project=my-project
GET  /api/conversations/:sessionId
GET  /api/search?q=<query>&page=1&limit=50
GET  /api/status
POST /api/hooks/session-start
POST /api/hooks/prompt
POST /api/hooks/stop
POST /api/hooks/compact
POST /api/hooks/session-end
```

All responses are JSON. The HTTP server uses only stdlib `net/http` — no external router.

---

## Design decisions

| Decision | Rationale |
|----------|-----------|
| Markdown as source of truth | Human-readable, openable in any editor, no proprietary format |
| SQLite via modernc.org/sqlite | Pure Go, CGO_ENABLED=0, single binary |
| CLI hooks (not HTTP) | Hook capture works even when `serve` is not running |
| Standalone FTS5 (not content=) | Avoids rowid-sync complexity, correct for batch session updates |
| No polling / background timer | All captures are event-driven via hooks; `serve` does one startup scan |
| Append-only .md files | Safe from partial write corruption; efficient for large sessions |
| Read-only UI | Never modifies Claude's data; prevents accidental corruption |
