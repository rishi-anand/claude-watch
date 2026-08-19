<p align="center">
  <img src="static/logo.png" alt="claude-watch logo" width="180"/>
</p>

<h1 align="center">claude-watch</h1>

<p align="center">
  <b>Every Claude Code conversation, kept.</b><br>
  Nothing lost to <code>/compact</code>.
</p>

<p align="center">
  <a href="https://claude-watch.pages.dev/"><b>Website</b></a> ·
  <a href="#quick-start">Install</a> ·
  <a href="#commands">Commands</a> ·
  <a href="#how-it-works">How it works</a>
</p>

---

## What it does

Claude Code compacts context when the window fills — summarizing and discarding old messages to free tokens. Once compacted, that history is **gone**.

**claude-watch** captures every session in real time via [Claude Code hooks](https://code.claude.com/docs/en/hooks), stores it as plain Markdown, and indexes it with SQLite FTS5. Every prompt, every response, every tool call — preserved, searchable, and readable in a local web UI at `localhost:7823`.

- Fires **before** `/compact` runs — captures the full transcript before summarization
- Stores in human-readable `.md` files — no proprietary format, no lock-in
- Fast full-text search across every session you've ever run
- Single Go binary. No CGO, no Docker, no database server

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

First run asks where to store sessions (default `~/claude-watch/`) and for permission to add Claude Code hooks to `~/.claude/settings.json`. After confirming, the browser opens at `http://localhost:7823` and existing sessions are indexed.

Every new Claude Code session is captured automatically from that point on — you don't need to keep `serve` running.

**3. Teach your coding agent to use it (optional but recommended)**

```bash
claude-watch install-skill
```

Installs a skill file for Claude Code, Codex, and Cursor so asking your agent _"what was that session where I debugged the flaky test?"_ routes through `claude-watch` instead of grepping raw JSONL. See [`install-skill`](#install-skill--teach-a-coding-agent-to-use-claude-watch) below.

---

## Commands

Featured commands (`search`, `list`, `export`) come first — they're what you'll reach for day-to-day. Everything else supports them.

### `search` — find any conversation, instantly

Full-text search across every captured conversation using SQLite FTS5. Same index the web UI queries, so CLI and browser stay in sync.

```bash
# Search everything
claude-watch search "database migration"

# Scope to a repo
claude-watch search "auth token" --repo ~/work/api

# Scope to a single session
claude-watch search "compact boundary" --session-id 83d192d6

# Pagination
claude-watch search "foo" --page 2 --limit 20

# JSON output (for scripting)
claude-watch search "kafka" --json | jq '.results[]'
```

**Example output:**

```
CONVERSATION ID                       REPO          MSG       TIMESTAMP            MATCH
------------------------------------  ------------  --------  -------------------  -----
83d192d6-e704-4975-b650-450aa2264072  billing-api   2ba2e957  2026-08-14 15:56:31  …run the database migration before the deploy…
b1c29a16-04b0-46c7-87d6-3f9bed8e996a  billing-api   efc7fc23  2026-08-12 09:21:04  …reverting the migration broke database constraints…
acae171d-f831-49c2-88cb-038285a3e627  launchpad-ai  1826283c  2026-08-09 18:03:57  …I want to add a database migration for the users…

Showing 3 of 12 matches (page 1, limit 50)
```

Matches are highlighted with ANSI colors in the terminal; the `--json` variant strips highlight markers.

**Query rules:**

| Query | Matches |
|-------|---------|
| `ssh tunnel` | messages containing both `ssh` and `tunnel` (implicit AND) |
| `feature-flag-config` | messages containing `feature`, `flag`, and `config` (hyphens split into parts) |
| `Cloud's` | messages containing `Cloud` (apostrophes split too) |

**Flags:**

| Flag | Purpose |
|------|---------|
| `--repo <path>` | Restrict search to sessions from one repo |
| `--session-id <id>` | Restrict search to one session |
| `--page N` · `--limit N` | Paginate (defaults: page 1, limit 50) |
| `--json` | Emit results as JSON (highlights stripped) |

---

### `list` — every session at a glance

Browse sessions directly from Claude's JSONL files. No SQLite index required — this reads `~/.claude/projects/**/*.jsonl` directly.

```bash
# All sessions across all repos
claude-watch list

# Sessions for one repo
claude-watch list --repo ~/work/api

# JSON output
claude-watch list --repo ~/work/api --json
```

**Example output:**

```
acae171d-f831-49c2-88cb-038285a3e627    stripe-webhook-api    2026-06-28 20:58 → 2026-06-28 21:21  [m:266 t:99]   Add list and export subcommands…
83d192d6-e704-4975-b650-450aa2264072    stripe-webhook-api    2026-06-06 09:37 → 2026-06-08 11:30  [m:297 t:109]  Rework database migration flow…
b1c29a16-04b0-46c7-87d6-3f9bed8e996a    stripe-webhook-api    2026-05-14 22:05 → 2026-05-14 22:07  [m:12 t:3]     Debug flaky test in user_test.go…
```

Columns: **session id** · **project** · **first activity → last activity** · **[m: messages, t: tool calls]** · **first user prompt**. Sorted newest first by last-active time.

**JSON output (`--json`):**

```json
[
  {
    "session_id": "acae171d-f831-49c2-88cb-038285a3e627",
    "project": "stripe-webhook-api",
    "started_at": "2026-06-29T03:58:13Z",
    "last_active_at": "2026-06-29T04:21:27Z",
    "messages": 267,
    "tool_calls": 99,
    "summary": "Add list and export subcommands..."
  }
]
```

---

### `export` — dump a session to Markdown

Export any conversation to clean Markdown. By default only user prompts and assistant responses are kept. Add `--include-tool-msg` for full detail — every tool call, input, and result.

```bash
# Dump to stdout
claude-watch export --session-id 83d192d6

# Scope to a repo (helps disambiguation when IDs collide)
claude-watch export --session-id 83d192d6 --repo ~/work/api

# Save to file
claude-watch export --session-id 83d192d6 -o conversation.md

# Include full tool call detail (inputs, results, IDs)
claude-watch export --session-id 83d192d6 -o full.md --include-tool-msg
```

**Default output** (clean, user + assistant only):

~~~markdown
---
session_id: 83d192d6-e704-4975-b650-450aa2264072
project: stripe-webhook-api
project_path: /Users/rishi/work/stripe-webhook-api
git_branch: main
started_at: 2026-06-06T09:37:50Z
last_active_at: 2026-06-08T11:30:06Z
model: claude-sonnet-4-6
has_compaction: true
---

## User · 2026-06-06 09:37:50

Add a database migration that introduces a `deleted_at` column on users…

## Assistant · 2026-06-06 09:38:02

I'll create a new migration under `db/migrations/`…
~~~

**With `--include-tool-msg`** (every tool call, input, and result):

~~~markdown
## Assistant · 2026-06-06 09:38:11

### Tool Call: `Agent`

**ID:** `toolu_01Xkz87zZm5nhEtY4Rej4NYx`

**Input:**
```json
{
  "description": "Explore Claude data folder",
  "subagent_type": "Explore",
  "prompt": "Explore the ~/.claude directory…"
}
```

## User · 2026-06-06 09:38:54

### Tool Result: `Agent`

**Tool Use ID:** `toolu_01Xkz87zZm5nhEtY4Rej4NYx`

```
Summary of findings…
```
~~~

Compaction boundaries and summaries are rendered inline so exported files reflect the true shape of the conversation.

---

### `serve` — local web UI

Start the HTTP server with a browser UI for browsing, reading, and searching conversations.

```bash
# Start server and open browser (default port 7823)
claude-watch serve

# Custom port
claude-watch serve --port 8080

# Start without opening browser
claude-watch serve --no-browser
```

On first run, `serve` walks you through:

1. Choosing a data directory (default `~/claude-watch/`).
2. Installing Claude Code hooks into `~/.claude/settings.json` — the file diff is shown before anything is written, and you can decline.

After confirming, existing JSONL files are indexed and the browser opens at `http://localhost:7823`. New sessions are captured via hooks — you don't need to keep `serve` running.

---

### `install-skill` — teach a coding agent to use claude-watch

Installs a skill file that tells your coding assistant how and when to call `claude-watch` for looking up prior conversations. Once installed, asking your agent _"what was that session where I debugged the flaky test?"_ routes through the CLI instead of grepping raw JSONL.

```bash
# Install for all supported tools (Claude Code, Codex, Cursor)
claude-watch install-skill

# See what would be written without touching anything
claude-watch install-skill --dry-run

# List supported tools and their target paths
claude-watch install-skill --list

# Install for a specific tool (or a comma-separated subset)
claude-watch install-skill --tool claude
claude-watch install-skill --tool claude,cursor

# Rewrite even if content is unchanged
claude-watch install-skill --force
```

**Supported targets:**

| Tool | Destination |
|------|-------------|
| `claude` | `~/.claude/skills/claude-watch/SKILL.md` |
| `codex`  | `~/.codex/AGENTS.md` (merged with begin/end markers — safe to re-run) |
| `cursor` | `~/.cursor/rules/claude-watch.mdc` |

---

### `hook` — real-time sync from Claude Code

Processes hook events from Claude Code and syncs the referenced transcript to Markdown + SQLite. Hook scripts are installed automatically on first `serve`; you rarely invoke this directly.

```bash
# Called automatically by Claude Code hooks (reads JSON from stdin)
echo '{"session_id":"abc","transcript_path":"/path/to/file.jsonl","cwd":"/tmp","hook_event_name":"Stop"}' \
  | claude-watch hook stop
```

**Supported events:** `SessionStart`, `UserPromptSubmit`, `Stop`, `PreCompact`, `SessionEnd`.

The `PreCompact` hook is the most critical — it fires **before** Claude Code compacts the context, giving `claude-watch` a chance to save the full transcript before summarization erases it.

---

### `rebuild` — rebuild the FTS index

Force a full rebuild of the SQLite FTS5 index from your `.md` session files. Use this if the index gets out of sync or after upgrading to a version that changed the schema.

```bash
claude-watch rebuild
```

The Markdown files are the source of truth; the index is disposable and always rebuildable from them.

---

## How it works

`claude-watch` uses [Claude Code hooks](https://code.claude.com/docs/en/hooks) — shell scripts that Claude Code invokes at key points in a conversation. Each hook pipes JSON to the `claude-watch` binary directly (no server, no HTTP round-trip):

```bash
cat | claude-watch hook stop
```

**Event flow:**

| Event | When it fires | What claude-watch does |
|-------|---------------|------------------------|
| `SessionStart` | New session begins | Record session, create Markdown stub |
| `UserPromptSubmit` | User sends a prompt | Incremental sync JSONL → MD → SQLite |
| `Stop` | Assistant finishes a turn | Full sync of the session |
| `PreCompact` | **Before** Claude compacts | Full sync — captures history before it's destroyed |
| `SessionEnd` | Session closes | Final sync |

**Storage layout** (in your data directory, `~/claude-watch/` by default):

```
~/claude-watch/
  sessions/{project}/{session-id}.md   ← source of truth (append-only)
  claude-watch.db                      ← SQLite FTS5 index (rebuildable)
  hooks/                               ← installed hook scripts
```

Because the Markdown files are the source of truth, you can inspect them by hand, grep them, version-control them, or delete the SQLite file and rebuild it whenever you want.

For the full architecture — JSONL parsing, message deduplication across resumed sessions, index schema — see [docs/technical-design.md](docs/technical-design.md).

---

## Links

- **Website:** https://claude-watch.pages.dev/
- **Releases:** https://github.com/rishi-anand/claude-watch/releases
- **Technical design:** [`docs/technical-design.md`](docs/technical-design.md)
- **License:** MIT
