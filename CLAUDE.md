# claude-watch

Read-only desktop app to browse, search, and preserve Claude Code conversation history.
Claude Code natively compacts context (summarizing old messages), which destroys conversation
history. This app solves that by syncing Claude's JSONL conversation files into Markdown files
(primary storage) + SQLite index — preserving full history even after compaction.

## Quick start
```bash
CGO_ENABLED=0 go build -o claude-watch .
./claude-watch serve          # start server + open browser at localhost:7823
./claude-watch hook stop      # called by Claude hooks (reads JSON from stdin)
./claude-watch rebuild        # rebuild SQLite index from .md files
```

## Key invariants
- **Read-only UI** — the app never modifies Claude's data
- **No CGO** — build with CGO_ENABLED=0, uses modernc.org/sqlite (pure Go)
- **Port 7823** — hardcoded default, overridable with --port or CLAUDE_WATCH_PORT
- **MD files are source of truth** — SQLite is a rebuildable search index only
- **Hooks use CLI, not HTTP** — hook scripts call `claude-watch hook <event>` directly

## Data directories
- Source code: `~/work/src/claude-watch/`
- App data: `~/work/claude-watch/`
  - `sessions/{project}/{session-id}.md` — conversation history (source of truth)
  - `claude-watch.db` — SQLite search index (rebuildable)
  - `hooks/*.sh` — installed hook scripts

## Claude's JSONL source
`~/.claude/projects/{encoded-path}/*.jsonl`
Each hook payload includes `transcript_path` — the exact JSONL path to sync.

## Architecture
1. Claude Code fires a hook event (SessionStart, UserPromptSubmit, Stop, PreCompact, SessionEnd)
2. Hook script calls `claude-watch hook <event>` with JSON payload on stdin
3. Binary reads `transcript_path` from payload, syncs JSONL → MD → SQLite
4. Browser UI at localhost:7823 reads from SQLite (list/search) and MD files (conversation content)

## Package structure
```
internal/
  config/       # paths, port, env vars
  claude/       # JSONL types, parser (bufio.Scanner 10MB buf), scanner
  markdown/     # write/append .md session files
  db/           # SQLite open + migrations
  store/        # sessions, messages, FTS5 search queries
  api/          # HTTP server + endpoints
  hooks/        # install hook scripts + merge settings.json
  sync/         # SyncFromTranscript, SyncAll, RebuildIndex
```

## Running
```bash
go run . serve --no-browser    # dev, no browser auto-open
go run . rebuild               # rebuild SQLite from .md files
CGO_ENABLED=0 go build .       # production binary
```

## Testing
No automated tests yet. Manual verification:
1. `go build .` must succeed with CGO_ENABLED=0
2. `./claude-watch serve` — browser opens, sessions visible
3. Run `echo '{"session_id":"test","transcript_path":"/path/to/file.jsonl","cwd":"/tmp","hook_event_name":"Stop"}' | ./claude-watch hook stop`
4. Search with comma (AND) and semicolon (OR) operators
