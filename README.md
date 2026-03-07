```
▗▄▄▖▗▖    ▗▄▖ ▗▖ ▗▖▗▄▄▄ ▗▄▄▄▖    ▗▖ ▗▖ ▗▄▖▗▄▄▄▖▗▄▄▖▗▖ ▗▖
▐▌   ▐▌   ▐▌ ▐▌▐▌ ▐▌▐▌  █▐▌       ▐▌ ▐▌▐▌ ▐▌ █ ▐▌   ▐▌ ▐▌
▐▌   ▐▌   ▐▛▀▜▌▐▌ ▐▌▐▌  █▐▛▀▀▘    ▐▌ ▐▌▐▛▀▜▌ █ ▐▌   ▐▛▀▜▌
▝▚▄▄▖▐▙▄▄▖▐▌ ▐▌▝▚▄▞▘▐▙▄▄▀▐▙▄▄▖    ▐▙█▟▌▐▌ ▐▌ █ ▝▚▄▄▖▐▌ ▐▌
```

> Browse, search, and **permanently preserve** your Claude Code conversation history.

Claude Code natively compacts context — summarizing and discarding old messages to free up token space. Once compacted, that history is gone. **claude-watch** captures every conversation in real time via [Claude Code hooks](https://docs.anthropic.com/en/docs/claude-code/hooks) and stores it in plain Markdown files and a searchable SQLite index, so nothing is ever lost.

---

## Quick start

**1. Install**

```bash
curl -fsSL https://raw.githubusercontent.com/rishi-anand/claude-watch/main/install.sh | bash
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
- **Dark/light theme** — toggle in the header, preference saved across sessions
- **Zero dependencies** — single static binary, no CGO, no Docker, no database server
- **Transparent setup** — shows exactly what will be written before touching any config

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

claude-watch uses [Claude Code hooks](https://docs.anthropic.com/en/docs/claude-code/hooks) — shell scripts that Claude Code invokes at key points in a conversation. Each hook calls the `claude-watch` CLI directly (no server required):

```bash
cat | claude-watch hook stop
```

This means sessions are captured even when `claude-watch serve` isn't running.

See [docs/technical-design.md](docs/technical-design.md) for full architecture details.

---

## License

MIT
