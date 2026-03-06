# Agent Definitions — claude-watch

This project uses parallel agent development. Each agent owns a specific domain.

## backend-agent
**Domain:** All Go code — data pipeline, SQLite, HTTP API, sync logic
**Entry point:** `main.go`, `internal/`
**Key files:**
- `internal/claude/` — JSONL parsing
- `internal/markdown/` — .md file writing
- `internal/db/` + `internal/store/` — SQLite schema and queries
- `internal/api/` — HTTP endpoints
- `internal/sync/` — sync orchestration
- `internal/hooks/` — hook script installation

**Constraints:**
- CGO_ENABLED=0, modernc.org/sqlite only
- No external HTTP router — stdlib net/http only
- bufio.Scanner with 10MB buffer for JSONL parsing
- MD files are append-only (never rewrite, only append new messages)

**Definition of done:**
- `CGO_ENABLED=0 go build .` succeeds
- All API endpoints return valid JSON
- Sessions from ~/.claude/projects/ appear on startup

## frontend-agent
**Domain:** `static/` — HTML, CSS, JavaScript
**Key files:** `static/index.html`, `static/style.css`, `static/app.js`

**Constraints:**
- No framework, no build step, no CDN dependencies
- Single-file JS (no modules)
- Dark terminal-style theme

**Definition of done:**
- Conversation list loads and is filterable by project
- Clicking a session shows full message thread
- Compaction markers and tool use blocks render correctly
- Search with all 3 operators works
- Session ID copy button works

## hooks-docs-agent
**Domain:** Hook scripts + documentation
**Key files:** `hooks/*.sh`, `CLAUDE.md`, `AGENTS.md`, `docs/`

**Definition of done:**
- Hook scripts exist in `hooks/` and use CLI invocation pattern
- CLAUDE.md accurately describes the project
- docs/definition-of-done.md has acceptance criteria
