package tests

import (
	"context"
	"testing"

	commontest "github.com/bobmcallan/vire-portal/tests/common"
)

var serverURL = commontest.ServerURL

func newBrowser(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()
	return commontest.NewBrowserContext(commontest.DefaultBrowserConfig())
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
