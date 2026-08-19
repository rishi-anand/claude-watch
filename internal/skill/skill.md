---
name: claude-watch
description: Browse, search, and export your own Claude Code conversation history via the `claude-watch` CLI. Use when the user asks about prior conversations, wants to look up something they discussed with Claude before, wants to export a past session to markdown, or asks "what was that session where I…". Works even after Claude Code has compacted the context — history is preserved in per-session markdown files plus a SQLite FTS5 index.
---

# claude-watch

`claude-watch` is a local tool that captures every Claude Code conversation (via hooks) and stores it as human-readable markdown plus a searchable SQLite index. It solves a specific problem: Claude Code's `/compact` throws away conversation history, and the JSONL files under `~/.claude/projects/…` don't have a good way to search across.

Prefer these CLI subcommands over hand-rolled `grep`/`jq` pipelines against `~/.claude/projects/**/*.jsonl`. The CLI already understands the JSONL schema, strips tool-only turns, deduplicates resumed sessions, and shares the same FTS5 search index as the web UI.

## Quick check

Before recommending or invoking, confirm the binary is on `PATH`:

```bash
claude-watch version
```

This confirms the binary is on `PATH` and runs. It prints the release tag (or `dev` plus a git revision for local builds), the Go version, and the platform — quote it verbatim in any bug report.

If the command is not found, the user can install with the one-liner from the project README (or `go build .` in the repo). Don't invent an install command — say the tool isn't installed and stop.

## Subcommands

### `search <query>` — Full-text search across all conversations

Uses SQLite FTS5. All words must match (implicit AND). Hyphens and apostrophes are word separators (`palette-agentic-cli` → matches `palette` AND `agentic` AND `cli`).

```bash
claude-watch search "ssh tunnel"                       # all sessions
claude-watch search "airgapped cluster" --repo /path   # scope to one repo
claude-watch search "compact boundary" --session-id <id>
claude-watch search "foo" --limit 20 --page 2          # pagination
claude-watch search "foo" --expand                     # print full message body per hit
claude-watch search "foo" --msg-id 97fd879c            # drill into one message (implies --expand)
claude-watch search "foo" --json                       # machine-readable
```

Default output is a five-column table: `CONVERSATION ID | REPO | MSG | TIMESTAMP | MATCH`. The `REPO` column is the project (repo) the session ran in — the same name `--repo` filters on. The `MSG` column is the first 8 chars of the message UUID — enough to feed back into `--msg-id`. Matches are highlighted with ANSI in the terminal; `--json` strips the `<mark>` tags and adds `session_id`, `project`, full message `uuid`, and `timestamp` per hit.

**Reading a specific hit in full.** From the table, grab the `MSG` prefix and re-run with `--msg-id <prefix>` (or the full UUID from `--json`). That filters to just that message and prints its full `content_text` with query terms highlighted — no need to `export` the whole session just to read one turn. `--expand` alone does the same for every hit on the page (useful when you want the top N matches in full).

Read the query semantics before framing results for the user:
- No phrase matching — quoted strings aren't respected, they're just split on whitespace.
- No OR / NOT operators.
- Case-insensitive.

If the user asks "find where I talked about X", start here.

### `list` — List sessions (no SQLite needed)

Reads JSONL files directly. Good for a quick catalog.

```bash
claude-watch list                       # all projects
claude-watch list --repo /path/to/repo  # one project
claude-watch list --json                # for scripting
```

Columns: session id, project, started → last-active, `[m:<messages> t:<tool-calls>]`, first user prompt (truncated to 100 chars).

### `export --session-id <id>` — Export one conversation to Markdown

```bash
claude-watch export --session-id <id>                          # stdout
claude-watch export --session-id <id> -o out.md                # save
claude-watch export --session-id <id> -o out.md --include-tool-msg   # full tool-call detail
claude-watch export --session-id <id> --repo /path/to/repo     # narrow the search
```

Default output shows user + assistant text only. Add `--include-tool-msg` when the user wants to see tool calls, inputs, and results (e.g., "what commands did I run in that session").

### `serve` — Web UI

```bash
claude-watch serve                    # open browser at localhost:7823
claude-watch serve --port 8080
claude-watch serve --no-browser
```

Only invoke if the user explicitly wants the UI. `serve` also installs Claude Code hooks on first run and does a full sync — don't launch it just to answer a search question. Use `search`/`list` for that.

### `rebuild` — Rebuild the SQLite FTS5 index

```bash
claude-watch rebuild
```

Use when `search` returns 0 hits for something the user swears they discussed, or the DB was deleted. Read-only fallback: `list` and `export` don't need the DB.

### `hook <event>` — Called by Claude Code hooks; don't invoke manually

## Typical patterns

**"Find where I discussed the FIPS build flag"**
```bash
claude-watch search "FIPS build" --limit 20
```
For a promising hit, either read that one turn in full…
```bash
claude-watch search "FIPS build" --msg-id <MSG-prefix-from-table>
```
…or export the whole session for surrounding context:
```bash
claude-watch export --session-id <id> -o /tmp/session.md
```
Prefer `--msg-id` when the snippet already looks like the answer; fall back to `export` when you need the conversation around it.

**"Show me my recent sessions in this repo"**
```bash
claude-watch list --repo "$PWD"
```

**"Dump the tool calls from session X"**
```bash
claude-watch export --session-id <id> --include-tool-msg -o /tmp/full.md
```

**"Pull the JSON so I can pipe it through jq"**
```bash
claude-watch search "kubevirt" --json | jq '.results[] | {id: .session_id, ts: .timestamp}'
claude-watch list --json | jq '.[] | select(.messages > 100)'
```

## When NOT to use this

- The user is asking about *this* conversation — you already have it in context, don't shell out.
- The user wants to modify their conversation history — this tool is read-only by design.
- The user is asking about someone else's Claude sessions — the store is local to their machine.

## Data locations (for troubleshooting only)

- `~/claude-watch/sessions/<project>/<session-id>.md` — markdown source of truth
- `~/claude-watch/claude-watch.db` — SQLite FTS5 index (rebuildable)
- `~/.claude/projects/<encoded-cwd>/*.jsonl` — Claude Code's raw transcripts (never modify)

Never `rm` anything under `~/.claude/projects/` — that's Claude Code's own state.
