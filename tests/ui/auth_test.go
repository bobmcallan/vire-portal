package tests

import (
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

func TestAuthGoogleLoginRedirect(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	baseURL := serverURL()
	t.Logf("Testing against server: %s", baseURL)

	var currentURL string

	// 1. Load Landing Page
	err := chromedp.Run(ctx,
		chromedp.Navigate(baseURL+"/"),
		chromedp.WaitVisible("body", chromedp.ByQuery),
	)
	if err != nil {
		t.Fatalf("failed to load landing page: %v", err)
	}

	takeScreenshot(t, ctx, "auth", "landing-before-click.png")

	// 2. Click Google Login and Wait for Redirect
	// The portal proxies the OAuth redirect server-side, so the browser should
	// be redirected to Google (or a portal error page), never to an internal address.
	err = chromedp.Run(ctx,
		chromedp.Click(`a[href="/api/auth/login/google"]`, chromedp.ByQuery),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),
		chromedp.Location(&currentURL),
	)
	if err != nil {
		takeScreenshot(t, ctx, "auth", "click-fail.png")
		t.Fatalf("failed during navigation: %v", err)
	}

	t.Logf("Landed on: %s", currentURL)
	takeScreenshot(t, ctx, "auth", "google-redirect-result.png")

	var pageTitle string
	chromedp.Run(ctx, chromedp.Title(&pageTitle))
	t.Logf("Page Title: %s", pageTitle)

	// 3. Verify the browser never sees internal Docker service names
	// The proxy should prevent addresses like "server:8080" from reaching the browser
	dockerServiceNames := []string{"server:", "vire-server:", "api:"}
	for _, name := range dockerServiceNames {
		if strings.Contains(currentURL, name) {
			t.Errorf("FAIL: Browser redirected to internal Docker address containing %q. URL: %s", name, currentURL)
		}
	}

	// 4. Verify we either reached Google OAuth or a portal error page
	// With the proxy, the browser should land on:
	// - accounts.google.com (if vire-server redirected to Google)
	// - A portal error page /error?reason=... (if vire-server was unreachable or didn't redirect)
	isGoogleAuth := strings.Contains(currentURL, "accounts.google.com")
	isPortalError := strings.Contains(currentURL, "/error")
	isPortalPage := strings.HasPrefix(currentURL, baseURL)

	if !isGoogleAuth && !isPortalError && !isPortalPage {
		t.Errorf("FAIL: Unexpected destination. Expected Google OAuth or portal error page, got URL: %s", currentURL)
	}

	// 5. If we reached Google, that's the ideal outcome
	if isGoogleAuth {
		t.Log("OK: Browser reached Google OAuth as expected")
	} else if isPortalError {
		t.Log("OK: Browser landed on portal error page (vire-server likely not configured for Google OAuth)")
	}
}
