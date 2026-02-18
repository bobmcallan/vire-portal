package tests

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	commontest "github.com/bobmcallan/vire-portal/tests/common"
	"github.com/chromedp/chromedp"
)

// suiteName is set by TestMain based on the test pattern
var suiteName = "ui"

// runner is the global test runner for artifact collection
var runner *commontest.TestRunner

// initRunner initializes the test runner for a specific suite.
func initRunner(suite string) *commontest.TestRunner {
	suiteName = suite
	runner = commontest.NewTestRunner(suite)
	return runner
}

// Note: InitResultsDir() is NOT called in init() anymore.
// The wrapper script (ui-test.sh) creates the results directory
// and the tests use commontest.GetResultsDir() which will use
// the existing directory or create one if needed.

var serverURL = commontest.GetTestURL

// newBrowser creates a browser context for testing.
func newBrowser(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()
	cfg := commontest.LoadTestConfig()
	browserCfg := &commontest.BrowserConfig{
		Headless: cfg.Browser.Headless,
		Timeout:  time.Duration(cfg.Browser.TimeoutSecs) * time.Second,
	}
	return commontest.NewBrowserContext(browserCfg)
}

// takeScreenshot captures a screenshot to the results directory.
// Can be called with either:
//
//	takeScreenshot(t, ctx, "name.png")           - saves to results dir
//	takeScreenshot(t, ctx, "subdir", "name.png") - saves to results/subdir/
func takeScreenshot(t *testing.T, ctx context.Context, args ...string) {
	t.Helper()

	var path string
	switch len(args) {
	case 1:
		// Add .png extension if not present
		name := args[0]
		if filepath.Ext(name) != ".png" {
			name = name + ".png"
		}
		path = filepath.Join(commontest.GetResultsDir(), name)
	case 2:
		// Add .png extension if not present
		name := args[1]
		if filepath.Ext(name) != ".png" {
			name = name + ".png"
		}
		path = filepath.Join(commontest.GetResultsDir(), args[0], name)
		os.MkdirAll(filepath.Dir(path), 0755)
	default:
		t.Logf("invalid takeScreenshot call: expected 1 or 2 string args, got %d", len(args))
		return
	}

	// Ensure results directory exists
	os.MkdirAll(filepath.Dir(path), 0755)

	// Set viewport to capture full page content
	var dims struct {
		Width  int64 `json:"width"`
		Height int64 `json:"height"`
	}
	err := chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				return {
					width: Math.max(document.documentElement.scrollWidth, window.innerWidth),
					height: Math.max(document.documentElement.scrollHeight, window.innerHeight)
				};
			})()
		`, &dims),
	)
	if err == nil && dims.Width > 0 && dims.Height > 0 {
		_ = chromedp.Run(ctx, chromedp.EmulateViewport(dims.Width, dims.Height))
	}

	if err := commontest.Screenshot(ctx, path); err != nil {
		t.Logf("failed to take screenshot %s: %v", path, err)
	} else {
		t.Logf("screenshot: %s", path)
	}
}

// newJSErrorCollector creates a collector for JavaScript errors.
func newJSErrorCollector(ctx context.Context) *commontest.JSErrorCollector {
	return commontest.NewJSErrorCollector(ctx)
}

func navigateAndWait(ctx context.Context, url string) error {
	return commontest.NavigateAndWait(ctx, url, 0)
}

func loginAndNavigate(ctx context.Context, targetURL string) error {
	return commontest.LoginAndNavigate(ctx, targetURL, 0)
}

func isHidden(ctx context.Context, selector string) (bool, error) {
	return commontest.IsHidden(ctx, selector)
}

func isVisible(ctx context.Context, selector string) (bool, error) {
	return commontest.IsVisible(ctx, selector)
}

func elementCount(ctx context.Context, selector string) (int, error) {
	return commontest.ElementCount(ctx, selector)
}

func getResultsDir() string {
	return commontest.GetResultsDir()
}

// ---- Test Assertion Helpers ----

// assertVisible asserts that an element is visible.
// Returns an error if not visible, suitable for use with runner.RunTest.
func assertVisible(ctx context.Context, selector, description string) error {
	visible, err := commontest.IsVisible(ctx, selector)
	if err != nil {
		return fmt.Errorf("error checking %s visibility: %w", description, err)
	}
	if !visible {
		return fmt.Errorf("%s not visible", description)
	}
	return nil
}

// assertElementCount asserts that an element count meets a minimum.
func assertElementCount(ctx context.Context, selector string, minCount int, description string) error {
	count, err := commontest.ElementCount(ctx, selector)
	if err != nil {
		return fmt.Errorf("error counting %s: %w", description, err)
	}
	if count < minCount {
		return fmt.Errorf("%s count = %d, want >= %d", description, count, minCount)
	}
	return nil
}

// assertTextContains asserts that an element's text contains expected text.
func assertTextContains(ctx context.Context, selector, expected, description string) error {
	contains, actual, err := commontest.TextContains(ctx, selector, expected)
	if err != nil {
		return fmt.Errorf("error checking %s text: %w", description, err)
	}
	if !contains {
		return fmt.Errorf("%s = %q, want contains %q", description, actual, expected)
	}
	return nil
}

// assertNoJSErrors asserts that no JavaScript errors occurred.
func assertNoJSErrors(collector *commontest.JSErrorCollector) error {
	if errs := collector.Errors(); len(errs) > 0 {
		return fmt.Errorf("JavaScript errors:\n  %s", joinErrors(errs))
	}
	return nil
}

// assertEval asserts that a JavaScript expression evaluates to true.
func assertEval(ctx context.Context, expr, description string) error {
	result, err := commontest.EvalBool(ctx, expr)
	if err != nil {
		return fmt.Errorf("error evaluating %s: %w", description, err)
	}
	if !result {
		return fmt.Errorf("%s: expression returned false", description)
	}
	return nil
}

func joinErrors(errs []string) string {
	result := ""
	for i, e := range errs {
		if i > 0 {
			result += "\n  "
		}
		result += e
	}
	return result
}
