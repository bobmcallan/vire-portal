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

	// Verify STRATEGY ALIGNMENT remains as the only placeholder
	found, err := commontest.EvalBool(ctx, `
		(() => {
			const headers = document.querySelectorAll('.stock-placeholder .panel-header');
			return Array.from(headers).some(h => h.textContent.includes('STRATEGY ALIGNMENT'));
		})()
	`)
	if err != nil {
		t.Fatalf("error checking placeholder: %v", err)
	}
	if !found {
		t.Errorf("placeholder section %q not found", "STRATEGY ALIGNMENT")
	}

	// Verify TRADE HISTORY is no longer a placeholder
	tradeHistoryIsPlaceholder, err := commontest.EvalBool(ctx, `
		(() => {
			const headers = document.querySelectorAll('.stock-placeholder .panel-header');
			return Array.from(headers).some(h => h.textContent.includes('TRADE HISTORY'));
		})()
	`)
	if err != nil {
		t.Fatalf("error checking trade history placeholder: %v", err)
	}
	if tradeHistoryIsPlaceholder {
		t.Error("TRADE HISTORY should no longer be a placeholder section")
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

func TestStockPriceChartSection(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/stock/BHP")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	_ = chromedp.Run(ctx, chromedp.Sleep(2*time.Second))

	takeScreenshot(t, ctx, "stock", "price-chart-section.png")

	// Check for the STOCK PRICE TREND section (may be hidden if no candle data)
	sectionExists, err := commontest.EvalBool(ctx, `
		(() => {
			const headers = document.querySelectorAll('.panel-header');
			return Array.from(headers).some(h => h.textContent.includes('STOCK PRICE TREND'));
		})()
	`)
	if err != nil {
		t.Fatalf("error checking price chart section: %v", err)
	}
	if !sectionExists {
		t.Skip("STOCK PRICE TREND section not found (no candle data available)")
	}

	// Verify the canvas element exists
	canvasExists, err := commontest.EvalBool(ctx, `!!document.getElementById('stock-price-chart')`)
	if err != nil {
		t.Fatalf("error checking chart canvas: %v", err)
	}
	if !canvasExists {
		t.Error("stock-price-chart canvas not found")
	}

	// Verify SMA toggle controls exist
	toggleCount, err := elementCount(ctx, ".growth-chart-controls .chart-toggle-label")
	if err != nil {
		t.Fatalf("error counting SMA toggles: %v", err)
	}
	if toggleCount < 3 {
		t.Errorf("SMA toggle count = %d, want >= 3 (SMA 20, SMA 50, SMA 200)", toggleCount)
	}
}

func TestStockFilingsSection(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/stock/BHP")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	_ = chromedp.Run(ctx, chromedp.Sleep(2*time.Second))

	takeScreenshot(t, ctx, "stock", "filings-section.png")

	// Check for the FILINGS TIMELINE section
	sectionExists, err := commontest.EvalBool(ctx, `
		(() => {
			const headers = document.querySelectorAll('.panel-header');
			return Array.from(headers).some(h => h.textContent.includes('FILINGS TIMELINE'));
		})()
	`)
	if err != nil {
		t.Fatalf("error checking filings section: %v", err)
	}
	if !sectionExists {
		t.Skip("FILINGS TIMELINE section not found (no filing data available)")
	}

	// Verify table headers
	headersCorrect, err := commontest.EvalBool(ctx, `
		(() => {
			const sections = document.querySelectorAll('.panel-headed');
			for (const sec of sections) {
				const header = sec.querySelector('.panel-header');
				if (header && header.textContent.includes('FILINGS TIMELINE')) {
					const ths = sec.querySelectorAll('.tool-table thead th');
					if (ths.length >= 3) {
						return ths[0].textContent.includes('Date') &&
							ths[1].textContent.includes('Headline') &&
							ths[2].textContent.includes('Type');
					}
				}
			}
			return false;
		})()
	`)
	if err != nil {
		t.Fatalf("error checking filings table headers: %v", err)
	}
	if !headersCorrect {
		t.Error("filings table headers do not match expected: Date, Headline, Type")
	}

	// Verify filing rows have clickable elements
	filingRowCount, err := elementCount(ctx, ".filing-row")
	if err != nil {
		t.Fatalf("error counting filing rows: %v", err)
	}
	if filingRowCount == 0 {
		t.Skip("no filing rows available in test environment")
	}
}

func TestStockNewsRemoved(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/stock/BHP")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	_ = chromedp.Run(ctx, chromedp.Sleep(2*time.Second))

	takeScreenshot(t, ctx, "stock", "news-removed.png")

	// Verify NEWS & SENTIMENT section is completely gone from the page
	newsExists, err := commontest.EvalBool(ctx, `
		(() => {
			const headers = document.querySelectorAll('.panel-header');
			return Array.from(headers).some(h => h.textContent.includes('NEWS') && h.textContent.includes('SENTIMENT'));
		})()
	`)
	if err != nil {
		t.Fatalf("error checking news section: %v", err)
	}
	if newsExists {
		t.Error("NEWS & SENTIMENT section should have been removed from the stock page")
	}
}

func TestStockWalkChartSection(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/stock/BHP")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	_ = chromedp.Run(ctx, chromedp.Sleep(2*time.Second))

	takeScreenshot(t, ctx, "stock", "walk-chart-section.png")

	// Check for the POSITION WALK section
	sectionExists, err := commontest.EvalBool(ctx, `
		(() => {
			const headers = document.querySelectorAll('.panel-header');
			return Array.from(headers).some(h => h.textContent.includes('POSITION WALK'));
		})()
	`)
	if err != nil {
		t.Fatalf("error checking walk chart section: %v", err)
	}
	if !sectionExists {
		t.Skip("POSITION WALK section not found (no position timeline data available)")
	}

	// Verify the walkChart canvas element exists
	canvasExists, err := commontest.EvalBool(ctx, `!!document.getElementById('walkChart')`)
	if err != nil {
		t.Fatalf("error checking walk chart canvas: %v", err)
	}
	if !canvasExists {
		t.Error("walkChart canvas not found")
	}

	// Verify the walk chart container structure
	scrollExists, err := isVisible(ctx, ".walk-chart-scroll")
	if err != nil {
		t.Fatalf("error checking walk chart scroll container: %v", err)
	}
	if !scrollExists {
		t.Error("walk-chart-scroll container not visible")
	}
}

func TestStockTradeHistorySection(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/stock/BHP")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	_ = chromedp.Run(ctx, chromedp.Sleep(2*time.Second))

	takeScreenshot(t, ctx, "stock", "trade-history-section.png")

	// Check for the TRADE HISTORY section (visible only when trades exist)
	sectionExists, err := commontest.EvalBool(ctx, `
		(() => {
			const headers = document.querySelectorAll('.panel-header');
			return Array.from(headers).some(h => h.textContent.trim() === 'TRADE HISTORY');
		})()
	`)
	if err != nil {
		t.Fatalf("error checking trade history section: %v", err)
	}
	if !sectionExists {
		t.Skip("TRADE HISTORY section not found (no trade data available)")
	}

	// Verify trade summary bar with Avg Cost, Breakeven, Units
	summaryExists, err := isVisible(ctx, ".trade-summary")
	if err != nil {
		t.Fatalf("error checking trade summary: %v", err)
	}
	if !summaryExists {
		t.Error("trade summary bar not visible")
	}

	// Verify table headers: Date, Type, Units, Price, Fees, Value
	headersCorrect, err := commontest.EvalBool(ctx, `
		(() => {
			const sections = document.querySelectorAll('.panel-headed');
			for (const sec of sections) {
				const header = sec.querySelector('.panel-header');
				if (header && header.textContent.trim() === 'TRADE HISTORY') {
					const ths = sec.querySelectorAll('.tool-table thead th');
					if (ths.length >= 6) {
						return ths[0].textContent.includes('Date') &&
							ths[1].textContent.includes('Type') &&
							ths[2].textContent.includes('Units') &&
							ths[3].textContent.includes('Price') &&
							ths[4].textContent.includes('Fees') &&
							ths[5].textContent.includes('Value');
					}
				}
			}
			return false;
		})()
	`)
	if err != nil {
		t.Fatalf("error checking trade table headers: %v", err)
	}
	if !headersCorrect {
		t.Error("trade history table headers do not match expected: Date, Type, Units, Price, Fees, Value")
	}
}

func TestStockGainBreakdown(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/stock/BHP")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	_ = chromedp.Run(ctx, chromedp.Sleep(2*time.Second))

	takeScreenshot(t, ctx, "stock", "gain-breakdown.png")

	// Verify CAPITAL RETURN label exists in the holding section
	capitalReturnExists, err := commontest.EvalBool(ctx, `
		(() => {
			const labels = document.querySelectorAll('.stock-detail-grid .stock-detail-field .label');
			return Array.from(labels).some(l => l.textContent.includes('CAPITAL RETURN'));
		})()
	`)
	if err != nil {
		t.Fatalf("error checking capital return label: %v", err)
	}
	if !capitalReturnExists {
		t.Skip("CAPITAL RETURN label not found (stock may not be held in portfolio)")
	}

	// Verify gain breakdown text with realized/unrealized is present
	breakdownExists, err := commontest.EvalBool(ctx, `
		(() => {
			const breakdowns = document.querySelectorAll('.gain-breakdown');
			return Array.from(breakdowns).some(b => b.textContent.includes('realized') && b.textContent.includes('unrealized'));
		})()
	`)
	if err != nil {
		t.Fatalf("error checking gain breakdown: %v", err)
	}
	if !breakdownExists {
		t.Error("gain breakdown with realized/unrealized text not found")
	}
}

func TestStockCompanyOverview(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/stock/BHP")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	_ = chromedp.Run(ctx, chromedp.Sleep(2*time.Second))

	takeScreenshot(t, ctx, "stock", "company-overview.png")

	// Check for the COMPANY OVERVIEW section
	sectionExists, err := commontest.EvalBool(ctx, `
		(() => {
			const headers = document.querySelectorAll('.panel-header');
			return Array.from(headers).some(h => h.textContent.includes('COMPANY OVERVIEW'));
		})()
	`)
	if err != nil {
		t.Fatalf("error checking company overview section: %v", err)
	}
	if !sectionExists {
		t.Skip("COMPANY OVERVIEW section not found (no fundamentals/timeline data available)")
	}

	// Check for fundamentals grid if present
	fundamentalsExists, err := commontest.EvalBool(ctx, `!!document.querySelector('.fundamentals-grid')`)
	if err != nil {
		t.Fatalf("error checking fundamentals grid: %v", err)
	}

	// Check for company events if present
	eventsExists, err := commontest.EvalBool(ctx, `!!document.querySelector('.company-event')`)
	if err != nil {
		t.Fatalf("error checking company events: %v", err)
	}

	if !fundamentalsExists && !eventsExists {
		t.Skip("COMPANY OVERVIEW section visible but no fundamentals/events data in test environment")
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
