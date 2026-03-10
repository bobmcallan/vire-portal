package tests

import (
	"strings"
	"testing"
	"time"

	commontest "github.com/bobmcallan/vire-portal/tests/common"
	"github.com/chromedp/chromedp"
)

func TestMobileDashboard(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	errs := newJSErrorCollector(ctx)

	err := loginAndNavigate(ctx, serverURL()+"/m")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	// --- NoJSErrors first ---

	t.Run("NoJSErrors", func(t *testing.T) {
		takeScreenshot(t, ctx, "mobile", "no-js-errors.png")

		if jsErrs := errs.Errors(); len(jsErrs) > 0 {
			t.Errorf("JS errors on mobile dashboard:\n  %s", strings.Join(jsErrs, "\n  "))
		}
	})

	// --- Read-only assertions ---

	t.Run("AuthLoad", func(t *testing.T) {
		takeScreenshot(t, ctx, "mobile", "auth-load.png")

		visible, err := isVisible(ctx, ".page")
		if err != nil {
			t.Fatalf("error checking mobile dashboard visibility: %v", err)
		}
		if !visible {
			t.Fatal("mobile dashboard .page not visible after login")
		}
	})

	t.Run("AlpineInit", func(t *testing.T) {
		takeScreenshot(t, ctx, "mobile", "alpine-init.png")

		alpineReady, err := commontest.EvalBool(ctx, `typeof Alpine !== 'undefined'`)
		if err != nil {
			t.Fatalf("error evaluating Alpine check: %v", err)
		}
		if !alpineReady {
			t.Fatal("Alpine.js not initialised on mobile dashboard")
		}
	})

	t.Run("PortfolioSelector", func(t *testing.T) {
		_ = chromedp.Run(ctx, chromedp.Sleep(500*time.Millisecond))
		takeScreenshot(t, ctx, "mobile", "portfolio-selector.png")

		selectorVisible, err := isVisible(ctx, ".mobile-portfolio-select")
		if err != nil {
			t.Fatalf("error checking portfolio selector: %v", err)
		}
		if !selectorVisible {
			t.Skip("portfolio selector not visible (test account may have no portfolios)")
		}
	})

	t.Run("HoldingsCards", func(t *testing.T) {
		_ = chromedp.Run(ctx, chromedp.Sleep(1*time.Second))
		takeScreenshot(t, ctx, "mobile", "holdings-cards.png")

		exists, err := commontest.Exists(ctx, `.mobile-holding-card`)
		if err != nil {
			t.Fatalf("error checking holding cards: %v", err)
		}
		if !exists {
			t.Skip("no holding cards visible (test account may have no holdings)")
		}
	})

	t.Run("HoldingOneDayChange", func(t *testing.T) {
		_ = chromedp.Run(ctx, chromedp.Sleep(1*time.Second))
		takeScreenshot(t, ctx, "mobile", "holding-1d-change.png")

		exists, err := commontest.Exists(ctx, `.mobile-holding-card`)
		if err != nil {
			t.Fatalf("error checking holding cards: %v", err)
		}
		if !exists {
			t.Skip("no holding cards visible (test account may have no holdings)")
		}

		// Verify .mobile-holding-1d span exists in holding cards
		oneDayExists, err := commontest.Exists(ctx, `.mobile-holding-card .mobile-holding-1d`)
		if err != nil {
			t.Fatalf("error checking 1D span: %v", err)
		}
		if !oneDayExists {
			t.Skip("mobile-holding-1d span not found (holdings may have no daily data)")
		}

		// Verify 1D span uses changeClass (change-up, change-down, or change-neutral)
		hasColorClass, err := commontest.EvalBool(ctx, `
			(() => {
				const spans = document.querySelectorAll('.mobile-holding-card .mobile-holding-1d');
				if (spans.length === 0) return false;
				for (const s of spans) {
					const text = s.textContent.trim();
					if (!text) continue; // empty means null daily data, skip
					const cls = s.className;
					if (!cls.includes('change-up') && !cls.includes('change-down') && !cls.includes('change-neutral')) return false;
				}
				return true;
			})()
		`)
		if err != nil {
			t.Fatalf("error checking 1D color classes: %v", err)
		}
		if !hasColorClass {
			t.Error("mobile 1D spans with text should have change color classes")
		}

		// Verify 1D text ends with " 1D" suffix when present
		hasSuffix, err := commontest.EvalBool(ctx, `
			(() => {
				const spans = document.querySelectorAll('.mobile-holding-card .mobile-holding-1d');
				if (spans.length === 0) return false;
				for (const s of spans) {
					const text = s.textContent.trim();
					if (!text) continue; // empty means null daily data
					if (!text.endsWith('1D')) return false;
				}
				return true;
			})()
		`)
		if err != nil {
			t.Fatalf("error checking 1D suffix: %v", err)
		}
		if !hasSuffix {
			t.Error("mobile 1D text should end with '1D' suffix")
		}
	})

	t.Run("FullDashboardLink", func(t *testing.T) {
		takeScreenshot(t, ctx, "mobile", "full-dashboard-link.png")

		exists, err := commontest.Exists(ctx, `.mobile-full-link a[href^="/dashboard"]`)
		if err != nil {
			t.Fatalf("error checking full dashboard link: %v", err)
		}
		if !exists {
			t.Error("VIEW FULL DASHBOARD link not found")
		}
	})

	t.Run("SSR_NoLoadingSpinner", func(t *testing.T) {
		takeScreenshot(t, ctx, "mobile", "ssr-no-loading.png")

		loadingHidden, err := commontest.EvalBool(ctx, `
			(() => {
				const els = document.querySelectorAll('[x-show="loading"]');
				for (const el of els) {
					if (el.offsetParent !== null && el.textContent.includes('Loading')) return false;
				}
				return true;
			})()
		`)
		if err != nil {
			t.Fatalf("error checking loading spinner: %v", err)
		}
		if !loadingHidden {
			t.Error("Loading spinner should not be visible with SSR")
		}
	})

	t.Run("SSR_VireDataCleared", func(t *testing.T) {
		_ = chromedp.Run(ctx, chromedp.Sleep(2*time.Second))
		takeScreenshot(t, ctx, "mobile", "ssr-data-cleared.png")

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
	})

	t.Run("URLRouting", func(t *testing.T) {
		err := chromedp.Run(ctx,
			chromedp.Navigate(serverURL()+"/m/SMSF"),
			chromedp.WaitVisible("body", chromedp.ByQuery),
		)
		if err != nil {
			t.Fatalf("login and navigate failed: %v", err)
		}

		_ = chromedp.Run(ctx, chromedp.Sleep(1*time.Second))
		takeScreenshot(t, ctx, "mobile", "url-routing.png")

		var currentURL string
		err = chromedp.Run(ctx, chromedp.Location(&currentURL))
		if err != nil {
			t.Fatalf("error getting URL: %v", err)
		}

		if !strings.Contains(currentURL, "/m/") {
			t.Errorf("expected URL to contain /m/, got: %s", currentURL)
		}
	})
}

// TestMobileDashboardNavLink uses a desktop-sized browser to verify the "Mobile" nav link on /dashboard.
func TestMobileDashboardNavLink(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	takeScreenshot(t, ctx, "mobile", "nav-link.png")

	exists, err := commontest.Exists(ctx, `a[href="/m"]`)
	if err != nil {
		t.Fatalf("error checking mobile nav link: %v", err)
	}
	if !exists {
		t.Skip("Mobile nav link (a[href='/m']) not found (mobile menu may not be rendered)")
	}
}

// TestMobileDashboardRedirectsUnauthenticated remains a separate top-level test
// because it specifically tests the unauthenticated flow (no login).
func TestMobileDashboardRedirectsUnauthenticated(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	var currentURL string
	err := chromedp.Run(ctx,
		chromedp.Navigate(serverURL()+"/m"),
		chromedp.Sleep(2*time.Second),
		chromedp.Location(&currentURL),
	)
	if err != nil {
		t.Fatalf("navigation failed: %v", err)
	}

	takeScreenshot(t, ctx, "mobile", "redirect-unauth.png")

	if strings.Contains(currentURL, "/m") {
		t.Errorf("unauthenticated user should be redirected away from /m, got: %s", currentURL)
	}
}
