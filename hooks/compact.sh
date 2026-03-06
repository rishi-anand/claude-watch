#!/usr/bin/env bash
# Claude Code PreCompact hook — CRITICAL: fires before compaction destroys history
# Payload fields: session_id, transcript_path, cwd, hook_event_name, trigger, custom_instructions
cat | BINARY_PATH hook compact
