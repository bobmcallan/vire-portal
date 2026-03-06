package tests

import (
	"strings"
	"testing"
	"time"

	commontest "github.com/bobmcallan/vire-portal/tests/common"
	"github.com/chromedp/chromedp"
)

func TestDashboardAuthLoad(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	takeScreenshot(t, ctx, "dashboard", "auth-load.png")

	visible, err := isVisible(ctx, ".page")
	if err != nil {
		t.Fatalf("error checking dashboard visibility: %v", err)
	}
	if !visible {
		t.Fatal("dashboard .page not visible after login")
	}
}

func TestDashboardNoJSErrors(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	errs := newJSErrorCollector(ctx)
	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	takeScreenshot(t, ctx, "dashboard", "no-js-errors.png")

	if jsErrs := errs.Errors(); len(jsErrs) > 0 {
		t.Errorf("JS errors on dashboard:\n  %s", strings.Join(jsErrs, "\n  "))
	}
}

func TestDashboardAlpineInit(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	takeScreenshot(t, ctx, "dashboard", "alpine-init.png")

	alpineReady, err := commontest.EvalBool(ctx, `typeof Alpine !== 'undefined'`)
	if err != nil {
		t.Fatalf("error evaluating Alpine check: %v", err)
	}
	if !alpineReady {
		t.Fatal("Alpine.js not initialised")
	}
}

func TestDashboardPortfolioDropdown(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	// Wait for Alpine to load portfolio data
	_ = chromedp.Run(ctx, chromedp.Sleep(500*time.Millisecond))

	takeScreenshot(t, ctx, "dashboard", "portfolio-dropdown.png")

	visible, err := isVisible(ctx, "select.portfolio-select")
	if err != nil {
		t.Fatalf("error checking portfolio dropdown visibility: %v", err)
	}
	if !visible {
		t.Fatal("portfolio dropdown (select.portfolio-select) not visible")
	}
}

func TestDashboardHoldingsTable(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	// Wait for Alpine to load holdings data
	_ = chromedp.Run(ctx, chromedp.Sleep(1*time.Second))

	takeScreenshot(t, ctx, "dashboard", "holdings-table.png")

	visible, err := isVisible(ctx, ".tool-table")
	if err != nil {
		t.Fatalf("error checking holdings table visibility: %v", err)
	}
	if !visible {
		t.Skip("holdings table not visible (no portfolio data available)")
	}

	// Alpine x-for templates render rows asynchronously; count may be 0 if
	// the MCP backend has no holdings for this portfolio — that's OK.
	count, err := elementCount(ctx, ".tool-table tbody tr")
	if err != nil {
		t.Fatalf("error counting table rows: %v", err)
	}
	t.Logf("holdings table rows: %d", count)

	// Verify holdings are sorted alphabetically by ticker
	if count >= 2 {
		sorted, err := commontest.EvalBool(ctx, `
			(() => {
				const rows = document.querySelectorAll('.tool-table tbody tr');
				const tickers = Array.from(rows).map(r => r.querySelector('.tool-name')?.textContent || '');
				for (let i = 1; i < tickers.length; i++) {
					if (tickers[i].localeCompare(tickers[i-1]) < 0) return false;
				}
				return true;
			})()
		`)
		if err != nil {
			t.Fatalf("error checking ticker sort order: %v", err)
		}
		if !sorted {
			t.Error("holdings table is not sorted alphabetically by ticker")
		}
	}
}

func TestDashboardShowClosedCheckbox(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	// Wait for Alpine to render
	_ = chromedp.Run(ctx, chromedp.Sleep(1*time.Second))

	takeScreenshot(t, ctx, "dashboard", "show-closed-before-click.png")

	// Check if the checkbox is actually visible (not just in DOM — it's inside x-show="holdings.length > 0")
	visible, err := commontest.EvalBool(ctx, `
		(() => {
			const el = document.querySelector('.portfolio-filter-label input[type="checkbox"]');
			if (!el) return false;
			const rect = el.getBoundingClientRect();
			return rect.width > 0 && rect.height > 0;
		})()
	`)
	if err != nil {
		t.Fatalf("error checking show-closed checkbox visibility: %v", err)
	}
	if !visible {
		t.Skip("show-closed checkbox not visible (no holdings data available)")
	}

	// Verify the label text
	labelOK, err := commontest.EvalBool(ctx, `
		(() => {
			const label = document.querySelector('.portfolio-filter-label');
			return label && label.textContent.includes('Show closed positions');
		})()
	`)
	if err != nil {
		t.Fatalf("error checking filter label text: %v", err)
	}
	if !labelOK {
		t.Error("show-closed checkbox label does not contain expected text")
	}

	// Record row count before clicking
	rowsBefore, err := elementCount(ctx, ".tool-table tbody tr")
	if err != nil {
		t.Fatalf("error counting rows before click: %v", err)
	}
	t.Logf("holdings rows before click: %d", rowsBefore)

	// Install fetch observer to detect the include_closed API call
	err = chromedp.Run(ctx, chromedp.Evaluate(`
		window.__closedFetchCalled = false;
		const _origFetch = window.fetch;
		window.fetch = function(url, opts) {
			if (typeof url === 'string' && url.includes('include_closed=true')) {
				window.__closedFetchCalled = true;
			}
			return _origFetch.apply(this, arguments);
		};
	`, nil))
	if err != nil {
		t.Fatalf("error installing fetch observer: %v", err)
	}

	// Click the "Show closed positions" checkbox
	err = chromedp.Run(ctx,
		chromedp.Click(".portfolio-filter-label input[type='checkbox']", chromedp.ByQuery),
	)
	if err != nil {
		t.Fatalf("error clicking show-closed checkbox: %v", err)
	}

	// Wait for the fetch to complete
	_ = chromedp.Run(ctx, chromedp.Sleep(2*time.Second))

	takeScreenshot(t, ctx, "dashboard", "show-closed-after-click.png")

	// Verify the fetch with include_closed=true was called
	fetchCalled, err := commontest.EvalBool(ctx, `window.__closedFetchCalled === true`)
	if err != nil {
		t.Fatalf("error checking fetch observer: %v", err)
	}
	if !fetchCalled {
		t.Error("clicking 'Show closed positions' did not trigger fetch with include_closed=true")
	}

	// Verify checkbox is now checked
	checked, err := commontest.EvalBool(ctx, `
		document.querySelector('.portfolio-filter-label input[type="checkbox"]').checked
	`)
	if err != nil {
		t.Fatalf("error checking checkbox state: %v", err)
	}
	if !checked {
		t.Error("checkbox should be checked after click")
	}

	// Verify closedLoading is false (fetch completed)
	loadingDone, err := commontest.EvalBool(ctx, `
		(() => {
			const comp = document.querySelector('[x-data]').__x.$data;
			return comp.closedLoading === false;
		})()
	`)
	if err != nil {
		t.Logf("could not check closedLoading state: %v", err)
	} else if !loadingDone {
		t.Error("closedLoading should be false after fetch completes")
	}

	// Log row count after clicking (may include closed positions now)
	rowsAfter, err := elementCount(ctx, ".tool-table tbody tr")
	if err != nil {
		t.Fatalf("error counting rows after click: %v", err)
	}
	t.Logf("holdings rows after click: %d (before: %d)", rowsAfter, rowsBefore)
	if rowsAfter > rowsBefore {
		t.Logf("closed positions added: %d new rows", rowsAfter-rowsBefore)
	}
}

func TestDashboardPortfolioSummary(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	// Wait for Alpine to render
	_ = chromedp.Run(ctx, chromedp.Sleep(1*time.Second))

	takeScreenshot(t, ctx, "dashboard", "portfolio-summary.png")

	// Check if portfolio summary is visible (only shows when filteredHoldings > 0)
	visible, err := isVisible(ctx, ".portfolio-summary")
	if err != nil {
		t.Fatalf("error checking portfolio summary visibility: %v", err)
	}
	if !visible {
		t.Skip("portfolio summary not visible (no holdings data available)")
	}

	// Verify the first .portfolio-summary row has exactly 3 items (PORTFOLIO VALUE, AVAILABLE CASH, GROSS CONTRIBUTIONS)
	count, err := elementCount(ctx, ".portfolio-summary:not(.portfolio-summary-performance) .portfolio-summary-item")
	if err != nil {
		t.Fatalf("error counting summary items: %v", err)
	}
	if count != 3 {
		t.Errorf("portfolio summary item count = %d, want 3", count)
	}

	// Verify the composition row labels
	labelsCorrect, err := commontest.EvalBool(ctx, `
		(() => {
			const row = document.querySelector('.portfolio-summary:not(.portfolio-summary-performance)');
			if (!row) return false;
			const labels = row.querySelectorAll('.portfolio-summary-item .label');
			if (labels.length !== 3) return false;
			const expected = ['PORTFOLIO VALUE', 'AVAILABLE CASH', 'GROSS CONTRIBUTIONS'];
			for (let i = 0; i < 3; i++) {
				if (!labels[i].textContent.includes(expected[i])) return false;
			}
			return true;
		})()
	`)
	if err != nil {
		t.Fatalf("error checking summary labels: %v", err)
	}
	if !labelsCorrect {
		t.Error("composition row labels should be PORTFOLIO VALUE, AVAILABLE CASH, GROSS CONTRIBUTIONS")
	}

	// Verify summary spans full content width (justify-content: space-between)
	spansWidth, err := commontest.EvalBool(ctx, `
		(() => {
			const el = document.querySelector('.portfolio-summary');
			if (!el) return false;
			const style = getComputedStyle(el);
			return style.justifyContent === 'space-between' && style.width !== '0px';
		})()
	`)
	if err != nil {
		t.Fatalf("error checking summary layout: %v", err)
	}
	if !spansWidth {
		t.Error("portfolio summary does not span full width (missing justify-content: space-between)")
	}

	// Verify summary values are populated (not empty or just "-")
	valuesPopulated, err := commontest.EvalBool(ctx, `
		(() => {
			const items = document.querySelectorAll('.portfolio-summary:not(.portfolio-summary-performance) .portfolio-summary-item .text-bold');
			if (items.length === 0) return false;
			for (const item of items) {
				const text = item.textContent.trim();
				if (!text || text === '') return false;
			}
			return true;
		})()
	`)
	if err != nil {
		t.Fatalf("error checking summary values: %v", err)
	}
	if !valuesPopulated {
		t.Error("portfolio summary values are empty")
	}

	// Verify Row 2 NET RETURN uses daily-only badges with "today" suffix (not D/W/M)
	perfRow, err := isVisible(ctx, ".portfolio-summary-performance")
	if err != nil {
		t.Fatalf("error checking performance row visibility: %v", err)
	}
	if perfRow {
		// Check .portfolio-change-today class exists (replaces .portfolio-changes in Row 2)
		todayBadgeExists, err := commontest.EvalBool(ctx, `
			document.querySelector('.portfolio-summary-performance .portfolio-change-today') !== null
		`)
		if err != nil {
			t.Fatalf("error checking portfolio-change-today class: %v", err)
		}
		if !todayBadgeExists {
			t.Error("Row 2 should use .portfolio-change-today class instead of .portfolio-changes for daily-only badges")
		}

		// Check that daily badges contain "today" suffix text
		todaySuffix, err := commontest.EvalBool(ctx, `
			(() => {
				const badges = document.querySelectorAll('.portfolio-summary-performance .portfolio-change-today span');
				if (badges.length === 0) return true; // no data, skip
				for (const b of badges) {
					const text = b.textContent.trim();
					if (text && !text.includes('today')) return false;
				}
				return true;
			})()
		`)
		if err != nil {
			t.Fatalf("error checking today suffix: %v", err)
		}
		if !todaySuffix {
			t.Error("Row 2 daily badges should contain 'today' suffix")
		}

		// Verify Row 2 does NOT have D/W/M multi-badge .portfolio-changes
		noMultiBadge, err := commontest.EvalBool(ctx, `
			document.querySelector('.portfolio-summary-performance .portfolio-changes') === null
		`)
		if err != nil {
			t.Fatalf("error checking absence of multi-badge: %v", err)
		}
		if !noMultiBadge {
			t.Error("Row 2 should not have .portfolio-changes (D/W/M) — should use .portfolio-change-today instead")
		}
	}
}

func TestDashboardColumnAlignment(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	// Wait for Alpine to render
	_ = chromedp.Run(ctx, chromedp.Sleep(1*time.Second))

	takeScreenshot(t, ctx, "dashboard", "column-alignment.png")

	// Check if holdings table is visible
	visible, err := isVisible(ctx, ".tool-table")
	if err != nil {
		t.Fatalf("error checking holdings table visibility: %v", err)
	}
	if !visible {
		t.Skip("holdings table not visible (no portfolio data available)")
	}

	// Verify column headers with .text-right class have computed text-align: right
	aligned, err := commontest.EvalBool(ctx, `
		(() => {
			const ths = document.querySelectorAll('.tool-table th.text-right');
			if (ths.length === 0) return false;
			for (const th of ths) {
				const style = getComputedStyle(th);
				if (style.textAlign !== 'right') return false;
			}
			return true;
		})()
	`)
	if err != nil {
		t.Fatalf("error checking column header alignment: %v", err)
	}
	if !aligned {
		t.Error("column headers with .text-right class do not have computed text-align: right")
	}
}

func TestDashboardGainColors(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	// Wait for Alpine to render
	_ = chromedp.Run(ctx, chromedp.Sleep(1*time.Second))

	takeScreenshot(t, ctx, "dashboard", "gain-colors.png")

	// 1. Verify gain CSS rules exist in stylesheets
	var cssResult string
	err = chromedp.Run(ctx, chromedp.Evaluate(`
		(() => {
			let foundPos = false, foundNeg = false;
			for (const s of document.styleSheets) {
				try {
					for (const r of s.cssRules) {
						if (r.selectorText === '.gain-positive' && r.style.color) foundPos = true;
						if (r.selectorText === '.gain-negative' && r.style.color) foundNeg = true;
					}
				} catch(e) { /* cross-origin */ }
			}
			if (foundPos && foundNeg) return 'both';
			if (foundPos) return 'positive-only';
			if (foundNeg) return 'negative-only';
			return 'none';
		})()
	`, &cssResult))
	if err != nil {
		t.Fatalf("error checking gain CSS rules: %v", err)
	}
	if cssResult != "both" {
		t.Errorf("gain CSS rules incomplete: found %s, want both .gain-positive and .gain-negative", cssResult)
	}

	// 2. Verify Gain% column exists in table header
	visible, err := isVisible(ctx, ".tool-table")
	if err != nil {
		t.Fatalf("error checking holdings table visibility: %v", err)
	}
	if !visible {
		t.Skip("holdings table not visible (no portfolio data available)")
	}

	gainHeadersFound, err := commontest.EvalBool(ctx, `
		(() => {
			const ths = document.querySelectorAll('.tool-table th');
			const headers = Array.from(ths).map(th => th.textContent.trim());
			return headers.includes('Return $') && headers.includes('Return %');
		})()
	`)
	if err != nil {
		t.Fatalf("error checking Return column headers: %v", err)
	}
	if !gainHeadersFound {
		t.Error("Return $ and Return % column headers not found in holdings table")
	}

	// 3. Verify return values are displayed in table rows (not empty) — last TWO cells are return columns
	var gainInfo string
	err = chromedp.Run(ctx, chromedp.Evaluate(`
		(() => {
			const rows = document.querySelectorAll('.tool-table tbody tr');
			if (rows.length === 0) return 'no-rows';
			// Last two cells in each row are Return $ and Return %
			const gainCells = [];
			for (const r of rows) {
				const cells = r.querySelectorAll('td');
				if (cells.length >= 2) {
					gainCells.push(cells[cells.length - 2]);
					gainCells.push(cells[cells.length - 1]);
				}
			}
			const empty = gainCells.filter(c => !c.textContent.trim() || c.textContent.trim() === '');
			if (empty.length > 0) return 'empty:' + empty.length;
			const withColor = gainCells.filter(c => c.classList.contains('gain-positive') || c.classList.contains('gain-negative'));
			const neutral = gainCells.filter(c => !c.classList.contains('gain-positive') && !c.classList.contains('gain-negative'));
			return 'rows:' + rows.length + ',gainCells:' + gainCells.length + ',colored:' + withColor.length + ',neutral:' + neutral.length;
		})()
	`, &gainInfo))
	if err != nil {
		t.Fatalf("error checking table gain values: %v", err)
	}
	t.Logf("table gain info: %s", gainInfo)
	if strings.HasPrefix(gainInfo, "empty:") {
		t.Errorf("return columns have empty cells: %s", gainInfo)
	}

	// 4. Verify gain colors in portfolio summary row 1 (if visible)
	row1Visible, err := isVisible(ctx, ".portfolio-summary:not(.portfolio-summary-cash):not(.portfolio-summary-equity)")
	if err != nil {
		t.Logf("warning: could not check row 1 visibility: %v", err)
	} else if row1Visible {
		var row1GainInfo string
		err = chromedp.Run(ctx, chromedp.Evaluate(`
			(() => {
				const items = document.querySelectorAll('.portfolio-summary:not(.portfolio-summary-performance) .portfolio-summary-item .text-bold');
				if (items.length < 1) return 'items:0';
				// If more than 1 item, items 1+ are capital return values
				if (items.length > 1) {
					const gainItems = Array.from(items).slice(1);
					const colored = gainItems.filter(i => i.classList.contains('gain-positive') || i.classList.contains('gain-negative'));
					const values = gainItems.map(i => i.textContent.trim());
					return 'values:[' + values.join(',') + '],colored:' + colored.length;
				}
				return 'items:1';
			})()
		`, &row1GainInfo))
		if err != nil {
			t.Fatalf("error checking row 1 gain: %v", err)
		}
		t.Logf("row 1 gain info: %s", row1GainInfo)
	}
}

func TestDashboardDefaultCheckbox(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	// Wait for Alpine to render
	_ = chromedp.Run(ctx, chromedp.Sleep(500*time.Millisecond))

	takeScreenshot(t, ctx, "dashboard", "default-checkbox.png")

	count, err := elementCount(ctx, ".portfolio-default-label input[type='checkbox']")
	if err != nil {
		t.Fatalf("error checking default checkbox: %v", err)
	}
	if count < 1 {
		t.Skip("default checkbox not visible (no portfolio selected)")
	}
}

func TestDashboardNoTemplateMarkers(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	takeScreenshot(t, ctx, "dashboard", "no-template-markers.png")

	var bodyText string
	err = chromedp.Run(ctx, chromedp.Evaluate(`document.body.innerText`, &bodyText))
	if err != nil {
		t.Fatalf("error getting body text: %v", err)
	}

	badMarkers := []string{"{{.", "<no value>", "{{template", "{{if", "{{range}"}
	for _, marker := range badMarkers {
		if strings.Contains(bodyText, marker) {
			t.Fatalf("raw template marker %q found in page body", marker)
		}
	}
}

func TestDashboardCapitalPerformance(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	_ = chromedp.Run(ctx, chromedp.Sleep(1*time.Second))

	takeScreenshot(t, ctx, "dashboard", "capital-performance.png")

	// ===== Row 1: Composition =====
	compositionVisible, err := isVisible(ctx, ".portfolio-summary:not(.portfolio-summary-performance)")
	if err != nil {
		t.Fatalf("error checking composition row visibility: %v", err)
	}
	if !compositionVisible {
		t.Skip("composition row not visible (no holdings data available)")
	}

	compositionCount, err := elementCount(ctx, ".portfolio-summary:not(.portfolio-summary-performance) .portfolio-summary-item")
	if err != nil {
		t.Fatalf("error counting composition summary items: %v", err)
	}
	if compositionCount != 3 {
		t.Errorf("composition summary item count = %d, want 3", compositionCount)
	}

	compositionLabelsCorrect, err := commontest.EvalBool(ctx, `
		(() => {
			const row = document.querySelector('.portfolio-summary:not(.portfolio-summary-performance)');
			if (!row) return false;
			const labels = row.querySelectorAll('.portfolio-summary-item .label');
			if (labels.length !== 3) return false;
			const expected = ['PORTFOLIO VALUE', 'AVAILABLE CASH', 'GROSS CONTRIBUTIONS'];
			for (let i = 0; i < 3; i++) {
				if (!labels[i].textContent.includes(expected[i])) return false;
			}
			return true;
		})()
	`)
	if err != nil {
		t.Fatalf("error checking composition labels: %v", err)
	}
	if !compositionLabelsCorrect {
		t.Error("composition row labels do not match expected: PORTFOLIO VALUE, AVAILABLE CASH, GROSS CONTRIBUTIONS")
	}

	// ===== Row 2: Performance =====
	perfVisible, err := isVisible(ctx, ".portfolio-summary-performance")
	if err != nil {
		t.Fatalf("error checking performance row visibility: %v", err)
	}
	if !perfVisible {
		t.Skip("performance row not visible (no holdings data available)")
	}

	perfCount, err := elementCount(ctx, ".portfolio-summary-performance .portfolio-summary-item")
	if err != nil {
		t.Fatalf("error counting performance summary items: %v", err)
	}
	if perfCount != 2 && perfCount != 3 {
		t.Errorf("performance summary item count = %d, want 2 or 3", perfCount)
	}

	perfLabelsCorrect, err := commontest.EvalBool(ctx, `
		(() => {
			const row = document.querySelector('.portfolio-summary-performance');
			if (!row) return false;
			const labels = row.querySelectorAll('.portfolio-summary-item .label');
			if (labels.length < 2) return false;
			if (!labels[0].textContent.includes('NET RETURN $')) return false;
			if (!labels[1].textContent.includes('NET RETURN %')) return false;
			if (labels.length === 3) {
				if (!labels[2].textContent.includes('DIVIDENDS')) return false;
			}
			return true;
		})()
	`)
	if err != nil {
		t.Fatalf("error checking performance labels: %v", err)
	}
	if !perfLabelsCorrect {
		t.Error("performance row labels do not match expected: NET RETURN $, NET RETURN %, and optionally DIVIDENDS")
	}

	returnGainColored, err := commontest.EvalBool(ctx, `
		(() => {
			const row = document.querySelector('.portfolio-summary-performance');
			if (!row) return false;
			const items = row.querySelectorAll('.portfolio-summary-item .text-bold');
			let hasGainClass = false;
			for (let i = 0; i < Math.min(items.length, 2); i++) {
				if (items[i].classList.contains('gain-positive') || items[i].classList.contains('gain-negative')) {
					hasGainClass = true;
				}
			}
			return hasGainClass;
		})()
	`)
	if err != nil {
		t.Fatalf("error checking return gain colors: %v", err)
	}
	if !returnGainColored {
		t.Log("return values have no color class (gains may be zero)")
	}
}

func TestDashboardRefreshButton(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	// Wait for Alpine to render
	_ = chromedp.Run(ctx, chromedp.Sleep(500*time.Millisecond))

	takeScreenshot(t, ctx, "dashboard", "refresh-button.png")

	// Check that portfolio header is visible
	headerVisible, err := isVisible(ctx, ".portfolio-header")
	if err != nil {
		t.Fatalf("error checking portfolio header visibility: %v", err)
	}
	if !headerVisible {
		t.Skip("portfolio header not visible (no portfolios available)")
	}

	// Verify refresh button exists in portfolio header
	refreshExists, err := commontest.EvalBool(ctx, `
		(() => {
			const header = document.querySelector('.portfolio-header');
			if (!header) return false;
			const btn = header.querySelector('button');
			return btn && btn.textContent.includes('REFRESH');
		})()
	`)
	if err != nil {
		t.Fatalf("error checking refresh button: %v", err)
	}
	if !refreshExists {
		t.Error("refresh button not found in portfolio header")
	}

	// Verify refresh button is right-aligned (margin-left: auto)
	rightAligned, err := commontest.EvalBool(ctx, `
		(() => {
			const header = document.querySelector('.portfolio-header');
			if (!header) return false;
			const btn = header.querySelector('button');
			if (!btn) return false;
			const style = getComputedStyle(btn);
			return style.marginLeft === 'auto';
		})()
	`)
	if err != nil {
		t.Fatalf("error checking refresh button alignment: %v", err)
	}
	if !rightAligned {
		t.Error("refresh button should have margin-left: auto (right-aligned in flex container)")
	}
}

func TestDashboardGrowthChart(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	// Wait for Alpine to render + growth data fetch
	_ = chromedp.Run(ctx, chromedp.Sleep(2*time.Second))

	takeScreenshot(t, ctx, "dashboard", "growth-chart.png")

	// Check if Chart.js is loaded
	chartJSLoaded, err := commontest.EvalBool(ctx, `typeof Chart !== 'undefined'`)
	if err != nil {
		t.Fatalf("error checking Chart.js availability: %v", err)
	}
	if !chartJSLoaded {
		t.Fatal("Chart.js not loaded")
	}

	// Check if growth chart canvas exists in the DOM
	canvasExists, err := commontest.EvalBool(ctx, `document.getElementById('growthChart') !== null`)
	if err != nil {
		t.Fatalf("error checking canvas existence: %v", err)
	}
	if !canvasExists {
		t.Fatal("growth chart canvas element not found in DOM")
	}

	// Check if growth chart section is visible (depends on growth data being available)
	sectionVisible, err := isVisible(ctx, ".growth-chart-section")
	if err != nil {
		t.Fatalf("error checking growth chart section visibility: %v", err)
	}
	if !sectionVisible {
		t.Skip("growth chart section not visible (no growth data available from API)")
	}

	// Verify Chart.js instance was created on the canvas
	chartCreated, err := commontest.EvalBool(ctx, `
		(() => {
			const canvas = document.getElementById('growthChart');
			return canvas && Chart.getChart(canvas) !== undefined;
		})()
	`)
	if err != nil {
		t.Fatalf("error checking Chart.js instance: %v", err)
	}
	if !chartCreated {
		t.Error("Chart.js instance not created on growthChart canvas")
	}

	// Verify the chart has 3 or 4 datasets (3 base + optional Gross Contributions)
	datasetCount, err := commontest.EvalBool(ctx, `
		(() => {
			const canvas = document.getElementById('growthChart');
			const chart = Chart.getChart(canvas);
			return chart && (chart.data.datasets.length === 3 || chart.data.datasets.length === 4);
		})()
	`)
	if err != nil {
		t.Fatalf("error checking chart datasets: %v", err)
	}
	if !datasetCount {
		t.Error("growth chart does not have 3 or 4 datasets")
	}

	// Verify dataset labels (first 3 required, 4th optional Gross Contributions)
	labelsCorrect, err := commontest.EvalBool(ctx, `
		(() => {
			const canvas = document.getElementById('growthChart');
			const chart = Chart.getChart(canvas);
			if (!chart) return false;
			const labels = chart.data.datasets.map(d => d.label);
			const base = labels[0] === 'Portfolio Value' && labels[1] === 'Equity Value' && labels[2] === 'Net Deposited';
			if (labels.length === 4) return base && labels[3] === 'Gross Contributions';
			return base;
		})()
	`)
	if err != nil {
		t.Fatalf("error checking dataset labels: %v", err)
	}
	if !labelsCorrect {
		t.Error("growth chart dataset labels do not match expected: Portfolio Value, Equity Value, Net Deposited [, Gross Contributions]")
	}

	// Verify .growth-chart-section wrapper exists and has expected styling (border, padding)
	sectionStyled, err := commontest.EvalBool(ctx, `
		(() => {
			const el = document.querySelector('.growth-chart-section');
			if (!el) return false;
			const style = getComputedStyle(el);
			return style.borderStyle !== 'none' && style.borderRadius === '0px';
		})()
	`)
	if err != nil {
		t.Fatalf("error checking chart section styles: %v", err)
	}
	if !sectionStyled {
		t.Error("growth-chart-section wrapper does not have expected styling")
	}

	// Verify .chart-toggle-label exists with "Show breakdown" text
	toggleExists, err := commontest.EvalBool(ctx, `
		(() => {
			const label = document.querySelector('.chart-toggle-label');
			if (!label) return false;
			return label.textContent.includes('Show breakdown');
		})()
	`)
	if err != nil {
		t.Fatalf("error checking chart toggle label: %v", err)
	}
	if !toggleExists {
		t.Error("chart-toggle-label not found or does not contain 'Show breakdown' text")
	}

	// Verify chart initially has only 1 visible dataset (breakdown datasets hidden)
	initialDatasets, err := commontest.EvalBool(ctx, `
		(() => {
			const canvas = document.getElementById('growthChart');
			const chart = Chart.getChart(canvas);
			if (!chart) return false;
			const visible = chart.data.datasets.filter((d, i) => !chart.getDatasetMeta(i).hidden);
			return visible.length === 1;
		})()
	`)
	if err != nil {
		t.Fatalf("error checking initial dataset visibility: %v", err)
	}
	if !initialDatasets {
		t.Error("chart should initially show only 1 visible dataset (Portfolio Value); breakdown should be hidden")
	}
}

// TestDashboardDesign checks all CSS/design constraints in a single test
func TestDashboardDesign(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	takeScreenshot(t, ctx, "dashboard", "design.png")

	// Check font-family
	var fontFamily string
	err = chromedp.Run(ctx, chromedp.Evaluate(`getComputedStyle(document.body).fontFamily`, &fontFamily))
	if err != nil {
		t.Fatalf("error getting font-family: %v", err)
	}
	ff := strings.ToLower(fontFamily)
	if !strings.Contains(ff, "ibm plex mono") && !strings.Contains(ff, "monospace") {
		t.Errorf("font-family = %q, want IBM Plex Mono / monospace", fontFamily)
	}

	// Check border-radius
	var borderRadiusViolators string
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const bad = [];
				document.querySelectorAll('*').forEach(el => {
					if (el.classList.contains('status-dot')) return;
					const r = getComputedStyle(el).borderRadius;
					if (r && r !== '0px') {
						const cls = el.className ? '.' + el.className.split(' ')[0] : '';
						bad.push(el.tagName.toLowerCase() + cls + ' (' + r + ')');
					}
				});
				return bad.slice(0, 5).join(', ');
			})()
		`, &borderRadiusViolators),
	)
	if err != nil {
		t.Fatalf("error checking border-radius: %v", err)
	}
	if borderRadiusViolators != "" {
		t.Errorf("border-radius found on: %s", borderRadiusViolators)
	}

	// Check box-shadow
	var boxShadowViolators string
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const bad = [];
				document.querySelectorAll('*').forEach(el => {
					const s = getComputedStyle(el).boxShadow;
					if (s && s !== 'none') {
						const cls = el.className ? '.' + el.className.split(' ')[0] : '';
						bad.push(el.tagName.toLowerCase() + cls);
					}
				});
				return bad.slice(0, 5).join(', ');
			})()
		`, &boxShadowViolators),
	)
	if err != nil {
		t.Fatalf("error checking box-shadow: %v", err)
	}
	if boxShadowViolators != "" {
		t.Errorf("box-shadow found on: %s", boxShadowViolators)
	}
}

func TestDashboardChangesRow(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	// Wait for Alpine to render
	_ = chromedp.Run(ctx, chromedp.Sleep(1*time.Second))

	takeScreenshot(t, ctx, "dashboard", "changes-row.png")

	// Check if portfolio summary is visible (only shows when filteredHoldings > 0)
	visible, err := isVisible(ctx, ".portfolio-summary")
	if err != nil {
		t.Fatalf("error checking portfolio summary visibility: %v", err)
	}
	if !visible {
		t.Skip("portfolio summary not visible (no holdings data available)")
	}

	// Verify .portfolio-changes element exists within the first portfolio-summary-item
	hasChanges, err := commontest.EvalBool(ctx, `
		(() => {
			const firstItem = document.querySelector('.portfolio-summary:not(.portfolio-summary-performance) .portfolio-summary-item');
			if (!firstItem) return false;
			const changes = firstItem.querySelector('.portfolio-changes');
			return changes !== null;
		})()
	`)
	if err != nil {
		t.Fatalf("error checking changes element: %v", err)
	}
	if !hasChanges {
		t.Skip("portfolio changes element not visible (no changes data available)")
	}

	// Verify D/W/M badges are present
	badgeCount, err := elementCount(ctx, ".portfolio-changes span")
	if err != nil {
		t.Fatalf("error counting change badges: %v", err)
	}
	// Should have 3 badges: D, W, M (when hasChanges is true)
	if badgeCount < 1 {
		t.Error("portfolio changes badges not found")
	}

	// Verify D/W/M labels exist
	badgesCorrect, err := commontest.EvalBool(ctx, `
		(() => {
			const changes = document.querySelector('.portfolio-changes');
			if (!changes) return false;
			const text = changes.textContent.trim();
			// Should contain D:, W:, M: labels (and percentages)
			return text.includes('D:') && text.includes('W:') && text.includes('M:');
		})()
	`)
	if err != nil {
		t.Fatalf("error checking change badge labels: %v", err)
	}
	if !badgesCorrect {
		t.Error("portfolio changes badges do not contain expected D:/W:/M: labels")
	}

	// Verify change-up/change-down/change-neutral classes are applied
	classesApplied, err := commontest.EvalBool(ctx, `
		(() => {
			const badges = document.querySelectorAll('.portfolio-changes span');
			if (badges.length === 0) return false;
			for (const badge of badges) {
				const classes = badge.className;
				const hasColorClass = classes.includes('change-up') || classes.includes('change-down') || classes.includes('change-neutral');
				if (!hasColorClass) return false;
			}
			return true;
		})()
	`)
	if err != nil {
		t.Fatalf("error checking change badge classes: %v", err)
	}
	if !classesApplied {
		t.Error("portfolio change badges do not have color classes (change-up, change-down, or change-neutral)")
	}
}

func TestDashboardReturnDollarChanges(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	_ = chromedp.Run(ctx, chromedp.Sleep(1*time.Second))

	takeScreenshot(t, ctx, "dashboard", "return-dollar-changes.png")

	visible, err := isVisible(ctx, ".portfolio-summary-performance")
	if err != nil {
		t.Fatalf("error checking performance row visibility: %v", err)
	}
	if !visible {
		t.Skip("performance row not visible (no holdings data available)")
	}

	hasChanges, err := commontest.EvalBool(ctx, `
		(() => {
			const firstItem = document.querySelector('.portfolio-summary-performance .portfolio-summary-item');
			if (!firstItem) return false;
			const changes = firstItem.querySelector('.portfolio-changes');
			return changes !== null;
		})()
	`)
	if err != nil {
		t.Fatalf("error checking return dollar changes element: %v", err)
	}
	if !hasChanges {
		t.Skip("return dollar changes element not visible (no return changes data available)")
	}

	badgesCorrect, err := commontest.EvalBool(ctx, `
		(() => {
			const perfRow = document.querySelector('.portfolio-summary-performance');
			if (!perfRow) return false;
			const firstItem = perfRow.querySelector('.portfolio-summary-item');
			if (!firstItem) return false;
			const changes = firstItem.querySelector('.portfolio-changes');
			if (!changes) return false;
			const text = changes.textContent.trim();
			return text.includes('D:') && text.includes('W:') && text.includes('M:');
		})()
	`)
	if err != nil {
		t.Fatalf("error checking return dollar change badges: %v", err)
	}
	if !badgesCorrect {
		t.Error("return dollar changes badges do not contain expected D:/W:/M: labels")
	}

	classesApplied, err := commontest.EvalBool(ctx, `
		(() => {
			const perfRow = document.querySelector('.portfolio-summary-performance');
			if (!perfRow) return false;
			const firstItem = perfRow.querySelector('.portfolio-summary-item');
			if (!firstItem) return false;
			const badges = firstItem.querySelectorAll('.portfolio-changes span');
			if (badges.length === 0) return false;
			for (const badge of badges) {
				const hasColorClass = badge.className.includes('change-up') || badge.className.includes('change-down') || badge.className.includes('change-neutral');
				if (!hasColorClass) return false;
			}
			return true;
		})()
	`)
	if err != nil {
		t.Fatalf("error checking return dollar change classes: %v", err)
	}
	if !classesApplied {
		t.Error("return dollar change badges do not have color classes (change-up, change-down, or change-neutral)")
	}
}

func TestDashboardReturnPctChanges(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	_ = chromedp.Run(ctx, chromedp.Sleep(1*time.Second))

	takeScreenshot(t, ctx, "dashboard", "return-pct-changes.png")

	visible, err := isVisible(ctx, ".portfolio-summary-performance")
	if err != nil {
		t.Fatalf("error checking performance row visibility: %v", err)
	}
	if !visible {
		t.Skip("performance row not visible (no holdings data available)")
	}

	hasChanges, err := commontest.EvalBool(ctx, `
		(() => {
			const items = document.querySelectorAll('.portfolio-summary-performance .portfolio-summary-item');
			if (items.length < 2) return false;
			const changes = items[1].querySelector('.portfolio-changes');
			return changes !== null;
		})()
	`)
	if err != nil {
		t.Fatalf("error checking return pct changes element: %v", err)
	}
	if !hasChanges {
		t.Skip("return pct changes element not visible (no return pct changes data available)")
	}

	badgesCorrect, err := commontest.EvalBool(ctx, `
		(() => {
			const items = document.querySelectorAll('.portfolio-summary-performance .portfolio-summary-item');
			if (items.length < 2) return false;
			const changes = items[1].querySelector('.portfolio-changes');
			if (!changes) return false;
			const text = changes.textContent.trim();
			return text.includes('D:') && text.includes('W:') && text.includes('M:');
		})()
	`)
	if err != nil {
		t.Fatalf("error checking return pct change badges: %v", err)
	}
	if !badgesCorrect {
		t.Error("return pct changes badges do not contain expected D:/W:/M: labels")
	}

	classesApplied, err := commontest.EvalBool(ctx, `
		(() => {
			const items = document.querySelectorAll('.portfolio-summary-performance .portfolio-summary-item');
			if (items.length < 2) return false;
			const badges = items[1].querySelectorAll('.portfolio-changes span');
			if (badges.length === 0) return false;
			for (const badge of badges) {
				const hasColorClass = badge.className.includes('change-up') || badge.className.includes('change-down') || badge.className.includes('change-neutral');
				if (!hasColorClass) return false;
			}
			return true;
		})()
	`)
	if err != nil {
		t.Fatalf("error checking return pct change classes: %v", err)
	}
	if !classesApplied {
		t.Error("return pct change badges do not have color classes (change-up, change-down, or change-neutral)")
	}
}

func TestDashboardGlossaryTooltips(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	_ = chromedp.Run(ctx, chromedp.Sleep(2*time.Second))

	takeScreenshot(t, ctx, "dashboard", "glossary-tooltips.png")

	visible, err := isVisible(ctx, ".portfolio-summary")
	if err != nil {
		t.Fatalf("error checking portfolio summary visibility: %v", err)
	}
	if !visible {
		t.Skip("portfolio summary not visible (no holdings data available)")
	}

	// Verify label-info tooltip icons exist
	count, err := elementCount(ctx, ".portfolio-summary .label-info")
	if err != nil {
		t.Fatalf("error counting label-info elements: %v", err)
	}
	if count < 1 {
		t.Error("no glossary tooltip icons (.label-info) found in portfolio summary")
	}
	t.Logf("found %d glossary tooltip icons", count)

	// Verify at least some tooltips have data-tooltip attributes populated
	tooltipsPopulated, err := commontest.EvalBool(ctx, `
		(() => {
			const icons = document.querySelectorAll('.portfolio-summary .label-info');
			if (icons.length === 0) return false;
			let populated = 0;
			for (const icon of icons) {
				const tooltip = icon.getAttribute('data-tooltip');
				if (tooltip && tooltip.length > 0) populated++;
			}
			return populated > 0;
		})()
	`)
	if err != nil {
		t.Fatalf("error checking tooltip content: %v", err)
	}
	if !tooltipsPopulated {
		t.Log("glossary tooltips not yet populated (glossary API may be unavailable)")
	}

	// Verify tooltip icons contain "i" text
	iconTextCorrect, err := commontest.EvalBool(ctx, `
		(() => {
			const icons = document.querySelectorAll('.portfolio-summary .label-info');
			if (icons.length === 0) return false;
			for (const icon of icons) {
				if (icon.textContent.trim() !== 'i') return false;
			}
			return true;
		})()
	`)
	if err != nil {
		t.Fatalf("error checking icon text: %v", err)
	}
	if !iconTextCorrect {
		t.Error("label-info icons do not all contain 'i' text")
	}
}

func TestDashboardBreadthBar(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	// Wait for Alpine to render and breadth to compute
	_ = chromedp.Run(ctx, chromedp.Sleep(2*time.Second))

	takeScreenshot(t, ctx, "dashboard", "breadth-bar.png")

	// Check if breadth bar section is visible (only shows when hasBreadth is true)
	visible, err := isVisible(ctx, ".breadth-bar-section")
	if err != nil {
		t.Fatalf("error checking breadth bar visibility: %v", err)
	}
	if !visible {
		t.Skip("breadth bar not visible (no holdings data or breadth not computed)")
	}

	// Verify breadth summary row exists with trend label
	summaryVisible, err := isVisible(ctx, ".breadth-summary-row")
	if err != nil {
		t.Fatalf("error checking breadth summary row: %v", err)
	}
	if !summaryVisible {
		t.Error("breadth summary row not visible")
	}

	// Verify trend label has a color class
	trendColored, err := commontest.EvalBool(ctx, `
		(() => {
			const el = document.querySelector('.breadth-trend');
			if (!el) return false;
			const cls = el.className;
			return cls.includes('change-up') || cls.includes('change-down') || cls.includes('change-neutral');
		})()
	`)
	if err != nil {
		t.Fatalf("error checking trend color class: %v", err)
	}
	if !trendColored {
		t.Error("breadth trend label does not have a color class (change-up, change-down, or change-neutral)")
	}

	// Verify the bar has at least 1 per-ticker segment
	segmentCount, err := elementCount(ctx, ".breadth-bar .breadth-segment")
	if err != nil {
		t.Fatalf("error counting breadth segments: %v", err)
	}
	if segmentCount < 1 {
		t.Errorf("breadth bar segment count = %d, want >= 1", segmentCount)
	}

	// Verify all segments have one of the expected status classes
	segmentsCorrect, err := commontest.EvalBool(ctx, `
		(() => {
			const segs = document.querySelectorAll('.breadth-bar .breadth-segment');
			if (segs.length === 0) return false;
			return Array.from(segs).every(s =>
				s.classList.contains('breadth-falling') ||
				s.classList.contains('breadth-flat') ||
				s.classList.contains('breadth-rising')
			);
		})()
	`)
	if err != nil {
		t.Fatalf("error checking breadth segment classes: %v", err)
	}
	if !segmentsCorrect {
		t.Error("breadth bar segments must each have one of: breadth-falling, breadth-flat, breadth-rising")
	}

	// Verify segments have title attributes containing ticker names
	segmentTitles, err := commontest.EvalBool(ctx, `
		(() => {
			const segs = document.querySelectorAll('.breadth-bar .breadth-segment');
			if (segs.length === 0) return false;
			return Array.from(segs).every(s => s.title && s.title.length > 0);
		})()
	`)
	if err != nil {
		t.Fatalf("error checking breadth segment titles: %v", err)
	}
	if !segmentTitles {
		t.Error("breadth bar segments must have title attributes with ticker info")
	}

	// Verify counts row has Rising/Flat/Falling labels
	countsCorrect, err := commontest.EvalBool(ctx, `
		(() => {
			const el = document.querySelector('.breadth-counts');
			if (!el) return false;
			const text = el.textContent;
			return text.includes('Rising') && text.includes('Flat') && text.includes('Falling');
		})()
	`)
	if err != nil {
		t.Fatalf("error checking breadth counts: %v", err)
	}
	if !countsCorrect {
		t.Error("breadth counts row does not contain Rising/Flat/Falling labels")
	}

	// Verify breadth bar section has expected styling (border, padding)
	styled, err := commontest.EvalBool(ctx, `
		(() => {
			const el = document.querySelector('.breadth-bar-section');
			if (!el) return false;
			const style = getComputedStyle(el);
			return style.borderStyle !== 'none' && style.padding !== '0px';
		})()
	`)
	if err != nil {
		t.Fatalf("error checking breadth bar styling: %v", err)
	}
	if !styled {
		t.Error("breadth bar section missing expected border/padding styling")
	}

	// Verify segment order: falling before flat before rising (per-ticker sort)
	segmentOrder, err := commontest.EvalBool(ctx, `
		(() => {
			const segments = document.querySelectorAll('.breadth-bar .breadth-segment');
			if (segments.length < 1) return false;
			const order = { 'breadth-falling': 0, 'breadth-flat': 1, 'breadth-rising': 2 };
			let lastOrder = -1;
			for (const s of segments) {
				let o = -1;
				if (s.classList.contains('breadth-falling')) o = 0;
				else if (s.classList.contains('breadth-flat')) o = 1;
				else if (s.classList.contains('breadth-rising')) o = 2;
				if (o < lastOrder) return false;
				lastOrder = o;
			}
			return true;
		})()
	`)
	if err != nil {
		t.Fatalf("error checking segment order: %v", err)
	}
	if !segmentOrder {
		t.Error("breadth bar segments should be ordered: falling, then flat, then rising")
	}

}

func TestDashboardHoldingTrendArrows(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	// Wait for Alpine to render holdings and breadth
	_ = chromedp.Run(ctx, chromedp.Sleep(2*time.Second))

	takeScreenshot(t, ctx, "dashboard", "holding-trend-arrows.png")

	// Check if holdings table is visible
	visible, err := isVisible(ctx, ".tool-table")
	if err != nil {
		t.Fatalf("error checking holdings table visibility: %v", err)
	}
	if !visible {
		t.Skip("holdings table not visible (no portfolio data)")
	}

	// Verify .holding-movement-row exists with .holding-movement-content td
	movementRowExists, err := commontest.EvalBool(ctx, `
		(() => {
			const row = document.querySelector('.holding-movement-row');
			if (!row) return false;
			return row.querySelector('.holding-movement-content') !== null;
		})()
	`)
	if err != nil {
		t.Fatalf("error checking holding movement rows: %v", err)
	}
	if !movementRowExists {
		t.Error(".holding-movement-row should exist with .holding-movement-content td")
	}
}

func TestDashboardLastSynced(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	// Wait for Alpine to render
	_ = chromedp.Run(ctx, chromedp.Sleep(1*time.Second))

	takeScreenshot(t, ctx, "dashboard", "last-synced.png")

	// Check if .portfolio-synced element exists
	visible, err := isVisible(ctx, ".portfolio-synced")
	if err != nil {
		t.Fatalf("error checking synced element visibility: %v", err)
	}
	if !visible {
		t.Skip("portfolio synced element not visible (no last_synced data available)")
	}

	// Verify "Synced" text is present
	syncedText, err := commontest.EvalBool(ctx, `
		(() => {
			const el = document.querySelector('.portfolio-synced');
			if (!el) return false;
			const text = el.textContent.trim();
			return text.includes('Synced');
		})()
	`)
	if err != nil {
		t.Fatalf("error checking synced text: %v", err)
	}
	if !syncedText {
		t.Error("portfolio synced element does not contain 'Synced' text")
	}

	// Verify date format (should contain day/month/year and time)
	dateFormatted, err := commontest.EvalBool(ctx, `
		(() => {
			const el = document.querySelector('.portfolio-synced');
			if (!el) return false;
			const text = el.textContent.trim();
			// Should match format like "Synced 3 Mar 2026 15:30"
			// Check for numbers (day/year) and month abbreviation
			return /\d+\s+\w+\s+\d{4}\s+\d{2}:\d{2}/.test(text);
		})()
	`)
	if err != nil {
		t.Fatalf("error checking date format: %v", err)
	}
	if !dateFormatted {
		t.Error("portfolio synced element does not contain properly formatted date/time")
	}

	// Verify element has text-muted class for styling
	hasMutedClass, err := commontest.EvalBool(ctx, `
		(() => {
			const el = document.querySelector('.portfolio-synced span');
			if (!el) return false;
			return el.className.includes('text-muted');
		})()
	`)
	if err != nil {
		t.Fatalf("error checking muted class: %v", err)
	}
	if !hasMutedClass {
		t.Error("portfolio synced span element does not have text-muted class")
	}

	// Verify synced element is right-aligned
	rightAligned, err := commontest.EvalBool(ctx, `
		(() => {
			const el = document.querySelector('.portfolio-synced');
			if (!el) return false;
			return getComputedStyle(el).textAlign === 'right';
		})()
	`)
	if err != nil {
		t.Fatalf("error checking synced alignment: %v", err)
	}
	if !rightAligned {
		t.Error("portfolio synced element should be right-aligned")
	}
}

func TestDashboardSSR_NoLoadingSpinner(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	takeScreenshot(t, ctx, "dashboard", "ssr-no-loading-spinner.png")

	// SSR pre-populates data, so "Loading portfolios..." should not be visible
	loadingHidden, err := commontest.EvalBool(ctx, `
		(() => {
			const els = document.querySelectorAll('[x-show="loading"]');
			for (const el of els) {
				if (el.offsetParent !== null && el.textContent.includes('Loading portfolios')) return false;
			}
			return true;
		})()
	`)
	if err != nil {
		t.Fatalf("error checking loading spinner: %v", err)
	}
	if !loadingHidden {
		t.Error("Loading portfolios spinner should not be visible with SSR")
	}

	// Portfolio selector should be immediately visible
	selectorVisible, err := isVisible(ctx, "select.portfolio-select")
	if err != nil {
		t.Fatalf("error checking portfolio selector: %v", err)
	}
	if !selectorVisible {
		t.Error("portfolio selector should be visible immediately with SSR")
	}
}

func TestDashboardSSR_DataPreRendered(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	takeScreenshot(t, ctx, "dashboard", "ssr-data-prerendered.png")

	// Holdings table should be visible without waiting for client-side fetch
	tableVisible, err := isVisible(ctx, ".tool-table")
	if err != nil {
		t.Fatalf("error checking holdings table: %v", err)
	}
	if !tableVisible {
		t.Skip("holdings table not visible (test account may have no holdings)")
	}

	// Alpine loading state should be false
	notLoading, err := commontest.EvalBool(ctx, `
		(() => {
			const comp = document.querySelector('[x-data]');
			if (!comp || !comp.__x) return false;
			return comp.__x.$data.loading === false;
		})()
	`)
	if err != nil {
		t.Fatalf("error checking Alpine loading state: %v", err)
	}
	if !notLoading {
		t.Skip("Alpine loading state not false (test account may lack portfolio data for SSR)")
	}
}

func TestDashboardSSR_VireDataCleared(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	// Wait for Alpine init to complete and clear SSR data
	_ = chromedp.Run(ctx, chromedp.Sleep(2*time.Second))

	takeScreenshot(t, ctx, "dashboard", "ssr-vire-data-cleared.png")

	// After Alpine init, window.__VIRE_DATA__ should be null (consumed by SSR path)
	// or still present with null portfolios (SSR path skipped, client-side fallback used)
	cleared, err := commontest.EvalBool(ctx, `
		window.__VIRE_DATA__ === null ||
		(window.__VIRE_DATA__ && window.__VIRE_DATA__.portfolios === null)
	`)
	if err != nil {
		t.Fatalf("error checking __VIRE_DATA__: %v", err)
	}
	if !cleared {
		t.Error("window.__VIRE_DATA__ should be null (consumed) or have null portfolios (no SSR data)")
	}
}
