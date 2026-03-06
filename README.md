```
▗▄▄▖▗▖    ▗▄▖ ▗▖ ▗▖▗▄▄▄ ▗▄▄▄▖    ▗▖ ▗▖ ▗▄▖▗▄▄▄▖▗▄▄▖▗▖ ▗▖
▐▌   ▐▌   ▐▌ ▐▌▐▌ ▐▌▐▌  █▐▌       ▐▌ ▐▌▐▌ ▐▌ █ ▐▌   ▐▌ ▐▌
▐▌   ▐▌   ▐▛▀▜▌▐▌ ▐▌▐▌  █▐▛▀▀▘    ▐▌ ▐▌▐▛▀▜▌ █ ▐▌   ▐▛▀▜▌
▝▚▄▄▖▐▙▄▄▖▐▌ ▐▌▝▚▄▞▘▐▙▄▄▀▐▙▄▄▖    ▐▙█▟▌▐▌ ▐▌ █ ▝▚▄▄▖▐▌ ▐▌
```

> Browse, search, and **permanently preserve** your Claude Code conversation history.

Claude Code natively compacts context — summarizing and discarding old messages to free up token space. Once compacted, that history is gone. **claude-watch** captures every conversation in real time via Claude Code hooks and stores it in plain Markdown files and a searchable SQLite index, so nothing is ever lost.

---

## Quick start

```bash
git clone https://github.com/rishi-anand/claude-watch
cd claude-watch
CGO_ENABLED=0 go build -o claude-watch .
./claude-watch serve          # starts server + opens browser at http://localhost:7823
```

Hooks are installed automatically on first run. From that point, every new Claude Code
session is captured in real time — no need to keep `serve` running.

---

## Why

- Claude Code's `/compact` destroys conversation history by design
- claude-watch installs hooks that fire **before** compaction, capturing the full transcript
- All history is stored in human-readable `.md` files — no proprietary format, no lock-in
- A local web UI lets you browse, search, and copy session IDs to resume conversations

---

## Features

- **Full history preservation** — captures every session in real time, survives compaction
- **Fast full-text search** — SQLite FTS5, supports AND / OR / phrase operators
- **Rich conversation view** — renders markdown, tool calls, tool results, compaction markers
- **Project filter** — browse sessions by project
- **Resume support** — copy a session ID and run `claude --resume <id>` to pick up where you left off
- **Zero dependencies** — single static binary, no CGO, no Docker, no database server
- **Automatic hook installation** — hooks are installed on first `serve`

---

## Install

```bash
# Clone and build (requires Go 1.22+)
git clone https://github.com/rishi-anand/claude-watch
cd claude-watch

CGO_ENABLED=0 go build -o claude-watch .
```

Or add to your `$PATH`:
```bash
mv claude-watch /usr/local/bin/claude-watch
```

---

## Usage

### Start the server

```bash
claude-watch serve
```

- Opens `http://localhost:7823` in your browser automatically
- Performs a full historical sync of all existing Claude Code sessions on startup
- Installs Claude Code hooks into `~/.claude/settings.json` (first run only)
- After first run, new sessions are captured automatically via hooks — no need to keep `serve` running for capture to work

### Flags

```bash
claude-watch serve --port 8080       # custom port (default: 7823)
claude-watch serve --no-browser      # don't auto-open browser
claude-watch serve --daemon          # run in background
```

### Rebuild the search index

If the SQLite database is deleted or corrupted, rebuild it from Markdown files:

```bash
claude-watch rebuild
```

### Hook subcommand (called automatically by Claude Code)

```bash
echo '<json payload>' | claude-watch hook stop
echo '<json payload>' | claude-watch hook session-start
echo '<json payload>' | claude-watch hook compact
```

You don't call this manually — Claude Code's hook system invokes it automatically.

---

## Search syntax

| Query | Meaning |
|-------|---------|
| `ssh tunnel` | Exact phrase "ssh tunnel" |
| `ssh,tunnel` | Messages containing BOTH "ssh" AND "tunnel" |
| `ssh;tunnel` | Messages containing EITHER "ssh" OR "tunnel" |
| `refactor,tests;ci` | "refactor" AND ("tests" OR "ci") |

---

## Resuming a conversation

1. Find the session in claude-watch
2. Click **Copy** next to the session ID
3. Run in your terminal:

```bash
claude --resume <session-id>
```

---

## Data storage

```
~/claude-watch/
  sessions/
    my-project/
      a1b2c3d4-....md     ← full conversation in Markdown (source of truth)
      e5f6a7b8-....md
    other-project/
      ...
  claude-watch.db         ← SQLite search index (safe to delete, rebuilt automatically)
  hooks/
    session-start.sh
    prompt.sh
    stop.sh
    compact.sh
    session-end.sh
```

**Markdown files are the source of truth.** SQLite is a rebuildable index. If you back up one thing, back up `~/claude-watch/sessions/`.

---

## Technical design

See [docs/technical-design.md](docs/technical-design.md) for full architecture, hook system internals, data flow, and design decisions.

---

## Building from source

```bash
# Standard build (no CGO required)
CGO_ENABLED=0 go build -o claude-watch .

# Verify it's statically linked
file ./claude-watch

# Run tests (BDD + Playwright e2e in e2e/)
cd e2e && go test ./... -v
```

---

## License

MIT
