#!/usr/bin/env bash
# Claude Code SessionStart hook
# Payload fields: session_id, transcript_path, cwd, hook_event_name, source, model
cat | BINARY_PATH hook session-start
