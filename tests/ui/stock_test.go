package tests

import (
	"strings"
	"testing"
	"time"

	commontest "github.com/bobmcallan/vire-portal/tests/common"
	"github.com/chromedp/chromedp"
)

func TestStockPageLoad(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/stock/BHP")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	takeScreenshot(t, ctx, "stock", "page-load.png")

	visible, err := isVisible(ctx, ".page")
	if err != nil {
		t.Fatalf("error checking stock page visibility: %v", err)
	}
	if !visible {
		t.Fatal("stock .page not visible after login")
	}

	// Verify ticker heading is rendered
	headingVisible, err := isVisible(ctx, ".stock-ticker-heading")
	if err != nil {
		t.Fatalf("error checking ticker heading: %v", err)
	}
	if !headingVisible {
		t.Fatal("stock ticker heading (.stock-ticker-heading) not visible")
	}

	// Verify heading text matches ticker
	headingText, err := commontest.EvalBool(ctx, `
		(() => {
			const h = document.querySelector('.stock-ticker-heading');
			return h && h.textContent.trim() === 'BHP';
		})()
	`)
	if err != nil {
		t.Fatalf("error checking heading text: %v", err)
	}
	if !headingText {
		t.Error("stock ticker heading does not show 'BHP'")
	}

	// Verify back link to dashboard exists
	backLink, err := commontest.Exists(ctx, `.stock-back-link a[href="/dashboard"]`)
	if err != nil {
		t.Fatalf("error checking back link: %v", err)
	}
	if !backLink {
		t.Error("back link to /dashboard not found")
	}

	// Verify placeholder sections exist
	placeholders := []string{"TRADE HISTORY", "PRICE CHART", "FILINGS", "STRATEGY ALIGNMENT"}
	for _, title := range placeholders {
		found, err := commontest.EvalBool(ctx, `
			(() => {
				const headers = document.querySelectorAll('.stock-placeholder .panel-header');
				return Array.from(headers).some(h => h.textContent.includes('`+title+`'));
			})()
		`)
		if err != nil {
			t.Fatalf("error checking placeholder %s: %v", title, err)
		}
		if !found {
			t.Errorf("placeholder section %q not found", title)
		}
	}
}

func TestStockPageNoJSErrors(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	errs := newJSErrorCollector(ctx)
	err := loginAndNavigate(ctx, serverURL()+"/stock/BHP")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	// Wait for Alpine to initialise and consume SSR data
	_ = chromedp.Run(ctx, chromedp.Sleep(2*time.Second))

	takeScreenshot(t, ctx, "stock", "no-js-errors.png")

	if jsErrs := errs.Errors(); len(jsErrs) > 0 {
		t.Errorf("JS errors on stock page:\n  %s", strings.Join(jsErrs, "\n  "))
	}
}

func TestStockPageTickerFromDashboard(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	// Wait for Alpine to render holdings
	_ = chromedp.Run(ctx, chromedp.Sleep(2*time.Second))

	takeScreenshot(t, ctx, "stock", "dashboard-before-click.png")

	// Check if holdings table has ticker links
	hasTickerLinks, err := commontest.EvalBool(ctx, `
		document.querySelector('.tool-table tbody .tool-name a') !== null
	`)
	if err != nil {
		t.Fatalf("error checking ticker links: %v", err)
	}
	if !hasTickerLinks {
		t.Skip("no ticker links in holdings table (no holdings data available)")
	}

	// Get the first ticker link href and text
	var tickerHref, tickerText string
	err = chromedp.Run(ctx, chromedp.Evaluate(`
		(() => {
			const a = document.querySelector('.tool-table tbody .tool-name a');
			return a ? a.getAttribute('href') : '';
		})()
	`, &tickerHref))
	if err != nil {
		t.Fatalf("error getting ticker href: %v", err)
	}

	err = chromedp.Run(ctx, chromedp.Evaluate(`
		(() => {
			const a = document.querySelector('.tool-table tbody .tool-name a');
			return a ? a.textContent.trim() : '';
		})()
	`, &tickerText))
	if err != nil {
		t.Fatalf("error getting ticker text: %v", err)
	}

	if !strings.HasPrefix(tickerHref, "/stock/") {
		t.Fatalf("ticker link href %q does not start with /stock/", tickerHref)
	}

	// Click the first ticker link
	err = chromedp.Run(ctx,
		chromedp.Click(".tool-table tbody .tool-name a", chromedp.ByQuery),
	)
	if err != nil {
		t.Fatalf("error clicking ticker link: %v", err)
	}

	// Wait for navigation
	_ = chromedp.Run(ctx, chromedp.Sleep(2*time.Second))

	takeScreenshot(t, ctx, "stock", "navigated-from-dashboard.png")

	// Verify we're on the stock page
	var currentURL string
	err = chromedp.Run(ctx, chromedp.Location(&currentURL))
	if err != nil {
		t.Fatalf("error getting URL: %v", err)
	}
	if !strings.Contains(currentURL, "/stock/") {
		t.Errorf("expected URL to contain /stock/, got: %s", currentURL)
	}

	// Verify the stock page rendered with the correct ticker
	headingVisible, err := isVisible(ctx, ".stock-ticker-heading")
	if err != nil {
		t.Fatalf("error checking ticker heading: %v", err)
	}
	if !headingVisible {
		t.Error("stock ticker heading not visible after navigation from dashboard")
	}
}

func TestStockPageAlpineInit(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/stock/BHP")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	takeScreenshot(t, ctx, "stock", "alpine-init.png")

	alpineReady, err := commontest.EvalBool(ctx, `typeof Alpine !== 'undefined'`)
	if err != nil {
		t.Fatalf("error evaluating Alpine check: %v", err)
	}
	if !alpineReady {
		t.Fatal("Alpine.js not initialised on stock page")
	}
}

func TestStockPageSSR_VireDataCleared(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/stock/BHP")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	// Wait for Alpine init to consume SSR data
	_ = chromedp.Run(ctx, chromedp.Sleep(2*time.Second))

	takeScreenshot(t, ctx, "stock", "ssr-data-cleared.png")

	cleared, err := commontest.EvalBool(ctx, `window.__VIRE_DATA__ === null`)
	if err != nil {
		t.Fatalf("error checking __VIRE_DATA__: %v", err)
	}
	if !cleared {
		t.Error("window.__VIRE_DATA__ should be null after Alpine init consumes it")
	}
}

func TestStockPageHoldingSection(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/stock/BHP")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	_ = chromedp.Run(ctx, chromedp.Sleep(1*time.Second))

	takeScreenshot(t, ctx, "stock", "holding-section.png")

	// Check for either holding panel or info banner (depending on whether BHP is held)
	holdingVisible, _ := isVisible(ctx, ".panel-headed:not(.stock-placeholder)")
	infoBannerVisible, _ := isVisible(ctx, ".info-banner")

	if !holdingVisible && !infoBannerVisible {
		t.Error("neither PORTFOLIO HOLDING panel nor info banner is visible")
	}

	if holdingVisible {
		// Verify PORTFOLIO HOLDING header
		headerCorrect, err := commontest.EvalBool(ctx, `
			(() => {
				const headers = document.querySelectorAll('.panel-headed:not(.stock-placeholder) .panel-header');
				return Array.from(headers).some(h => h.textContent.includes('PORTFOLIO HOLDING'));
			})()
		`)
		if err != nil {
			t.Fatalf("error checking holding header: %v", err)
		}
		if !headerCorrect {
			t.Error("PORTFOLIO HOLDING panel header not found")
		}

		// Verify detail grid has fields
		fieldCount, err := elementCount(ctx, ".stock-detail-grid .stock-detail-field")
		if err != nil {
			t.Fatalf("error counting detail fields: %v", err)
		}
		if fieldCount < 5 {
			t.Errorf("stock detail field count = %d, want >= 5 (NAME, VALUE, WEIGHT, RETURN $, RETURN %%)", fieldCount)
		}
	}
}

func TestStockPageRedirectsEmpty(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/stock/")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	_ = chromedp.Run(ctx, chromedp.Sleep(1*time.Second))

	takeScreenshot(t, ctx, "stock", "redirect-empty.png")

	var currentURL string
	err = chromedp.Run(ctx, chromedp.Location(&currentURL))
	if err != nil {
		t.Fatalf("error getting URL: %v", err)
	}

	// Empty ticker should redirect to dashboard
	if strings.HasSuffix(currentURL, "/stock/") {
		t.Error("empty ticker should redirect away from /stock/")
	}
}

func TestStockPageUnauthenticated(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	var currentURL string
	err := chromedp.Run(ctx,
		chromedp.Navigate(serverURL()+"/stock/BHP"),
		chromedp.Sleep(2*time.Second),
		chromedp.Location(&currentURL),
	)
	if err != nil {
		t.Fatalf("navigation failed: %v", err)
	}

	takeScreenshot(t, ctx, "stock", "redirect-unauth.png")

	if strings.Contains(currentURL, "/stock/") {
		t.Errorf("unauthenticated user should be redirected away from /stock/, got: %s", currentURL)
	}
}
