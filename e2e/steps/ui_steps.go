package steps

import (
	"context"
	"fmt"
	"time"

	"github.com/cucumber/godog"
	"github.com/playwright-community/playwright-go"
)

// UIState holds Playwright browser state for UI scenarios.
type UIState struct {
	pw      *playwright.Playwright
	browser playwright.Browser
	page    playwright.Page
}

func NewUIState() *UIState {
	return &UIState{}
}

func (u *UIState) cleanup() {
	if u.page != nil {
		u.page.Close()
	}
	if u.browser != nil {
		u.browser.Close()
	}
	if u.pw != nil {
		u.pw.Stop()
	}
}

func InitUISteps(ctx *godog.ScenarioContext, state *ScenarioState, ui *UIState) {
	ctx.After(func(c context.Context, sc *godog.Scenario, err error) (context.Context, error) {
		ui.cleanup()
		return c, nil
	})

	ctx.Step(`^I open the browser at "([^"]*)"$`, func(rawURL string) error {
		pw, err := playwright.Run()
		if err != nil {
			return fmt.Errorf("starting playwright: %w", err)
		}
		ui.pw = pw

		browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
			Headless: playwright.Bool(true),
		})
		if err != nil {
			return fmt.Errorf("launching browser: %w", err)
		}
		ui.browser = browser

		page, err := browser.NewPage()
		if err != nil {
			return fmt.Errorf("creating page: %w", err)
		}
		ui.page = page

		if _, err := page.Goto(rawURL, playwright.PageGotoOptions{
			WaitUntil: playwright.WaitUntilStateNetworkidle,
		}); err != nil {
			return fmt.Errorf("navigating to %s: %w", rawURL, err)
		}
		return nil
	})

	ctx.Step(`^I see the conversation list$`, func() error {
		locator := ui.page.Locator("[data-testid='conversation-list'], .conversation-list, .sidebar ul, nav ul")
		if err := locator.First().WaitFor(playwright.LocatorWaitForOptions{
			Timeout: playwright.Float(5000),
		}); err != nil {
			return fmt.Errorf("conversation list not found: %w", err)
		}
		return nil
	})

	ctx.Step(`^the conversation list has at least one item$`, func() error {
		locator := ui.page.Locator("[data-testid='conversation-item'], .conversation-item, .sidebar li, nav li")
		if err := locator.First().WaitFor(playwright.LocatorWaitForOptions{
			Timeout: playwright.Float(5000),
		}); err != nil {
			return fmt.Errorf("no conversation items found: %w", err)
		}
		count, err := locator.Count()
		if err != nil {
			return err
		}
		if count == 0 {
			return fmt.Errorf("conversation list is empty")
		}
		return nil
	})

	ctx.Step(`^I click the first conversation$`, func() error {
		locator := ui.page.Locator("[data-testid='conversation-item'], .conversation-item, .sidebar li, nav li")
		if err := locator.First().WaitFor(playwright.LocatorWaitForOptions{
			Timeout: playwright.Float(5000),
		}); err != nil {
			return fmt.Errorf("no conversation items to click: %w", err)
		}
		return locator.First().Click()
	})

	ctx.Step(`^the session header is visible$`, func() error {
		locator := ui.page.Locator("[data-testid='session-header'], .session-header, header")
		if err := locator.First().WaitFor(playwright.LocatorWaitForOptions{
			Timeout: playwright.Float(5000),
		}); err != nil {
			return fmt.Errorf("session header not visible: %w", err)
		}
		return nil
	})

	ctx.Step(`^the message thread is visible$`, func() error {
		locator := ui.page.Locator("[data-testid='message-thread'], .message-thread, .messages, main")
		if err := locator.First().WaitFor(playwright.LocatorWaitForOptions{
			Timeout: playwright.Float(5000),
		}); err != nil {
			return fmt.Errorf("message thread not visible: %w", err)
		}
		return nil
	})

	ctx.Step(`^the message thread has at least one message$`, func() error {
		locator := ui.page.Locator("[data-testid='message'], .message, .message-bubble, article")
		if err := locator.First().WaitFor(playwright.LocatorWaitForOptions{
			Timeout: playwright.Float(5000),
		}); err != nil {
			return fmt.Errorf("no messages in thread: %w", err)
		}
		return nil
	})

	ctx.Step(`^I see a "([^"]*)" button in the session header$`, func(label string) error {
		selector := fmt.Sprintf("[data-testid='session-header'] button:has-text('%s'), .session-header button:has-text('%s'), header button:has-text('%s')", label, label, label)
		locator := ui.page.Locator(selector)
		if err := locator.First().WaitFor(playwright.LocatorWaitForOptions{
			Timeout: playwright.Float(5000),
		}); err != nil {
			return fmt.Errorf("button %q not found in session header: %w", label, err)
		}
		return nil
	})

	ctx.Step(`^I see the project filter dropdown$`, func() error {
		locator := ui.page.Locator("[data-testid='project-filter'], select, .project-filter")
		if err := locator.First().WaitFor(playwright.LocatorWaitForOptions{
			Timeout: playwright.Float(5000),
		}); err != nil {
			return fmt.Errorf("project filter dropdown not found: %w", err)
		}
		return nil
	})

	ctx.Step(`^I see the search input$`, func() error {
		locator := ui.page.Locator("[data-testid='search-input'], input[type='search'], input[placeholder*='earch'], .search-input")
		if err := locator.First().WaitFor(playwright.LocatorWaitForOptions{
			Timeout: playwright.Float(5000),
		}); err != nil {
			return fmt.Errorf("search input not found: %w", err)
		}
		return nil
	})

	ctx.Step(`^I type "([^"]*)" in the search bar$`, func(text string) error {
		locator := ui.page.Locator("[data-testid='search-input'], input[type='search'], input[placeholder*='earch'], .search-input")
		if err := locator.First().WaitFor(playwright.LocatorWaitForOptions{
			Timeout: playwright.Float(5000),
		}); err != nil {
			return fmt.Errorf("search input not found: %w", err)
		}
		return locator.First().Fill(text)
	})

	ctx.Step(`^I wait (\d+) milliseconds$`, func(ms int) error {
		time.Sleep(time.Duration(ms) * time.Millisecond)
		return nil
	})

	ctx.Step(`^the sidebar shows search results or "([^"]*)"$`, func(fallback string) error {
		resultsLocator := ui.page.Locator("[data-testid='search-result'], .search-result, .sidebar li, nav li")
		noResultsLocator := ui.page.Locator(fmt.Sprintf("text=%s", fallback))

		// Wait a moment for results to render
		time.Sleep(500 * time.Millisecond)

		resultsCount, _ := resultsLocator.Count()
		noResultsCount, _ := noResultsLocator.Count()

		if resultsCount == 0 && noResultsCount == 0 {
			return fmt.Errorf("neither search results nor %q message found", fallback)
		}
		return nil
	})
}
