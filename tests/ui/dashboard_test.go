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

	// Check if portfolio summary is visible (only shows when filteredHoldings > 0)
	visible, err := isVisible(ctx, ".portfolio-summary")
	if err != nil {
		t.Fatalf("error checking portfolio summary visibility: %v", err)
	}
	if !visible {
		t.Skip("portfolio summary not visible (no holdings data available)")
	}

	// Verify 3 summary items exist
	count, err := elementCount(ctx, ".portfolio-summary .portfolio-summary-item")
	if err != nil {
		t.Fatalf("error counting summary items: %v", err)
	}
	if count != 3 {
		t.Errorf("portfolio summary item count = %d, want 3", count)
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
			if (items.length < 3) return false;
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
			// Items 1 and 2 are TOTAL GAIN $ and TOTAL GAIN % — should have gain class if non-zero
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
		t.Fatalf("error checking summary gain colors: %v", err)
	}
	if !summaryGainColored {
		t.Log("summary gain values have no color class (gains may be zero)")
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

	gainHeaderFound, err := commontest.EvalBool(ctx, `
		(() => {
			const ths = document.querySelectorAll('.tool-table th');
			return Array.from(ths).some(th => th.textContent.includes('Gain'));
		})()
	`)
	if err != nil {
		t.Fatalf("error checking Gain column header: %v", err)
	}
	if !gainHeaderFound {
		t.Error("Gain% column header not found in holdings table")
	}

	// 3. Verify gain values are displayed in table rows (not empty)
	var gainInfo string
	err = chromedp.Run(ctx, chromedp.Evaluate(`
		(() => {
			const rows = document.querySelectorAll('.tool-table tbody tr');
			if (rows.length === 0) return 'no-rows';
			// Last cell in each row is the gain column
			const lastCells = Array.from(rows).map(r => {
				const cells = r.querySelectorAll('td');
				return cells[cells.length - 1];
			});
			const empty = lastCells.filter(c => !c.textContent.trim() || c.textContent.trim() === '');
			if (empty.length > 0) return 'empty:' + empty.length;
			const withColor = lastCells.filter(c => c.classList.contains('gain-positive') || c.classList.contains('gain-negative'));
			const neutral = lastCells.filter(c => !c.classList.contains('gain-positive') && !c.classList.contains('gain-negative'));
			return 'rows:' + rows.length + ',colored:' + withColor.length + ',neutral:' + neutral.length;
		})()
	`, &gainInfo))
	if err != nil {
		t.Fatalf("error checking table gain values: %v", err)
	}
	t.Logf("table gain info: %s", gainInfo)
	if strings.HasPrefix(gainInfo, "empty:") {
		t.Errorf("gain column has empty cells: %s", gainInfo)
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
				if (items.length < 3) return 'items:' + items.length;
				// Items at index 1 and 2 are gain $ and gain %
				const gainItems = [items[1], items[2]];
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

func TestDashboardStrategyEditor(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	// Wait for Alpine to render
	_ = chromedp.Run(ctx, chromedp.Sleep(500*time.Millisecond))

	visible, err := isVisible(ctx, "textarea.portfolio-editor")
	if err != nil {
		t.Fatalf("error checking strategy editor visibility: %v", err)
	}
	if !visible {
		t.Skip("strategy editor not visible (no portfolio selected)")
	}

	// Verify the STRATEGY panel header exists
	strategyFound, err := commontest.EvalBool(ctx, `
		(() => {
			const headers = document.querySelectorAll('.panel-header');
			return Array.from(headers).some(h => h.textContent.includes('STRATEGY'));
		})()
	`)
	if err != nil {
		t.Fatalf("error checking STRATEGY header: %v", err)
	}
	if !strategyFound {
		t.Fatal("STRATEGY panel header not found")
	}
}

func TestDashboardPlanEditor(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	// Wait for Alpine to render
	_ = chromedp.Run(ctx, chromedp.Sleep(500*time.Millisecond))

	// Check that there are at least 2 portfolio-editor textareas (strategy + plan)
	count, err := elementCount(ctx, "textarea.portfolio-editor")
	if err != nil {
		t.Fatalf("error counting portfolio editors: %v", err)
	}
	if count < 2 {
		t.Skip("plan editor not visible (no portfolio selected)")
	}

	// Verify the PLAN panel header exists
	planFound, err := commontest.EvalBool(ctx, `
		(() => {
			const headers = document.querySelectorAll('.panel-header');
			return Array.from(headers).some(h => h.textContent.includes('PLAN'));
		})()
	`)
	if err != nil {
		t.Fatalf("error checking PLAN header: %v", err)
	}
	if !planFound {
		t.Fatal("PLAN panel header not found")
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

// TestDashboardDesign checks all CSS/design constraints in a single test
func TestDashboardDesign(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

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
