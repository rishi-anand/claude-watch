package e2e_test

import (
	"testing"

	"github.com/cucumber/godog"
	"github.com/rishi/claude-watch/e2e/steps"
)

func TestFeatures(t *testing.T) {
	suite := godog.TestSuite{
		ScenarioInitializer: InitializeScenario,
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{"features"},
			TestingT: t,
		},
	}
	if suite.Run() != 0 {
		t.Fatal("non-zero status returned, failed to run feature tests")
	}
}

func InitializeScenario(ctx *godog.ScenarioContext) {
	state := steps.NewScenarioState()
	ui := steps.NewUIState()
	hs := steps.NewHookState()

	steps.InitCommonSteps(ctx, state)
	steps.InitAPISteps(ctx, state)
	steps.InitUISteps(ctx, state, ui)
	steps.InitHookSteps(ctx, state, hs)
}
