package tests

import (
	"strings"
	"testing"
	"time"

	commontest "github.com/bobmcallan/vire-portal/tests/common"
	"github.com/chromedp/chromedp"
)

func TestCapitalAuthLoad(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/capital")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	takeScreenshot(t, ctx, "capital", "auth-load.png")

	visible, err := isVisible(ctx, ".page")
	if err != nil {
		t.Fatalf("error checking capital page visibility: %v", err)
	}
	if !visible {
		t.Fatal("capital .page not visible after login")
	}
}

func TestCapitalNoJSErrors(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	errs := newJSErrorCollector(ctx)
	err := loginAndNavigate(ctx, serverURL()+"/capital")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	takeScreenshot(t, ctx, "capital", "no-js-errors.png")

	if jsErrs := errs.Errors(); len(jsErrs) > 0 {
		t.Errorf("JS errors on capital page:\n  %s", strings.Join(jsErrs, "\n  "))
	}
}

func TestCapitalAlpineInit(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/capital")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	takeScreenshot(t, ctx, "capital", "alpine-init.png")

	alpineReady, err := commontest.EvalBool(ctx, `typeof Alpine !== 'undefined'`)
	if err != nil {
		t.Fatalf("error evaluating Alpine check: %v", err)
	}
	if !alpineReady {
		t.Fatal("Alpine.js not initialised")
	}
}

func TestCapitalPortfolioDropdown(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/capital")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	// Wait for Alpine to load portfolio data
	_ = chromedp.Run(ctx, chromedp.Sleep(500*time.Millisecond))

	takeScreenshot(t, ctx, "capital", "portfolio-dropdown.png")

	visible, err := isVisible(ctx, "select.portfolio-select")
	if err != nil {
		t.Fatalf("error checking portfolio dropdown visibility: %v", err)
	}
	if !visible {
		t.Fatal("portfolio dropdown (select.portfolio-select) not visible")
	}
}

func TestCapitalAccountBalances(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/capital")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	_ = chromedp.Run(ctx, chromedp.Sleep(1*time.Second))
	takeScreenshot(t, ctx, "capital", "account-balances.png")

	visible, err := isVisible(ctx, ".portfolio-summary-accounts")
	if err != nil {
		t.Fatalf("error checking account balances visibility: %v", err)
	}
	if !visible {
		t.Skip("account balances row not visible (no accounts available)")
	}

	count, err := elementCount(ctx, ".portfolio-summary-accounts .portfolio-summary-item")
	if err != nil {
		t.Fatalf("error counting account balance items: %v", err)
	}
	if count < 2 {
		t.Errorf("account balance item count = %d, want >= 2 (accounts + TOTAL CASH)", count)
	}

	hasTotalCash, err := commontest.EvalBool(ctx, `
		(() => {
			const row = document.querySelector('.portfolio-summary-accounts');
			if (!row) return false;
			const labels = row.querySelectorAll('.portfolio-summary-item .label');
			for (const label of labels) {
				if (label.textContent.trim() === 'TOTAL CASH') return true;
			}
			return false;
		})()
	`)
	if err != nil {
		t.Fatalf("error checking TOTAL CASH label: %v", err)
	}
	if !hasTotalCash {
		t.Error("TOTAL CASH label not found in account balances row")
	}
}

func TestCapitalCategoryBreakdown(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/capital")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	_ = chromedp.Run(ctx, chromedp.Sleep(1*time.Second))
	takeScreenshot(t, ctx, "capital", "category-breakdown.png")

	hasCategoryRow, err := commontest.EvalBool(ctx, `
		(() => {
			const rows = document.querySelectorAll('.portfolio-summary:not(.portfolio-summary-accounts)');
			for (const row of rows) {
				const style = getComputedStyle(row);
				if (style.display !== 'none' && row.querySelectorAll('.portfolio-summary-item .label').length > 0) {
					return true;
				}
			}
			return false;
		})()
	`)
	if err != nil {
		t.Fatalf("error checking category breakdown: %v", err)
	}
	if !hasCategoryRow {
		t.Skip("category breakdown row not visible (all categories zero)")
	}

	labelsUppercase, err := commontest.EvalBool(ctx, `
		(() => {
			const rows = document.querySelectorAll('.portfolio-summary:not(.portfolio-summary-accounts)');
			let row = null;
			for (const r of rows) {
				if (getComputedStyle(r).display !== 'none') { row = r; break; }
			}
			if (!row) return false;
			const labels = row.querySelectorAll('.portfolio-summary-item .label');
			if (labels.length === 0) return false;
			for (const label of labels) {
				if (label.textContent.trim() !== label.textContent.trim().toUpperCase()) return false;
			}
			return true;
		})()
	`)
	if err != nil {
		t.Fatalf("error checking category labels: %v", err)
	}
	if !labelsUppercase {
		t.Error("category breakdown labels are not uppercase")
	}
}

func TestCapitalTransactionsTable(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/capital")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	// Wait for Alpine to load transaction data
	_ = chromedp.Run(ctx, chromedp.Sleep(1*time.Second))

	takeScreenshot(t, ctx, "capital", "transactions-table.png")

	// Check if panel-headed section exists
	visible, err := isVisible(ctx, ".panel-headed")
	if err != nil {
		t.Fatalf("error checking transactions table visibility: %v", err)
	}
	if !visible {
		t.Skip("transactions table not visible (no transaction data available)")
	}

	// Verify panel header text
	headerCorrect, err := commontest.EvalBool(ctx, `
		(() => {
			const header = document.querySelector('.panel-header');
			return header && header.textContent.trim() === 'CASH TRANSACTIONS';
		})()
	`)
	if err != nil {
		t.Fatalf("error checking panel header: %v", err)
	}
	if !headerCorrect {
		t.Error("panel header text does not match 'CASH TRANSACTIONS'")
	}

	// Verify table column headers: DATE, ACCOUNT, CATEGORY, AMOUNT, DESCRIPTION
	columnsCorrect, err := commontest.EvalBool(ctx, `
		(() => {
			const ths = document.querySelectorAll('.tool-table th');
			if (ths.length !== 5) return false;
			const expected = ['DATE', 'ACCOUNT', 'CATEGORY', 'AMOUNT', 'DESCRIPTION'];
			for (let i = 0; i < 5; i++) {
				if (ths[i].textContent.trim() !== expected[i]) return false;
			}
			return true;
		})()
	`)
	if err != nil {
		t.Fatalf("error checking table columns: %v", err)
	}
	if !columnsCorrect {
		t.Error("table column headers do not match expected: DATE, ACCOUNT, CATEGORY, AMOUNT, DESCRIPTION")
	}
}

func TestCapitalTransactionColors(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/capital")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	// Wait for Alpine to render
	_ = chromedp.Run(ctx, chromedp.Sleep(1*time.Second))

	takeScreenshot(t, ctx, "capital", "transaction-colors.png")

	// Check if table has rows
	count, err := elementCount(ctx, ".tool-table tbody tr")
	if err != nil {
		t.Fatalf("error counting table rows: %v", err)
	}
	if count == 0 {
		t.Skip("no transaction rows available")
	}

	// Verify that amount cells have gain-positive or gain-negative classes
	hasColors, err := commontest.EvalBool(ctx, `
		(() => {
			const rows = document.querySelectorAll('.tool-table tbody tr');
			if (rows.length === 0) return false;
			let hasGainClass = false;
			for (const row of rows) {
				const amountCell = row.querySelectorAll('td')[3];
				if (amountCell && (amountCell.classList.contains('gain-positive') || amountCell.classList.contains('gain-negative'))) {
					hasGainClass = true;
				}
			}
			return hasGainClass;
		})()
	`)
	if err != nil {
		t.Fatalf("error checking transaction colors: %v", err)
	}
	if !hasColors {
		t.Error("transaction amount cells have no gain color classes")
	}
}

func TestCapitalPagination(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/capital")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	// Wait for Alpine to render
	_ = chromedp.Run(ctx, chromedp.Sleep(1*time.Second))

	takeScreenshot(t, ctx, "capital", "pagination.png")

	// Check if pagination element exists
	paginationExists, err := commontest.EvalBool(ctx, `document.querySelector('.pagination') !== null`)
	if err != nil {
		t.Fatalf("error checking pagination existence: %v", err)
	}
	if !paginationExists {
		t.Skip("pagination not visible (no transaction data or single page)")
	}

	// Verify pagination has PREV, page info, and NEXT elements
	paginationCorrect, err := commontest.EvalBool(ctx, `
		(() => {
			const pagination = document.querySelector('.pagination');
			if (!pagination) return false;
			const text = pagination.textContent;
			return text.includes('PREV') && text.includes('NEXT');
		})()
	`)
	if err != nil {
		t.Fatalf("error checking pagination content: %v", err)
	}
	if !paginationCorrect {
		t.Error("pagination does not contain PREV and NEXT controls")
	}
}

func TestCapitalDefaultCheckbox(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/capital")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	// Wait for Alpine to render
	_ = chromedp.Run(ctx, chromedp.Sleep(500*time.Millisecond))

	takeScreenshot(t, ctx, "capital", "default-checkbox.png")

	count, err := elementCount(ctx, ".portfolio-default-label input[type='checkbox']")
	if err != nil {
		t.Fatalf("error checking default checkbox: %v", err)
	}
	if count < 1 {
		t.Skip("default checkbox not visible (no portfolio selected)")
	}
}

func TestCapitalNoRefreshButton(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/capital")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	// Wait for Alpine to render
	_ = chromedp.Run(ctx, chromedp.Sleep(500*time.Millisecond))

	takeScreenshot(t, ctx, "capital", "no-refresh-button.png")

	// Capital page should NOT have a refresh button (per requirements)
	headerVisible, err := isVisible(ctx, ".portfolio-header")
	if err != nil {
		t.Fatalf("error checking portfolio header visibility: %v", err)
	}
	if !headerVisible {
		t.Skip("portfolio header not visible (no portfolios available)")
	}

	hasRefresh, err := commontest.EvalBool(ctx, `
		(() => {
			const header = document.querySelector('.portfolio-header');
			if (!header) return false;
			const btn = header.querySelector('button');
			return btn && btn.textContent.includes('REFRESH');
		})()
	`)
	if err != nil {
		t.Fatalf("error checking for refresh button: %v", err)
	}
	if hasRefresh {
		t.Error("capital page should NOT have a refresh button")
	}
}

func TestCapitalNoTemplateMarkers(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/capital")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	takeScreenshot(t, ctx, "capital", "no-template-markers.png")

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

func TestCapitalDesign(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/capital")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	takeScreenshot(t, ctx, "capital", "design.png")

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

func TestCapitalNavLinkFromDashboard(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	takeScreenshot(t, ctx, "capital", "nav-link-from-dashboard.png")

	// Verify Capital nav link exists in .nav-links
	exists, err := commontest.Exists(ctx, `.nav-links a[href="/capital"]`)
	if err != nil {
		t.Fatalf("error checking capital nav link: %v", err)
	}
	if !exists {
		t.Error("Capital link (a[href='/capital']) not found in .nav-links")
	}
}

func TestCapitalNavLinkActive(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/capital")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	takeScreenshot(t, ctx, "capital", "nav-link-active.png")

	// Verify Capital nav link is active on /capital page
	navLinkActive, err := commontest.EvalBool(ctx, `
		(() => {
			const link = document.querySelector('.nav-links a[href="/capital"]');
			return link !== null && link.classList.contains('active');
		})()
	`)
	if err != nil {
		t.Fatalf("error checking nav link active state: %v", err)
	}
	if !navLinkActive {
		t.Error("Capital nav link not found or not active on /capital page")
	}
}
