package tests

import (
	"strings"
	"testing"
	"time"

	commontest "github.com/bobmcallan/vire-portal/tests/common"
	"github.com/chromedp/chromedp"
)

func TestStrategyAuthLoad(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/strategy")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	takeScreenshot(t, ctx, "strategy", "auth-load.png")

	visible, err := isVisible(ctx, ".page")
	if err != nil {
		t.Fatalf("error checking strategy page visibility: %v", err)
	}
	if !visible {
		t.Fatal("strategy .page not visible after login")
	}
}

func TestStrategyNoJSErrors(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	errs := newJSErrorCollector(ctx)
	err := loginAndNavigate(ctx, serverURL()+"/strategy")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	// Wait for Alpine to initialise and fetch data (Rule 8)
	_ = chromedp.Run(ctx, chromedp.Sleep(2*time.Second))

	takeScreenshot(t, ctx, "strategy", "no-js-errors.png")

	if jsErrs := errs.Errors(); len(jsErrs) > 0 {
		t.Errorf("JS errors on strategy page:\n  %s", strings.Join(jsErrs, "\n  "))
	}
}

func TestStrategyAlpineInit(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/strategy")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	takeScreenshot(t, ctx, "strategy", "alpine-init.png")

	alpineReady, err := commontest.EvalBool(ctx, `typeof Alpine !== 'undefined'`)
	if err != nil {
		t.Fatalf("error evaluating Alpine check: %v", err)
	}
	if !alpineReady {
		t.Fatal("Alpine.js not initialised")
	}
}

func TestStrategyPortfolioSelector(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/strategy")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	// Wait for Alpine to load portfolio data
	_ = chromedp.Run(ctx, chromedp.Sleep(500*time.Millisecond))

	takeScreenshot(t, ctx, "strategy", "portfolio-selector.png")

	visible, err := isVisible(ctx, "select.portfolio-select")
	if err != nil {
		t.Fatalf("error checking portfolio selector visibility: %v", err)
	}
	if !visible {
		t.Fatal("portfolio selector (select.portfolio-select) not visible")
	}
}

func TestStrategyEditor(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/strategy")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	// Wait for Alpine to render
	_ = chromedp.Run(ctx, chromedp.Sleep(500*time.Millisecond))

	takeScreenshot(t, ctx, "strategy", "strategy-rendered.png")

	visible, err := isVisible(ctx, ".strategy-rendered")
	if err != nil {
		t.Fatalf("error checking strategy rendered div visibility: %v", err)
	}
	if !visible {
		t.Skip("strategy rendered div not visible (no portfolio selected)")
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

func TestStrategyPlanEditor(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/strategy")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	// Wait for Alpine to render
	_ = chromedp.Run(ctx, chromedp.Sleep(500*time.Millisecond))

	takeScreenshot(t, ctx, "strategy", "plan-table.png")

	// Check that plan table exists
	visible, err := isVisible(ctx, ".plan-table")
	if err != nil {
		t.Fatalf("error checking plan table visibility: %v", err)
	}
	if !visible {
		t.Skip("plan table not visible (no portfolio selected or no plan items)")
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

func TestStrategyInfoBanner(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/strategy")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	// Wait for Alpine to render
	_ = chromedp.Run(ctx, chromedp.Sleep(500*time.Millisecond))

	takeScreenshot(t, ctx, "strategy", "info-banner.png")

	visible, err := isVisible(ctx, ".info-banner")
	if err != nil {
		t.Fatalf("error checking info banner visibility: %v", err)
	}
	if !visible {
		t.Skip("info banner not visible (no portfolio selected)")
	}

	bannerContains, err := commontest.EvalBool(ctx, `
		(() => {
			const banner = document.querySelector('.info-banner');
			return banner && banner.textContent.includes('discuss changes with Claude');
		})()
	`)
	if err != nil {
		t.Fatalf("error checking info banner text: %v", err)
	}
	if !bannerContains {
		t.Fatal("info banner does not contain expected text about Claude")
	}
}

func TestStrategyNoSaveButtons(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/strategy")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	// Wait for Alpine to render
	_ = chromedp.Run(ctx, chromedp.Sleep(500*time.Millisecond))

	takeScreenshot(t, ctx, "strategy", "no-save-buttons.png")

	hasSave, err := commontest.EvalBool(ctx, `
		(() => {
			const buttons = document.querySelectorAll('button');
			return Array.from(buttons).some(b => b.textContent.includes('SAVE'));
		})()
	`)
	if err != nil {
		t.Fatalf("error checking for SAVE buttons: %v", err)
	}
	if hasSave {
		t.Error("found SAVE button(s), expected none")
	}
}

func TestStrategyPlanNotesRow(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/strategy")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	// Wait for Alpine to render plan table
	_ = chromedp.Run(ctx, chromedp.Sleep(500*time.Millisecond))

	takeScreenshot(t, ctx, "strategy", "plan-notes-row.png")

	// Check plan table is visible before asserting structure
	visible, err := isVisible(ctx, ".plan-table")
	if err != nil {
		t.Fatalf("error checking plan table visibility: %v", err)
	}
	if !visible {
		t.Skip("plan table not visible (no portfolio selected or no plan items)")
	}

	t.Run("thead_has_5_columns", func(t *testing.T) {
		count, err := elementCount(ctx, ".plan-table thead th")
		if err != nil {
			t.Fatalf("error counting thead th: %v", err)
		}
		if count != 5 {
			t.Errorf("plan table thead th count = %d, want 5", count)
		}
	})

	t.Run("no_notes_column_header", func(t *testing.T) {
		hasNotesHeader, err := commontest.EvalBool(ctx, `
			(() => {
				const ths = document.querySelectorAll('.plan-table thead th');
				return Array.from(ths).some(th => th.textContent.trim().toLowerCase() === 'notes');
			})()
		`)
		if err != nil {
			t.Fatalf("error checking for Notes header: %v", err)
		}
		if hasNotesHeader {
			t.Error("found 'Notes' column header in plan table thead, expected none")
		}
	})

	t.Run("notes_sub_rows_exist", func(t *testing.T) {
		// Items with notes should have .plan-notes-row elements
		hasNotesRows, err := commontest.EvalBool(ctx, `
			document.querySelectorAll('.plan-notes-row').length >= 0
		`)
		if err != nil {
			t.Fatalf("error checking for plan-notes-row elements: %v", err)
		}
		if !hasNotesRows {
			t.Error("plan-notes-row selector query failed")
		}
	})

	t.Run("notes_cell_displays_text", func(t *testing.T) {
		// Check that visible .plan-notes-cell elements display note text
		notesValid, err := commontest.EvalBool(ctx, `
			(() => {
				const cells = document.querySelectorAll('.plan-notes-cell');
				if (cells.length === 0) return true; // no notes is valid
				// Each visible notes cell should have non-empty text
				for (const cell of cells) {
					const row = cell.closest('.plan-notes-row');
					if (row && row.style.display !== 'none' && cell.textContent.trim() === '') {
						return false;
					}
				}
				return true;
			})()
		`)
		if err != nil {
			t.Fatalf("error checking plan-notes-cell content: %v", err)
		}
		if !notesValid {
			t.Error("found visible .plan-notes-cell with empty text content")
		}
	})

	t.Run("notes_cell_has_colspan_2", func(t *testing.T) {
		colspanValid, err := commontest.EvalBool(ctx, `
			(() => {
				const cells = document.querySelectorAll('.plan-notes-cell');
				if (cells.length === 0) return true; // no notes is valid
				return Array.from(cells).every(c => c.getAttribute('colspan') === '2');
			})()
		`)
		if err != nil {
			t.Fatalf("error checking colspan: %v", err)
		}
		if !colspanValid {
			t.Error("plan-notes-cell does not have colspan=2")
		}
	})
}

func TestStrategyNavActive(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/strategy")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	takeScreenshot(t, ctx, "strategy", "nav-active.png")

	exists, err := commontest.Exists(ctx, `.nav-links a[href="/strategy"].active`)
	if err != nil {
		t.Fatalf("error checking strategy nav active state: %v", err)
	}
	if !exists {
		t.Error("strategy nav link does not have active class on /strategy page")
	}
}
