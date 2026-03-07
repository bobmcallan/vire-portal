package tests

import (
	"strings"
	"testing"
	"time"

	commontest "github.com/bobmcallan/vire-portal/tests/common"
	"github.com/chromedp/chromedp"
)

func TestMobileDashboardAuthLoad(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/m")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	takeScreenshot(t, ctx, "mobile", "auth-load.png")

	visible, err := isVisible(ctx, ".page")
	if err != nil {
		t.Fatalf("error checking mobile dashboard visibility: %v", err)
	}
	if !visible {
		t.Fatal("mobile dashboard .page not visible after login")
	}
}

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

func TestMobileDashboardNoJSErrors(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	errs := newJSErrorCollector(ctx)
	err := loginAndNavigate(ctx, serverURL()+"/m")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	takeScreenshot(t, ctx, "mobile", "no-js-errors.png")

	if jsErrs := errs.Errors(); len(jsErrs) > 0 {
		t.Errorf("JS errors on mobile dashboard:\n  %s", strings.Join(jsErrs, "\n  "))
	}
}

func TestMobileDashboardAlpineInit(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/m")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	takeScreenshot(t, ctx, "mobile", "alpine-init.png")

	alpineReady, err := commontest.EvalBool(ctx, `typeof Alpine !== 'undefined'`)
	if err != nil {
		t.Fatalf("error evaluating Alpine check: %v", err)
	}
	if !alpineReady {
		t.Fatal("Alpine.js not initialised on mobile dashboard")
	}
}

func TestMobileDashboardPortfolioSelector(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/m")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	_ = chromedp.Run(ctx, chromedp.Sleep(500*time.Millisecond))
	takeScreenshot(t, ctx, "mobile", "portfolio-selector.png")

	selectorVisible, err := isVisible(ctx, ".mobile-portfolio-select")
	if err != nil {
		t.Fatalf("error checking portfolio selector: %v", err)
	}
	if !selectorVisible {
		t.Skip("portfolio selector not visible (test account may have no portfolios)")
	}
}

func TestMobileDashboardHoldingsCards(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/m")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	_ = chromedp.Run(ctx, chromedp.Sleep(1*time.Second))
	takeScreenshot(t, ctx, "mobile", "holdings-cards.png")

	exists, err := commontest.Exists(ctx, `.mobile-holding-card`)
	if err != nil {
		t.Fatalf("error checking holding cards: %v", err)
	}
	if !exists {
		t.Skip("no holding cards visible (test account may have no holdings)")
	}
}

func TestMobileDashboardFullDashboardLink(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/m")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	takeScreenshot(t, ctx, "mobile", "full-dashboard-link.png")

	exists, err := commontest.Exists(ctx, `.mobile-full-link a[href^="/dashboard"]`)
	if err != nil {
		t.Fatalf("error checking full dashboard link: %v", err)
	}
	if !exists {
		t.Error("VIEW FULL DASHBOARD link not found")
	}
}

func TestMobileDashboardSSR_NoLoadingSpinner(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/m")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

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
}

func TestMobileDashboardSSR_VireDataCleared(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/m")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

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
}

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
		t.Error("Mobile nav link (a[href='/m']) not found")
	}
}

func TestMobileDashboardURLRouting(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/m/SMSF")
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
}
