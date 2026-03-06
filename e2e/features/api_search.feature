Feature: Search API
  As a user of claude-watch
  I want to search across conversations
  So that I can find specific discussions

  Background:
    Given the claude-watch server is running

  Scenario: Single word search returns results
    When I search for "implement"
    Then the response status is 200
    And the response contains a "results" array

  Scenario: AND search with comma operator
    When I search for "go,build"
    Then the response status is 200
    And the search results are tagged with operator "AND"

  Scenario: OR search with semicolon operator
    When I search for "go;python"
    Then the response status is 200

  Scenario: OR returns at least as many results as AND
    When I search for "go,build" and store the count as "and_count"
    And I search for "go;build" and store the count as "or_count"
    Then "or_count" is greater than or equal to "and_count"

  Scenario: Multi-word phrase search
    When I search for "go build"
    Then the response status is 200
    And the response contains a "results" array

  Scenario: Status endpoint is healthy
    When I request GET "/api/status"
    Then the response status is 200
    And the response field "ok" is true
    And the response field "sessionCount" is greater than 0
