package tests

import (
	"strings"
	"testing"
	"time"

	commontest "github.com/bobmcallan/vire-portal/tests/common"
	"github.com/chromedp/chromedp"
)

func TestGeminiPageLayout(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/admin/gemini")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	takeScreenshot(t, ctx, "gemini", "page-layout.png")

	var currentURL string
	_ = chromedp.Run(ctx, chromedp.Location(&currentURL))
	if !strings.Contains(currentURL, "/admin/gemini") {
		t.Skip("redirected away from /admin/gemini (user may not have admin role)")
	}

	pageVisible, err := isVisible(ctx, ".page")
	if err != nil {
		t.Fatalf("error checking page visibility: %v", err)
	}
	if !pageVisible {
		t.Fatal(".page class not found")
	}

	bodyVisible, err := isVisible(ctx, ".page-body")
	if err != nil {
		t.Fatalf("error checking page-body visibility: %v", err)
	}
	if !bodyVisible {
		t.Fatal(".page-body class not found")
	}
}

func TestGeminiPageNavVisible(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/admin/gemini")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	takeScreenshot(t, ctx, "gemini", "nav-visible.png")

	var currentURL string
	_ = chromedp.Run(ctx, chromedp.Location(&currentURL))
	if !strings.Contains(currentURL, "/admin/gemini") {
		t.Skip("redirected away from /admin/gemini (user may not have admin role)")
	}

	navVisible, err := isVisible(ctx, ".nav")
	if err != nil {
		t.Fatalf("error checking nav visibility: %v", err)
	}
	if !navVisible {
		t.Fatal("nav not visible on gemini page")
	}
}

func TestGeminiPageTitle(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/admin/gemini")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	takeScreenshot(t, ctx, "gemini", "page-title.png")

	// Check if we got redirected (non-admin user)
	var currentURL string
	_ = chromedp.Run(ctx, chromedp.Location(&currentURL))
	if !strings.Contains(currentURL, "/admin/gemini") {
		t.Skip("redirected away from /admin/gemini (user may not have admin role)")
	}

	titleExists, err := commontest.EvalBool(ctx, `
		(() => {
			const h1 = document.querySelector('.section-title');
			return h1 && h1.textContent.includes('GEMINI USAGE');
		})()
	`)
	if err != nil {
		t.Fatalf("error checking page title: %v", err)
	}
	if !titleExists {
		t.Error("page title does not contain 'GEMINI USAGE'")
	}
}

func TestGeminiPeriodTabs(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/admin/gemini")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	_ = chromedp.Run(ctx, chromedp.Sleep(1*time.Second))

	takeScreenshot(t, ctx, "gemini", "period-tabs.png")

	var currentURL string
	_ = chromedp.Run(ctx, chromedp.Location(&currentURL))
	if !strings.Contains(currentURL, "/admin/gemini") {
		t.Skip("redirected away from /admin/gemini (user may not have admin role)")
	}

	// Verify 3 period tabs exist (Day, Week, Month)
	tabCount, err := elementCount(ctx, ".tabs .tab")
	if err != nil {
		t.Fatalf("error counting period tabs: %v", err)
	}
	if tabCount != 3 {
		t.Errorf("period tab count = %d, want 3 (Day, Week, Month)", tabCount)
	}

	// Verify tab labels
	tabLabelsCorrect, err := commontest.EvalBool(ctx, `
		(() => {
			const tabs = document.querySelectorAll('.tabs .tab');
			if (tabs.length !== 3) return false;
			return tabs[0].textContent.trim() === 'Day' &&
				tabs[1].textContent.trim() === 'Week' &&
				tabs[2].textContent.trim() === 'Month';
		})()
	`)
	if err != nil {
		t.Fatalf("error checking tab labels: %v", err)
	}
	if !tabLabelsCorrect {
		t.Error("period tabs do not match expected: Day, Week, Month")
	}

	// Verify Week tab is active by default
	weekActive, err := commontest.EvalBool(ctx, `
		(() => {
			const tabs = document.querySelectorAll('.tabs .tab');
			return tabs.length >= 2 && tabs[1].classList.contains('active');
		})()
	`)
	if err != nil {
		t.Fatalf("error checking active tab: %v", err)
	}
	if !weekActive {
		t.Error("Week tab is not active by default")
	}
}

func TestGeminiRendersStats(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/admin/gemini")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	_ = chromedp.Run(ctx, chromedp.Sleep(2*time.Second))

	takeScreenshot(t, ctx, "gemini", "renders-stats.png")

	var currentURL string
	_ = chromedp.Run(ctx, chromedp.Location(&currentURL))
	if !strings.Contains(currentURL, "/admin/gemini") {
		t.Skip("redirected away from /admin/gemini (user may not have admin role)")
	}

	// Check for SUMMARY panel
	summaryExists, err := commontest.EvalBool(ctx, `
		(() => {
			const headers = document.querySelectorAll('.panel-header');
			return Array.from(headers).some(h => h.textContent.includes('SUMMARY'));
		})()
	`)
	if err != nil {
		t.Fatalf("error checking summary panel: %v", err)
	}

	// Check for "No usage data" message
	noDataVisible, err := commontest.EvalBool(ctx, `
		(() => {
			const banners = document.querySelectorAll('.info-banner');
			return Array.from(banners).some(b => b.textContent.includes('No usage data'));
		})()
	`)
	if err != nil {
		t.Fatalf("error checking no-data banner: %v", err)
	}

	if !summaryExists && !noDataVisible {
		t.Error("neither SUMMARY panel nor 'No usage data' message visible")
	}

	// If summary exists, check for stat fields
	if summaryExists {
		statLabels := []string{"TOTAL CALLS", "PROMPT TOKENS", "OUTPUT TOKENS"}
		for _, label := range statLabels {
			found, err := commontest.EvalBool(ctx, `
				(() => {
					const labels = document.querySelectorAll('.stock-detail-field .label');
					return Array.from(labels).some(l => l.textContent.includes('`+label+`'));
				})()
			`)
			if err != nil {
				t.Fatalf("error checking stat label %s: %v", label, err)
			}
			if !found {
				t.Errorf("stat label %q not found in summary", label)
			}
		}
	}
}

func TestGeminiNavLink(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/admin/gemini")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	takeScreenshot(t, ctx, "gemini", "nav-link.png")

	var currentURL string
	_ = chromedp.Run(ctx, chromedp.Location(&currentURL))
	if !strings.Contains(currentURL, "/admin/gemini") {
		t.Skip("redirected away from /admin/gemini (user may not have admin role)")
	}

	// Open the hamburger dropdown to check admin nav links
	err = chromedp.Run(ctx,
		chromedp.Click(".nav-hamburger", chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("error clicking hamburger: %v", err)
	}

	takeScreenshot(t, ctx, "gemini", "nav-dropdown.png")

	// Check for Gemini * link in dropdown
	geminiLinkExists, err := commontest.EvalBool(ctx, `
		(() => {
			const links = document.querySelectorAll('.nav-dropdown .nav-admin-item');
			return Array.from(links).some(a => a.textContent.includes('Gemini') && a.getAttribute('href') === '/admin/gemini');
		})()
	`)
	if err != nil {
		t.Fatalf("error checking gemini nav link: %v", err)
	}
	if !geminiLinkExists {
		t.Error("Gemini * admin nav link not found in dropdown")
	}
}

func TestGeminiPageFooterVisible(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/admin/gemini")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	takeScreenshot(t, ctx, "gemini", "footer-visible.png")

	var currentURL string
	_ = chromedp.Run(ctx, chromedp.Location(&currentURL))
	if !strings.Contains(currentURL, "/admin/gemini") {
		t.Skip("redirected away from /admin/gemini (user may not have admin role)")
	}

	footerVisible, err := isVisible(ctx, ".footer")
	if err != nil {
		t.Fatalf("error checking footer visibility: %v", err)
	}
	if !footerVisible {
		t.Fatal("footer not visible on gemini page")
	}
}

func TestGeminiPageNoJSErrors(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	errs := newJSErrorCollector(ctx)
	err := loginAndNavigate(ctx, serverURL()+"/admin/gemini")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	// Wait for Alpine to initialise and consume SSR data
	_ = chromedp.Run(ctx, chromedp.Sleep(2*time.Second))

	takeScreenshot(t, ctx, "gemini", "no-js-errors.png")

	var currentURL string
	_ = chromedp.Run(ctx, chromedp.Location(&currentURL))
	if !strings.Contains(currentURL, "/admin/gemini") {
		t.Skip("redirected away from /admin/gemini (user may not have admin role)")
	}

	if jsErrs := errs.Errors(); len(jsErrs) > 0 {
		t.Errorf("JS errors on gemini page:\n  %s", strings.Join(jsErrs, "\n  "))
	}
}

func TestGeminiPageRequiresAuth(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	var currentURL string
	err := chromedp.Run(ctx,
		chromedp.Navigate(serverURL()+"/admin/gemini"),
		chromedp.Sleep(2*time.Second),
		chromedp.Location(&currentURL),
	)
	if err != nil {
		t.Fatalf("navigation failed: %v", err)
	}

	takeScreenshot(t, ctx, "gemini", "requires-auth.png")

	if strings.Contains(currentURL, "/admin/gemini") {
		t.Errorf("unauthenticated user should be redirected away from /admin/gemini, got: %s", currentURL)
	}
}
