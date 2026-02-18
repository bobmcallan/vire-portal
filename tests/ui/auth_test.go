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
	// We wait for the URL to change from the landing page.
	// Since the redirect targets a different domain/port, we expect navigation.
	err = chromedp.Run(ctx,
		chromedp.Click(`a[href="/api/auth/login/google"]`, chromedp.ByQuery),
		// Wait for URL to NOT be the landing page
		chromedp.WaitReady("body", chromedp.ByQuery), // Wait for next page body
		chromedp.Sleep(2*time.Second),                // Grace period for redirects
		chromedp.Location(&currentURL),
	)
	if err != nil {
		takeScreenshot(t, ctx, "auth", "click-fail.png")
		t.Fatalf("failed during navigation: %v", err)
	}

	t.Logf("Landed on: %s", currentURL)
	takeScreenshot(t, ctx, "auth", "google-redirect-result.png")

	// 3. Analyze Result
	// If the portal redirect worked, we should be at the API server (or Google).
	// If the config is wrong (e.g. localhost:8501 but server is down), Chrome might show an error page.
	// Chrome error pages usually have "chrome-error://" or title "Error".

	var pageTitle string
	chromedp.Run(ctx, chromedp.Title(&pageTitle))
	t.Logf("Page Title: %s", pageTitle)

	if strings.Contains(currentURL, "chrome-error") || strings.Contains(pageTitle, "Error") || strings.Contains(pageTitle, "Refused") {
		t.Errorf("FAIL: Browser failed to connect to redirect target. URL: %s", currentURL)
	}

	// Verify we are NOT on the portal landing page anymore
	if strings.HasPrefix(currentURL, baseURL) && !strings.Contains(currentURL, "api/auth") {
		t.Errorf("FAIL: Did not redirect away from portal. URL: %s", currentURL)
	}

	// Verify we attempted to hit the API login endpoint
	if !strings.Contains(currentURL, "/api/auth/login/google") && !strings.Contains(currentURL, "accounts.google.com") {
		t.Errorf("FAIL: Unexpected destination. URL: %s", currentURL)
	}

	// Strict check: We should eventually reach Google.
	// If we are stuck on localhost, the API server didn't redirect us.
	if strings.Contains(currentURL, "localhost") || strings.Contains(currentURL, "127.0.0.1") {
		t.Errorf("FAIL: Stuck on local redirect. API server did not forward to Google. URL: %s", currentURL)
	}
}
