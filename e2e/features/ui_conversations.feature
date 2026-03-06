Feature: Conversation UI
  As a user
  I want to browse conversations in the browser
  So that I can read my Claude Code history

  Background:
    Given the claude-watch server is running
    And I open the browser at "http://localhost:7823"

  Scenario: Conversation list loads
    Then I see the conversation list
    And the conversation list has at least one item

  Scenario: Clicking a conversation shows details
    Given the conversation list has at least one item
    When I click the first conversation
    Then the session header is visible
    And the message thread is visible
    And the message thread has at least one message

  Scenario: Session ID copy button is present
    When I click the first conversation
    Then I see a "Copy" button in the session header

  Scenario: Project filter dropdown is present
    Then I see the project filter dropdown

  Scenario: Search bar is visible
    Then I see the search input

  Scenario: Searching shows results in sidebar
    When I type "implement" in the search bar
    And I wait 500 milliseconds
    Then the sidebar shows search results or "No results"
