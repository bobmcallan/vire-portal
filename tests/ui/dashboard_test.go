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

	takeScreenshot(t, ctx, "dashboard", "show-closed-checkbox.png")

	count, err := elementCount(ctx, ".portfolio-filter-label input[type='checkbox']")
	if err != nil {
		t.Fatalf("error checking show-closed checkbox: %v", err)
	}
	if count < 1 {
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

	// Verify the first .portfolio-summary row has 5 items
	count, err := elementCount(ctx, ".portfolio-summary:not(.portfolio-summary-capital) .portfolio-summary-item")
	if err != nil {
		t.Fatalf("error counting summary items: %v", err)
	}
	if count != 5 {
		t.Errorf("portfolio summary item count = %d, want 5", count)
	}

	// Verify the 5 summary labels are "TOTAL VALUE", "NET EQUITY CAPITAL", "AVAILABLE CASH", "NET RETURN $", "NET RETURN %"
	labelsCorrect, err := commontest.EvalBool(ctx, `
		(() => {
			const row = document.querySelector('.portfolio-summary:not(.portfolio-summary-capital)');
			if (!row) return false;
			const labels = row.querySelectorAll('.portfolio-summary-item .label');
			if (labels.length !== 5) return false;
			const expected = ['TOTAL VALUE', 'NET EQUITY CAPITAL', 'AVAILABLE CASH', 'NET RETURN $', 'NET RETURN %'];
			for (let i = 0; i < 5; i++) {
				if (labels[i].textContent.trim() !== expected[i]) return false;
			}
			return true;
		})()
	`)
	if err != nil {
		t.Fatalf("error checking summary labels: %v", err)
	}
	if !labelsCorrect {
		t.Error("portfolio summary labels do not match expected: TOTAL VALUE, NET EQUITY CAPITAL, AVAILABLE CASH, NET RETURN $, NET RETURN %")
	}

	// If capital performance row exists, verify it also has 5 items
	capitalCount, err := elementCount(ctx, ".portfolio-summary-capital .portfolio-summary-item")
	if err != nil {
		t.Fatalf("error counting capital summary items: %v", err)
	}
	if capitalCount > 0 && capitalCount != 5 {
		t.Errorf("capital summary item count = %d, want 5", capitalCount)
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
			const items = document.querySelectorAll('.portfolio-summary-item .text-bold');
			if (items.length < 5) return false;
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

	// Verify gain values in summary have color classes applied
	summaryGainColored, err := commontest.EvalBool(ctx, `
		(() => {
			const items = document.querySelectorAll('.portfolio-summary-item .text-bold');
			// Items 2 and 3 are NET RETURN $ and NET RETURN % — should have gain class if non-zero
			let hasGainClass = false;
			for (let i = 2; i < items.length; i++) {
				if (items[i].classList.contains('gain-positive') || items[i].classList.contains('gain-negative')) {
					hasGainClass = true;
				}
			}
			return hasGainClass;
		})()
	`)
	if err != nil {
		t.Fatalf("error checking summary gain colors: %v", err)
	}
	if !summaryGainColored {
		t.Log("summary gain values have no color class (gains may be zero)")
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

	// 4. Verify gain colors in portfolio summary (if visible)
	summaryVisible, err := isVisible(ctx, ".portfolio-summary")
	if err != nil {
		t.Logf("warning: could not check summary visibility: %v", err)
	} else if summaryVisible {
		var summaryGainInfo string
		err = chromedp.Run(ctx, chromedp.Evaluate(`
			(() => {
				const items = document.querySelectorAll('.portfolio-summary-item .text-bold');
				if (items.length < 5) return 'items:' + items.length;
				// Items at index 3 and 4 are return $ and return % (after adding AVAILABLE CASH at index 2)
				const gainItems = [items[3], items[4]];
				const colored = gainItems.filter(i => i.classList.contains('gain-positive') || i.classList.contains('gain-negative'));
				const values = gainItems.map(i => i.textContent.trim());
				return 'values:[' + values.join(',') + '],colored:' + colored.length;
			})()
		`, &summaryGainInfo))
		if err != nil {
			t.Fatalf("error checking summary gain: %v", err)
		}
		t.Logf("summary gain info: %s", summaryGainInfo)
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

	// Wait for Alpine to render
	_ = chromedp.Run(ctx, chromedp.Sleep(1*time.Second))

	takeScreenshot(t, ctx, "dashboard", "capital-performance.png")

	// Check if capital performance row exists (only shown when capital data available)
	capitalVisible, err := isVisible(ctx, ".portfolio-summary-capital")
	if err != nil {
		t.Fatalf("error checking capital performance visibility: %v", err)
	}
	if !capitalVisible {
		t.Skip("capital performance row not visible (no capital data available)")
	}

	// Verify 5 capital summary items
	count, err := elementCount(ctx, ".portfolio-summary-capital .portfolio-summary-item")
	if err != nil {
		t.Fatalf("error counting capital summary items: %v", err)
	}
	if count != 5 {
		t.Errorf("capital summary item count = %d, want 5", count)
	}

	// Verify capital summary labels
	labelsCorrect, err := commontest.EvalBool(ctx, `
		(() => {
			const row = document.querySelector('.portfolio-summary-capital');
			if (!row) return false;
			const labels = row.querySelectorAll('.portfolio-summary-item .label');
			if (labels.length !== 5) return false;
			const expected = ['TOTAL DEPOSITED', 'CAPITAL GAIN $', 'CAPITAL GAIN %', 'SIMPLE RETURN %', 'ANNUALIZED %'];
			for (let i = 0; i < 5; i++) {
				if (labels[i].textContent.trim() !== expected[i]) return false;
			}
			return true;
		})()
	`)
	if err != nil {
		t.Fatalf("error checking capital labels: %v", err)
	}
	if !labelsCorrect {
		t.Error("capital summary labels do not match expected: TOTAL DEPOSITED, CAPITAL GAIN $, CAPITAL GAIN %, SIMPLE RETURN %, ANNUALIZED %")
	}

	// Verify capital gain values have color classes applied
	capitalGainColored, err := commontest.EvalBool(ctx, `
		(() => {
			const row = document.querySelector('.portfolio-summary-capital');
			if (!row) return false;
			const items = row.querySelectorAll('.portfolio-summary-item .text-bold');
			// Items 1-4 are CAPITAL GAIN $, CAPITAL GAIN %, SIMPLE RETURN %, ANNUALIZED %
			let hasGainClass = false;
			for (let i = 1; i < items.length; i++) {
				if (items[i].classList.contains('gain-positive') || items[i].classList.contains('gain-negative')) {
					hasGainClass = true;
				}
			}
			return hasGainClass;
		})()
	`)
	if err != nil {
		t.Fatalf("error checking capital gain colors: %v", err)
	}
	if !capitalGainColored {
		t.Log("capital gain values have no color class (gains may be zero)")
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

func TestDashboardIndicators(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	// Wait for Alpine to render + indicators fetch
	_ = chromedp.Run(ctx, chromedp.Sleep(2*time.Second))

	takeScreenshot(t, ctx, "dashboard", "indicators.png")

	// Check if indicators row exists (only shown when indicators data available)
	indicatorsVisible, err := isVisible(ctx, ".portfolio-indicators")
	if err != nil {
		t.Fatalf("error checking indicators visibility: %v", err)
	}
	if !indicatorsVisible {
		t.Skip("indicators row not visible (no indicator data available)")
	}

	// Verify indicator items exist
	count, err := elementCount(ctx, ".portfolio-indicators .indicator-item")
	if err != nil {
		t.Fatalf("error counting indicator items: %v", err)
	}
	if count < 2 {
		t.Errorf("indicator item count = %d, want >= 2", count)
	}

	// Verify TREND and RSI labels
	labelsCorrect, err := commontest.EvalBool(ctx, `
		(() => {
			const row = document.querySelector('.portfolio-indicators');
			if (!row) return false;
			const text = row.textContent;
			return text.includes('TREND:') && text.includes('RSI:');
		})()
	`)
	if err != nil {
		t.Fatalf("error checking indicator labels: %v", err)
	}
	if !labelsCorrect {
		t.Error("indicator row does not contain expected TREND: and RSI: labels")
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

	// Check if growth chart container is visible (depends on growth data being available)
	containerVisible, err := isVisible(ctx, ".growth-chart-container")
	if err != nil {
		t.Fatalf("error checking growth chart container visibility: %v", err)
	}
	if !containerVisible {
		t.Skip("growth chart container not visible (no growth data available from API)")
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

	// Verify the chart has 3 datasets (Portfolio Value, Cost Basis, Net Deposited)
	datasetCount, err := commontest.EvalBool(ctx, `
		(() => {
			const canvas = document.getElementById('growthChart');
			const chart = Chart.getChart(canvas);
			return chart && chart.data.datasets.length === 3;
		})()
	`)
	if err != nil {
		t.Fatalf("error checking chart datasets: %v", err)
	}
	if !datasetCount {
		t.Error("growth chart does not have exactly 3 datasets")
	}

	// Verify dataset labels
	labelsCorrect, err := commontest.EvalBool(ctx, `
		(() => {
			const canvas = document.getElementById('growthChart');
			const chart = Chart.getChart(canvas);
			if (!chart) return false;
			const labels = chart.data.datasets.map(d => d.label);
			return labels[0] === 'Portfolio Value' && labels[1] === 'Cost Basis' && labels[2] === 'Net Deposited';
		})()
	`)
	if err != nil {
		t.Fatalf("error checking dataset labels: %v", err)
	}
	if !labelsCorrect {
		t.Error("growth chart dataset labels do not match expected: Portfolio Value, Cost Basis, Net Deposited")
	}

	// Verify chart container has the correct styling (border, no border-radius)
	containerStyled, err := commontest.EvalBool(ctx, `
		(() => {
			const el = document.querySelector('.growth-chart-container');
			if (!el) return false;
			const style = getComputedStyle(el);
			return style.borderStyle !== 'none' && style.borderRadius === '0px';
		})()
	`)
	if err != nil {
		t.Fatalf("error checking chart container styles: %v", err)
	}
	if !containerStyled {
		t.Error("growth chart container does not have expected monochrome styling")
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
