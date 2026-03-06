Feature: Conversation API
  As a user of claude-watch
  I want to access conversation data via the API
  So that I can browse my Claude Code history

  Background:
    Given the claude-watch server is running

  Scenario: List conversations returns results
    When I request GET "/api/conversations?limit=10"
    Then the response status is 200
    And the response contains a "conversations" array
    And the "conversations" array is not empty
    And each conversation has fields "sessionId,projectName,startedAt,messageCount"

  Scenario: Conversation detail returns messages
    Given there is at least one conversation
    When I request the detail for the first conversation
    Then the response status is 200
    And the response contains a "messages" array
    And the "messages" array is not empty
    And each message has fields "uuid,msgType,timestamp"

  Scenario: Conversation detail includes content
    Given there is at least one conversation
    When I request the detail for the first conversation
    Then at least one message has non-empty "contentText" or "contentJson"

  Scenario: Unknown session returns 404
    When I request GET "/api/conversations/00000000-0000-0000-0000-000000000000"
    Then the response status is 404
