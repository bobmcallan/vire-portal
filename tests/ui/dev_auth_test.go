package tests

import (
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// TestDevAuthLandingNoCookie verifies landing page has no visible auth state
func TestDevAuthLandingNoCookie(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	if err := navigateAndWait(ctx, serverURL()+"/"); err != nil {
		t.Fatal(err)
	}

	takeScreenshot(t, ctx, "dev-auth", "landing-no-cookie.png")

	// Verify landing page shows login buttons (not logged in)
	loginBtns, err := elementCount(ctx, "a[href='/api/auth/login/google']")
	if err != nil {
		t.Fatal(err)
	}
	if loginBtns < 1 {
		t.Error("expected login buttons on landing page (user should not be logged in)")
	}

	// Verify dev login form is visible
	devLoginVisible, err := isVisible(ctx, ".landing-dev-login")
	if err != nil {
		t.Fatal(err)
	}
	if !devLoginVisible {
		t.Error("dev login form should be visible on landing page")
	}
}

// TestDevAuthLoginRedirect verifies DEV LOGIN redirects to dashboard
func TestDevAuthLoginRedirect(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	// Navigate to landing page
	if err := navigateAndWait(ctx, serverURL()+"/"); err != nil {
		t.Fatal(err)
	}

	takeScreenshot(t, ctx, "dev-auth", "login-before-click.png")

	// Check dev login form exists
	devLoginVisible, err := isVisible(ctx, ".landing-dev-login")
	if err != nil {
		t.Fatal(err)
	}
	if !devLoginVisible {
		t.Fatal("dev login form not visible on landing page")
	}

	// Click the submit button (browser will handle form submission with hidden inputs)
	err = chromedp.Run(ctx,
		chromedp.Click(".landing-dev-login button[type='submit']", chromedp.ByQuery),
	)
	if err != nil {
		t.Fatal(err)
	}

	// Wait for page to load after form submission
	var currentURL string
	err = chromedp.Run(ctx,
		chromedp.Sleep(1500*time.Millisecond),
		chromedp.Location(&currentURL),
		chromedp.WaitVisible("body", chromedp.ByQuery),
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

	takeScreenshot(t, ctx, "dev-auth", "cookie-jwt-check.png")

	// Verify nav is visible (confirms logged in state)
	navVisible, err := isVisible(ctx, ".nav")
	if err != nil {
		t.Fatal(err)
	}
	if !navVisible {
		t.Error("nav should be visible when logged in")
	}

	// Verify we can access dashboard content
	dashboardSections, err := elementCount(ctx, ".dashboard-section")
	if err != nil {
		t.Fatal(err)
	}
	if dashboardSections < 1 {
		t.Errorf("expected at least 1 dashboard section, got: %d", dashboardSections)
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

	takeScreenshot(t, ctx, "dev-auth", "logout-before.png")

	// Verify logged in (nav visible)
	navVisible, err := isVisible(ctx, ".nav")
	if err != nil {
		t.Fatal(err)
	}
	if !navVisible {
		t.Fatal("expected nav visible before logout")
	}

	// Navigate to landing page (should clear cookie/logout)
	if err := navigateAndWait(ctx, serverURL()+"/"); err != nil {
		t.Fatal(err)
	}

	takeScreenshot(t, ctx, "dev-auth", "logout-after.png")

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

	takeScreenshot(t, ctx, "dev-auth", "logout-dashboard-attempt.png")

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

// TestDevAuthSettingsMCPEndpoint verifies the DEV MCP section appears in settings
func TestDevAuthSettingsMCPEndpoint(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	// Login first
	err := loginAndNavigate(ctx, serverURL()+"/settings")
	if err != nil {
		t.Fatal(err)
	}

	takeScreenshot(t, ctx, "dev-auth", "settings-mcp-page.png")

	// Verify DEV MCP section is present (contains the title)
	var mcpSectionText string
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const sections = document.querySelectorAll('.dashboard-section');
				for (const section of sections) {
					const title = section.querySelector('.section-title');
					if (title && title.textContent.includes('DEV MCP')) {
						return section.innerText;
					}
				}
				return '';
			})()
		`, &mcpSectionText),
	)
	if err != nil {
		t.Fatal(err)
	}

	if mcpSectionText == "" {
		// Debug: log all section titles
		var allTitles string
		chromedp.Run(ctx,
			chromedp.Evaluate(`
				(() => {
					const titles = document.querySelectorAll('.section-title');
					return Array.from(titles).map(t => t.textContent).join(', ');
				})()
			`, &allTitles),
		)
		t.Fatalf("DEV MCP ENDPOINT section not found. Available sections: %s", allTitles)
	}

	t.Logf("Found DEV MCP section: %s", truncate(mcpSectionText, 200))
}

// TestDevAuthSettingsMCPURL validates the MCP URL format
func TestDevAuthSettingsMCPURL(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	// Login first
	err := loginAndNavigate(ctx, serverURL()+"/settings")
	if err != nil {
		t.Fatal(err)
	}

	takeScreenshot(t, ctx, "dev-auth", "settings-mcp-url.png")

	// Extract the MCP URL from the code block
	var mcpURL string
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const sections = document.querySelectorAll('.dashboard-section');
				for (const section of sections) {
					const title = section.querySelector('.section-title');
					if (title && title.textContent.includes('DEV MCP')) {
						const codeBlock = section.querySelector('.code-block');
						return codeBlock ? codeBlock.textContent.trim() : '';
					}
				}
				return '';
			})()
		`, &mcpURL),
	)
	if err != nil {
		t.Fatal(err)
	}

	if mcpURL == "" {
		t.Fatal("MCP URL not found in DEV MCP section")
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
