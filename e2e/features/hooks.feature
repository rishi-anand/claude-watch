Feature: Hook System
  As a user
  I want Claude Code hooks to capture sessions
  So that conversation history is preserved in real-time

  Background:
    Given the claude-watch server is running

  Scenario: Hook scripts are installed
    Then the file "~/work/claude-watch/hooks/session-start.sh" exists
    And the file "~/work/claude-watch/hooks/stop.sh" exists
    And the file "~/work/claude-watch/hooks/compact.sh" exists

  Scenario: Hook CLI processes session-start payload
    When I invoke the hook "session-start" with payload:
      """
      {
        "session_id": "test-hook-123",
        "transcript_path": "/nonexistent/path.jsonl",
        "cwd": "/tmp/test-project",
        "hook_event_name": "SessionStart",
        "source": "startup",
        "model": "claude-sonnet-4-6"
      }
      """
    Then the hook exits with code 0

  Scenario: settings.json contains hook entries
    Then the file "~/.claude/settings.json" contains "claude-watch"
    And the file "~/.claude/settings.json" contains "SessionStart"
    And the file "~/.claude/settings.json" contains "PreCompact"
