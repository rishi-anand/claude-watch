<p align="center">
  <img src="static/logo.png" alt="claude-watch logo" width="200"/>
</p>

<h1 align="center">claude-watch</h1>

<p align="center">Browse, search, and <b>permanently preserve</b> your Claude Code conversation history.</p>

Claude Code natively compacts context — summarizing and discarding old messages to free up token space. Once compacted, that history is gone. **claude-watch** captures every conversation in real time via [Claude Code hooks](https://code.claude.com/docs/en/hooks) and stores it in plain Markdown files and a searchable SQLite index, so nothing is ever lost.

---

## Quick start

**1. Install**

```bash
curl -fsSL https://github.com/rishi-anand/claude-watch/releases/latest/download/install.sh | bash
```

Detects your OS and architecture (macOS/Linux, amd64/arm64) and installs to `~/.local/bin/claude-watch`.

**2. Start**

```bash
claude-watch serve
```

On first run, you'll be asked:
- Where to store session files (default: `~/claude-watch/`)
- Permission to update `~/.claude/settings.json` with Claude Code hooks

After confirming, the browser opens at `http://localhost:7823` and all your existing sessions are indexed. Every new Claude Code session is captured automatically from that point on — no need to keep `serve` running.

---

## Why

- Claude Code's `/compact` destroys conversation history by design
- claude-watch hooks in **before** compaction, capturing the full transcript
- All history is stored in human-readable `.md` files — no proprietary format, no lock-in
- A local web UI lets you browse, search, and copy session IDs to resume conversations

---

## Features

- **Full history preservation** — captures every session in real time, survives compaction
- **Fast full-text search** — SQLite FTS5, all words must match (implicit AND)
- **Rich conversation view** — renders markdown, tool calls, tool results, compaction markers
- **Project filter** — browse sessions by project
- **CLI list & export** — browse and export sessions from the command line, no SQLite needed
- **Dark/light theme** — toggle in the header, preference saved across sessions
- **Zero dependencies** — single static binary, no CGO, no Docker, no database server
- **Transparent setup** — shows exactly what will be written before touching any config

---

## Commands

### `serve` — Web UI

Start the HTTP server with a browser-based UI for browsing, reading, and searching conversations.

```bash
# Start server and open browser (default port 7823)
claude-watch serve

# Custom port
claude-watch serve --port 8080

# Start without opening browser
claude-watch serve --no-browser
```

### `list` — List sessions (no SQLite required)

Browse sessions directly from Claude's JSONL files. Shows session ID, project, start/last-active dates, message count (`m:`), tool call count (`t:`), and first user prompt.

```bash
# List all sessions across all repos
claude-watch list

# List sessions for a specific repo
claude-watch list --repo /path/to/your/project

# JSON output (for scripting)
claude-watch list --repo /path/to/your/project --json
```

**Example output:**
```
acae171d-f831-49c2-88cb-038285a3e627    my-project   2026-03-28 20:58 → 2026-03-28 21:21  [m:266 t:99]  Add list and export subcommands...
b1c29a16-04b0-46c7-87d6-3f9bed8e996a    my-project   2026-03-06 09:37 → 2026-03-08 11:30  [m:297 t:109]  My goal is to create a UI where...
```

**JSON output (`--json`):**
```json
[
  {
    "session_id": "acae171d-f831-49c2-88cb-038285a3e627",
    "project": "my-project",
    "started_at": "2026-03-29T03:58:13Z",
    "last_active_at": "2026-03-29T04:21:27Z",
    "messages": 267,
    "tool_calls": 99,
    "summary": "Add list and export subcommands..."
  }
]
```

### `export` — Export session to Markdown (no SQLite required)

Export any conversation to a clean Markdown file. By default, only user prompts and assistant responses are included. Add `--include-tool-msg` for full detail with every tool call, input, and result.

```bash
# Export to stdout (clean — user/assistant text only)
claude-watch export --session-id <session-id>

# Export for a specific repo
claude-watch export --session-id <session-id> --repo /path/to/your/project

# Save to file
claude-watch export --session-id <session-id> -o conversation.md

# Include full tool call details (inputs, results, IDs)
claude-watch export --session-id <session-id> -o full-detail.md --include-tool-msg
```

**Default output** (clean conversation):
```markdown
---
session_id: b1c29a16-04b0-46c7-87d6-3f9bed8e996a
project: my-project
started_at: 2026-03-06T17:37:50Z
last_active_at: 2026-03-08T18:30:06Z
model: claude-sonnet-4-6
---

## User · 2026-03-06 17:37:50

My goal is to create a UI where I can see all my conversations...

## Assistant · 2026-03-06 17:38:02

Let me start by exploring the relevant areas in parallel...
```

**With `--include-tool-msg`** (full detail including tool calls and results):
```markdown
## Assistant · 2026-03-06 17:38:11

### Tool Call: `Agent`

**ID:** `toolu_01Xkz87zZm5nhEtY4Rej4NYx`

**Input:**
```json
{
  "description": "Explore Claude data folder",
  "subagent_type": "Explore",
  "prompt": "Explore the ~/.claude directory..."
}
```

## User · 2026-03-06 17:38:54

### Tool Result: `Agent`

**Tool Use ID:** `toolu_01Xkz87zZm5nhEtY4Rej4NYx`

```
Summary of findings...
```
```

### `hook` — Real-time sync via Claude Code hooks

Processes hook events from Claude Code to sync conversations in real time. Hook scripts are installed automatically on first `serve`.

```bash
# Called automatically by Claude Code hooks (reads JSON from stdin)
echo '{"session_id":"abc","transcript_path":"/path/to/file.jsonl","cwd":"/tmp","hook_event_name":"Stop"}' \
  | claude-watch hook stop
```

Supported events: `SessionStart`, `UserPromptSubmit`, `Stop`, `PreCompact`, `SessionEnd`

The `PreCompact` hook is the most critical — it fires **before** Claude compacts the context, ensuring full history is preserved.

### `rebuild` — Rebuild search index

Force rebuild the SQLite FTS5 search index from all session files.

```bash
claude-watch rebuild
```

---

## Search

Type words to search — all words must match. Hyphens and apostrophes are treated as word separators.

| Query | Matches |
|-------|---------|
| `ssh tunnel` | messages containing both "ssh" and "tunnel" |
| `palette-agentic-cli` | messages containing "palette", "agentic", and "cli" |
| `Cloud's` | messages containing "Cloud" |

---

## How it works

claude-watch uses [Claude Code hooks](https://code.claude.com/docs/en/hooks) — shell scripts that Claude Code invokes at key points in a conversation. Each hook calls the `claude-watch` CLI directly (no server required):

```bash
cat | claude-watch hook stop
```

This means sessions are captured even when `claude-watch serve` isn't running. The `list` and `export` commands read Claude's JSONL files directly — no SQLite, no server, no setup needed.

See [docs/technical-design.md](docs/technical-design.md) for full architecture details.

---

## License

MIT
