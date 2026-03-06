#!/usr/bin/env bash
# Claude Code SessionEnd hook
# Payload fields: session_id, transcript_path, cwd, hook_event_name, reason
cat | BINARY_PATH hook session-end
