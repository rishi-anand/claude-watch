package steps

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cucumber/godog"
)

// HookState holds state for hook scenario steps.
type HookState struct {
	exitCode int
}

func NewHookState() *HookState {
	return &HookState{}
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}

func InitHookSteps(ctx *godog.ScenarioContext, state *ScenarioState, hs *HookState) {
	ctx.Step(`^the file "([^"]*)" exists$`, func(path string) error {
		expanded := expandHome(path)
		if _, err := os.Stat(expanded); os.IsNotExist(err) {
			return fmt.Errorf("file does not exist: %s", expanded)
		}
		return nil
	})

	ctx.Step(`^I invoke the hook "([^"]*)" with payload:$`, func(event string, payload *godog.DocString) error {
		// Find the claude-watch binary
		binary := filepath.Join(os.Getenv("HOME"), "work", "src", "claude-watch", "claude-watch")
		if _, err := os.Stat(binary); os.IsNotExist(err) {
			// Try looking on PATH
			var pathErr error
			binary, pathErr = exec.LookPath("claude-watch")
			if pathErr != nil {
				return fmt.Errorf("claude-watch binary not found")
			}
		}

		cmd := exec.Command(binary, "hook", event)
		cmd.Stdin = strings.NewReader(payload.Content)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		err := cmd.Run()
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				hs.exitCode = exitErr.ExitCode()
			} else {
				return fmt.Errorf("running hook: %w", err)
			}
		} else {
			hs.exitCode = 0
		}
		return nil
	})

	ctx.Step(`^the hook exits with code (\d+)$`, func(expected int) error {
		if hs.exitCode != expected {
			return fmt.Errorf("expected exit code %d, got %d", expected, hs.exitCode)
		}
		return nil
	})

	ctx.Step(`^the file "([^"]*)" contains "([^"]*)"$`, func(path, substr string) error {
		expanded := expandHome(path)
		data, err := os.ReadFile(expanded)
		if err != nil {
			return fmt.Errorf("reading %s: %w", expanded, err)
		}
		if !strings.Contains(string(data), substr) {
			return fmt.Errorf("file %s does not contain %q", expanded, substr)
		}
		return nil
	})
}
