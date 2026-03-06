#!/usr/bin/env bash
# Claude Code Stop hook
# Payload fields: session_id, transcript_path, cwd, hook_event_name, stop_hook_active, last_assistant_message
cat | BINARY_PATH hook stop
