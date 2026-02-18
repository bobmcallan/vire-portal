package tests

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	commontest "github.com/bobmcallan/vire-portal/tests/common"
)

func init() {
	commontest.InitResultsDir()
}

var serverURL = commontest.GetTestURL

func newBrowser(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()
	cfg := commontest.LoadTestConfig()
	browserCfg := &commontest.BrowserConfig{
		Headless: cfg.Browser.Headless,
		Timeout:  time.Duration(cfg.Browser.TimeoutSecs) * time.Second,
	}
	return commontest.NewBrowserContext(browserCfg)
}

func takeScreenshot(t *testing.T, ctx context.Context, subdir, name string) {
	t.Helper()
	dir := getScreenshotDir(subdir)
	path := filepath.Join(dir, name)
	if err := commontest.Screenshot(ctx, path); err != nil {
		t.Logf("failed to take screenshot %s: %v", name, err)
	} else {
		t.Logf("screenshot: %s", path)
	}
}

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

func getScreenshotDir(subdir string) string {
	return commontest.GetScreenshotDir(subdir)
}
