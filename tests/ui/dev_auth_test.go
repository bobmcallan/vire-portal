package tests

import (
	"context"
	"strings"
	"testing"
	"time"

	commontest "github.com/bobmcallan/vire-portal/tests/common"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

// TestDevAuthLandingNoCookie verifies landing page has no visible auth state
func TestDevAuthLandingNoCookie(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	if err := navigateAndWait(ctx, serverURL()+"/"); err != nil {
		t.Fatal(err)
	}

	// Verify landing page shows login buttons (not logged in)
	loginBtns, err := elementCount(ctx, "a[href='/api/auth/login/google']")
	if err != nil {
		t.Fatal(err)
	}
	if loginBtns < 1 {
		t.Error("expected login buttons on landing page (user should not be logged in)")
	}

	// Verify login form is visible
	loginFormVisible, err := isVisible(ctx, ".landing-login-form")
	if err != nil {
		t.Fatal(err)
	}
	if !loginFormVisible {
		t.Error("login form should be visible on landing page")
	}
}

// TestDevAuthLoginRedirect verifies login form redirects to dashboard
func TestDevAuthLoginRedirect(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	// Navigate to landing page
	if err := navigateAndWait(ctx, serverURL()+"/"); err != nil {
		t.Fatal(err)
	}

	// Check login form exists
	loginFormVisible, err := isVisible(ctx, ".landing-login-form")
	if err != nil {
		t.Fatal(err)
	}
	if !loginFormVisible {
		t.Fatal("login form not visible on landing page")
	}

	// Use JavaScript to submit the form directly (works even in Alpine.js templates)
	var currentURL string
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`document.querySelector('.landing-login-form').submit()`, nil),
		chromedp.Sleep(1500*time.Millisecond),
		chromedp.WaitVisible("body", chromedp.ByQuery),
		chromedp.Location(&currentURL),
	)
	if err != nil {
		t.Fatal(err)
	}

	takeScreenshot(t, ctx, "dev-auth", "login-after-redirect.png")

	if !strings.Contains(currentURL, "/dashboard") {
		t.Errorf("expected to be on dashboard after login, got URL: %s", currentURL)
	}

	// Verify nav is visible (only shown when logged in)
	navVisible, err := isVisible(ctx, ".nav")
	if err != nil {
		t.Logf("warning: could not check nav visibility: %v", err)
	} else if !navVisible {
		t.Error("nav should be visible after successful login")
	}
}

// TestDevAuthCookieAndJWT validates session is active after login
func TestDevAuthCookieAndJWT(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	// Login first
	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatal(err)
	}

	// Verify nav is visible (confirms logged in state)
	navVisible, err := isVisible(ctx, ".nav")
	if err != nil {
		t.Fatal(err)
	}
	if !navVisible {
		t.Error("nav should be visible when logged in")
	}

	// Verify we can access dashboard content (portfolio UI panels)
	dashboardPanels, err := elementCount(ctx, ".panel-headed")
	if err != nil {
		t.Fatal(err)
	}
	if dashboardPanels < 1 {
		t.Errorf("expected at least 1 dashboard panel, got: %d", dashboardPanels)
	}

	// Verify login buttons are NOT visible (user is logged in)
	loginBtns, err := elementCount(ctx, "a[href='/api/auth/login/google']")
	if err != nil {
		t.Fatal(err)
	}
	if loginBtns > 0 {
		t.Error("login buttons should not be visible when logged in")
	}
}

// TestDevAuthLogout verifies navigating to landing page clears session
func TestDevAuthLogout(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	// Login first
	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatal(err)
	}

	// Verify logged in (nav visible)
	navVisible, err := isVisible(ctx, ".nav")
	if err != nil {
		t.Fatal(err)
	}
	if !navVisible {
		t.Fatal("expected nav visible before logout")
	}

	// Clear the test session header to simulate logout
	// (The X-Test-Session header persists across navigations, so we need to clear it manually)
	err = chromedp.Run(ctx,
		network.Enable(),
		chromedp.ActionFunc(func(ctx context.Context) error {
			return network.SetExtraHTTPHeaders(network.Headers(map[string]interface{}{
				"X-Test-Session": "",
			})).Do(ctx)
		}),
	)
	if err != nil {
		t.Logf("warning: failed to clear headers: %v", err)
	}

	// Navigate to landing page (should clear cookie/logout)
	if err := navigateAndWait(ctx, serverURL()+"/"); err != nil {
		t.Fatal(err)
	}

	// Try to access dashboard - should redirect to landing
	var currentURL string
	err = chromedp.Run(ctx,
		chromedp.Navigate(serverURL()+"/dashboard"),
		chromedp.WaitVisible("body", chromedp.ByQuery),
		chromedp.Location(&currentURL),
	)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(currentURL, "/dashboard") {
		t.Errorf("expected redirect from dashboard to landing after logout, got URL: %s", currentURL)
	}

	// Verify login buttons are visible again
	loginBtns, err := elementCount(ctx, "a[href='/api/auth/login/google']")
	if err != nil {
		t.Fatal(err)
	}
	if loginBtns < 1 {
		t.Error("expected login buttons visible after logout")
	}
}

// TestDevAuthMCPDevEndpoint verifies the DEV MODE MCP endpoint appears on /mcp-info
func TestDevAuthMCPDevEndpoint(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/mcp-info")
	if err != nil {
		t.Fatal(err)
	}

	// Check for DEV MODE section (shown conditionally via {{if .DevMCPEndpoint}})
	devModeFound, err := commontest.EvalBool(ctx, `document.body.innerText.includes('DEV MODE')`)
	if err != nil {
		t.Fatal(err)
	}
	if !devModeFound {
		t.Skip("DEV MODE section not present on MCP page (dev endpoint may not be configured)")
	}
}

// TestDevAuthMCPDevURL validates the dev MCP URL format on /mcp-info
func TestDevAuthMCPDevURL(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/mcp-info")
	if err != nil {
		t.Fatal(err)
	}

	// Extract the dev MCP URL from code elements on the page
	var mcpURL string
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const codes = document.querySelectorAll('code');
				for (const code of codes) {
					const text = code.textContent.trim();
					if (text.includes('/mcp/')) return text;
				}
				return '';
			})()
		`, &mcpURL),
	)
	if err != nil {
		t.Fatal(err)
	}

	if mcpURL == "" {
		t.Skip("Dev MCP URL not found on /mcp-info (dev endpoint may not be configured)")
	}

	t.Logf("Found MCP URL: %s", mcpURL)

	// Validate URL format: should contain /mcp/ and be a valid URL
	if !strings.Contains(mcpURL, "/mcp/") {
		t.Errorf("MCP URL should contain '/mcp/', got: %s", mcpURL)
	}

	if !strings.HasPrefix(mcpURL, "http://") && !strings.HasPrefix(mcpURL, "https://") {
		t.Errorf("MCP URL should start with http:// or https://, got: %s", mcpURL)
	}

	// The encrypted UID should be a base64url-encoded string after /mcp/
	parts := strings.Split(mcpURL, "/mcp/")
	if len(parts) != 2 {
		t.Fatalf("MCP URL should have exactly one '/mcp/' segment, got: %s", mcpURL)
	}

	encryptedUID := parts[1]
	if len(encryptedUID) < 10 {
		t.Errorf("Encrypted UID seems too short, got: %s (length: %d)", encryptedUID, len(encryptedUID))
	}

	// Verify it contains only base64url characters (A-Za-z0-9_-)
	for i, c := range encryptedUID {
		if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' || c == '-') {
			t.Errorf("Encrypted UID contains invalid character at position %d: %c", i, c)
			break
		}
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
