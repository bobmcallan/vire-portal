package tests

import (
	"strings"
	"testing"
	"time"

	commontest "github.com/bobmcallan/vire-portal/tests/common"
	"github.com/chromedp/chromedp"
)

func TestStock(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	errs := newJSErrorCollector(ctx)

	err := loginAndNavigate(ctx, serverURL()+"/stock/BHP")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	// Wait for Alpine to initialise and consume SSR data
	_ = chromedp.Run(ctx, chromedp.Sleep(2*time.Second))

	t.Run("NoJSErrors", func(t *testing.T) {
		takeScreenshot(t, ctx, "stock", "no-js-errors.png")

		if jsErrs := errs.Errors(); len(jsErrs) > 0 {
			t.Errorf("JS errors on stock page:\n  %s", strings.Join(jsErrs, "\n  "))
		}
	})

	t.Run("PageLoad", func(t *testing.T) {
		takeScreenshot(t, ctx, "stock", "page-load.png")

		visible, err := isVisible(ctx, ".page")
		if err != nil {
			t.Fatalf("error checking stock page visibility: %v", err)
		}
		if !visible {
			t.Fatal("stock .page not visible after login")
		}

		headingVisible, err := isVisible(ctx, ".stock-ticker-heading")
		if err != nil {
			t.Fatalf("error checking ticker heading: %v", err)
		}
		if !headingVisible {
			t.Fatal("stock ticker heading (.stock-ticker-heading) not visible")
		}

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

		backLink, err := commontest.Exists(ctx, `.stock-back-link a[href="/dashboard"]`)
		if err != nil {
			t.Fatalf("error checking back link: %v", err)
		}
		if !backLink {
			t.Error("back link to /dashboard not found")
		}

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
	})

	t.Run("AlpineInit", func(t *testing.T) {
		takeScreenshot(t, ctx, "stock", "alpine-init.png")

		alpineReady, err := commontest.EvalBool(ctx, `typeof Alpine !== 'undefined'`)
		if err != nil {
			t.Fatalf("error evaluating Alpine check: %v", err)
		}
		if !alpineReady {
			t.Fatal("Alpine.js not initialised on stock page")
		}
	})

	t.Run("SSR_VireDataCleared", func(t *testing.T) {
		takeScreenshot(t, ctx, "stock", "ssr-data-cleared.png")

		cleared, err := commontest.EvalBool(ctx, `window.__VIRE_DATA__ === null`)
		if err != nil {
			t.Fatalf("error checking __VIRE_DATA__: %v", err)
		}
		if !cleared {
			t.Error("window.__VIRE_DATA__ should be null after Alpine init consumes it")
		}
	})

	t.Run("HoldingSection", func(t *testing.T) {
		takeScreenshot(t, ctx, "stock", "holding-section.png")

		holdingVisible, _ := isVisible(ctx, ".panel-headed:not(.stock-placeholder)")
		infoBannerVisible, _ := isVisible(ctx, ".info-banner")

		if !holdingVisible && !infoBannerVisible {
			t.Error("neither PORTFOLIO HOLDING panel nor info banner is visible")
		}

		if holdingVisible {
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

			fieldCount, err := elementCount(ctx, ".stock-detail-grid .stock-detail-field")
			if err != nil {
				t.Fatalf("error counting detail fields: %v", err)
			}
			if fieldCount < 5 {
				t.Errorf("stock detail field count = %d, want >= 5 (NAME, VALUE, WEIGHT, RETURN $, RETURN %%)", fieldCount)
			}
		}
	})

	t.Run("PriceChartRemoved", func(t *testing.T) {
		takeScreenshot(t, ctx, "stock", "price-chart-removed.png")

		sectionExists, err := commontest.EvalBool(ctx, `
			(() => {
				const headers = document.querySelectorAll('.panel-header');
				return Array.from(headers).some(h => h.textContent.includes('STOCK PRICE TREND'));
			})()
		`)
		if err != nil {
			t.Fatalf("error checking price chart section: %v", err)
		}
		if sectionExists {
			t.Error("STOCK PRICE TREND section should have been removed from the stock page")
		}

		canvasExists, err := commontest.EvalBool(ctx, `!!document.getElementById('stock-price-chart')`)
		if err != nil {
			t.Fatalf("error checking chart canvas: %v", err)
		}
		if canvasExists {
			t.Error("stock-price-chart canvas should have been removed")
		}
	})

	t.Run("FilingsSectionRemoved", func(t *testing.T) {
		takeScreenshot(t, ctx, "stock", "filings-removed.png")

		sectionExists, err := commontest.EvalBool(ctx, `
			(() => {
				const headers = document.querySelectorAll('.panel-header');
				return Array.from(headers).some(h => h.textContent.includes('FILINGS TIMELINE'));
			})()
		`)
		if err != nil {
			t.Fatalf("error checking filings section: %v", err)
		}
		if sectionExists {
			t.Error("FILINGS TIMELINE section should have been removed from the stock page")
		}
	})

	t.Run("NewsRemoved", func(t *testing.T) {
		takeScreenshot(t, ctx, "stock", "news-removed.png")

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
	})

	t.Run("WalkChartSection", func(t *testing.T) {
		takeScreenshot(t, ctx, "stock", "walk-chart-section.png")

		sectionExists, err := commontest.EvalBool(ctx, `
			(() => {
				const headers = document.querySelectorAll('.panel-header');
				return Array.from(headers).some(h => h.textContent.includes('POSITION P&L'));
			})()
		`)
		if err != nil {
			t.Fatalf("error checking walk chart section: %v", err)
		}
		if !sectionExists {
			t.Skip("POSITION P&L section not found (no position timeline data available)")
		}

		canvasExists, err := commontest.EvalBool(ctx, `!!document.getElementById('walkChart')`)
		if err != nil {
			t.Fatalf("error checking walk chart canvas: %v", err)
		}
		if !canvasExists {
			t.Error("walkChart canvas not found")
		}

		scrollExists, err := isVisible(ctx, ".walk-chart-scroll")
		if err != nil {
			t.Fatalf("error checking walk chart scroll container: %v", err)
		}
		if !scrollExists {
			t.Error("walk-chart-scroll container not visible")
		}
	})

	t.Run("TradeHistorySection", func(t *testing.T) {
		takeScreenshot(t, ctx, "stock", "trade-history-section.png")

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

		summaryExists, err := isVisible(ctx, ".trade-summary")
		if err != nil {
			t.Fatalf("error checking trade summary: %v", err)
		}
		if !summaryExists {
			t.Error("trade summary bar not visible")
		}

		headersCorrect, err := commontest.EvalBool(ctx, `
			(() => {
				const sections = document.querySelectorAll('.panel-headed');
				for (const sec of sections) {
					const header = sec.querySelector('.panel-header');
					if (header && header.textContent.trim() === 'TRADE HISTORY') {
						const ths = sec.querySelectorAll('.tool-table thead th');
						if (ths.length >= 7) {
							return ths[0].textContent.includes('Date') &&
								ths[1].textContent.includes('Type') &&
								ths[2].textContent.includes('Units') &&
								ths[3].textContent.includes('Price') &&
								ths[4].textContent.includes('Fees') &&
								ths[5].textContent.includes('Value') &&
								ths[6].textContent.includes('Realised P&L');
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
			t.Error("trade history table headers do not match expected: Date, Type, Units, Price, Fees, Value, Realised P&L")
		}
	})

	t.Run("GainBreakdown", func(t *testing.T) {
		takeScreenshot(t, ctx, "stock", "gain-breakdown.png")

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
	})

	t.Run("CompanyOverview", func(t *testing.T) {
		takeScreenshot(t, ctx, "stock", "company-overview.png")

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

		fundamentalsExists, err := commontest.EvalBool(ctx, `!!document.querySelector('.fundamentals-grid')`)
		if err != nil {
			t.Fatalf("error checking fundamentals grid: %v", err)
		}

		eventsExists, err := commontest.EvalBool(ctx, `!!document.querySelector('.company-event')`)
		if err != nil {
			t.Fatalf("error checking company events: %v", err)
		}

		if !fundamentalsExists && !eventsExists {
			t.Skip("COMPANY OVERVIEW section visible but no fundamentals/events data in test environment")
		}
	})
}

// TestStockPageUnauthenticated checks redirect for unauthenticated users. Needs its own browser (no login).
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

// TestStockPageTickerFromDashboard navigates from /dashboard, so it needs its own browser session.
func TestStockPageTickerFromDashboard(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	_ = chromedp.Run(ctx, chromedp.Sleep(2*time.Second))

	takeScreenshot(t, ctx, "stock", "dashboard-before-click.png")

	hasTickerLinks, err := commontest.EvalBool(ctx, `
		document.querySelector('.tool-table tbody .tool-name a') !== null
	`)
	if err != nil {
		t.Fatalf("error checking ticker links: %v", err)
	}
	if !hasTickerLinks {
		t.Skip("no ticker links in holdings table (no holdings data available)")
	}

	var tickerHref string
	err = chromedp.Run(ctx, chromedp.Evaluate(`
		(() => {
			const a = document.querySelector('.tool-table tbody .tool-name a');
			return a ? a.getAttribute('href') : '';
		})()
	`, &tickerHref))
	if err != nil {
		t.Fatalf("error getting ticker href: %v", err)
	}

	var tickerText string
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

	err = chromedp.Run(ctx,
		chromedp.Click(".tool-table tbody .tool-name a", chromedp.ByQuery),
	)
	if err != nil {
		t.Fatalf("error clicking ticker link: %v", err)
	}

	_ = chromedp.Run(ctx, chromedp.Sleep(2*time.Second))

	takeScreenshot(t, ctx, "stock", "navigated-from-dashboard.png")

	var currentURL string
	err = chromedp.Run(ctx, chromedp.Location(&currentURL))
	if err != nil {
		t.Fatalf("error getting URL: %v", err)
	}
	if !strings.Contains(currentURL, "/stock/") {
		t.Errorf("expected URL to contain /stock/, got: %s", currentURL)
	}

	headingVisible, err := isVisible(ctx, ".stock-ticker-heading")
	if err != nil {
		t.Fatalf("error checking ticker heading: %v", err)
	}
	if !headingVisible {
		t.Error("stock ticker heading not visible after navigation from dashboard")
	}
}

// TestStockPageRedirectsEmpty checks empty ticker redirect. Needs its own browser (different URL).
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

	if strings.HasSuffix(currentURL, "/stock/") {
		t.Error("empty ticker should redirect away from /stock/")
	}
}
