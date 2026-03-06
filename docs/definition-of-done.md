# Definition of Done — claude-watch

## Build
- [ ] `CGO_ENABLED=0 go build -ldflags="-s -w" -o claude-watch .` succeeds
- [ ] Binary size < 30MB
- [ ] No CGO dependencies (`otool -L ./claude-watch` shows only system libs on macOS)

## Startup
- [ ] `./claude-watch serve` starts without error
- [ ] Browser opens at http://localhost:7823 within 1 second
- [ ] All existing sessions from ~/.claude/projects/ are visible on first load
- [ ] Startup scan completes in < 30 seconds for 200+ sessions

## Session browsing
- [ ] Conversation list shows project name, first message, date, message count
- [ ] Compaction badge (orange dot) visible for sessions with compacted context
- [ ] Clicking a session loads full message thread
- [ ] User messages visually distinct from assistant messages
- [ ] Tool use blocks shown in collapsible <details> with tool name + input JSON
- [ ] Compaction markers shown inline as horizontal rule with token count
- [ ] Compaction summary shown in collapsible <details>

## Search
- [ ] `foo bar` (space) finds sessions with exact phrase "foo bar"
- [ ] `foo,bar` (comma) finds sessions containing BOTH "foo" AND "bar"
- [ ] `foo;bar` (semicolon) finds sessions containing EITHER "foo" OR "bar"
- [ ] Search results show snippet with match context
- [ ] Clicking search result opens the session

## Session ID
- [ ] Session ID visible in session header
- [ ] Copy button copies UUID to clipboard
- [ ] Copied UUID can be used with `claude --resume <uuid>`

## Memory panel
- [ ] Memory panel visible at bottom of session view (if MEMORY.md exists for that project)
- [ ] Shows raw MEMORY.md content
- [ ] Collapsible

## Hooks
- [ ] `~/work/claude-watch/hooks/` directory exists after first `serve`
- [ ] All 5 hook scripts are present and executable
- [ ] `~/.claude/settings.json` contains hook entries for all 5 events
- [ ] Starting a new Claude Code session triggers SessionStart hook
- [ ] Submitting a prompt triggers UserPromptSubmit hook
- [ ] Session captured in claude-watch within the same hook call

## Real-time capture
- [ ] PreCompact hook fires and syncs BEFORE compaction occurs
- [ ] Full conversation history preserved in .md file even after compaction
- [ ] Stop hook syncs latest messages after each Claude response

## Data persistence
- [ ] MD files written to ~/work/claude-watch/sessions/{project}/{session-id}.md
- [ ] Killing and restarting claude-watch preserves all data
- [ ] `./claude-watch rebuild` re-creates SQLite from .md files
- [ ] SQLite can be deleted and rebuilt without data loss (MD files are source of truth)

## Project filter
- [ ] Project dropdown filters conversation list
- [ ] All projects shown by default
