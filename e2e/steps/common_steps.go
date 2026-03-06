package steps

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/cucumber/godog"
)

const baseURL = "http://localhost:7823"

// ScenarioState holds per-scenario state shared across step definitions.
type ScenarioState struct {
	Resp       *http.Response
	Body       []byte
	JSONData   map[string]interface{}
	StatusCode int

	// for storing named counts (search comparisons)
	Counts map[string]int

	// first conversation session ID (cached)
	FirstSessionID string
}

func NewScenarioState() *ScenarioState {
	return &ScenarioState{
		Counts: make(map[string]int),
	}
}

// ParseJSON parses s.Body into s.JSONData.
func (s *ScenarioState) ParseJSON() error {
	s.JSONData = make(map[string]interface{})
	return json.Unmarshal(s.Body, &s.JSONData)
}

func InitCommonSteps(ctx *godog.ScenarioContext, state *ScenarioState) {
	ctx.Step(`^the claude-watch server is running$`, func() error {
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get(baseURL + "/api/status")
		if err != nil {
			return fmt.Errorf("server not reachable at %s: %w", baseURL, err)
		}
		resp.Body.Close()
		if resp.StatusCode != 200 {
			return fmt.Errorf("expected status 200 from /api/status, got %d", resp.StatusCode)
		}
		return nil
	})
}
