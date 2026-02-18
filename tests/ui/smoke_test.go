package tests

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/bobmcallan/vire-portal/tests/common"
	"github.com/chromedp/chromedp"
)

func TestSmokeLandingNoJSErrors(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	errs := newJSErrorCollector(ctx)
	if err := navigateAndWait(ctx, serverURL()+"/"); err != nil {
		t.Fatal(err)
	}

	takeScreenshot(t, ctx, "smoke", "landing-no-js-errors.png")

	if jsErrs := errs.Errors(); len(jsErrs) > 0 {
		t.Errorf("JS errors on landing page:\n  %s", strings.Join(jsErrs, "\n  "))
	}
}

func TestSmokeLandingLoginButtons(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	if err := navigateAndWait(ctx, serverURL()+"/"); err != nil {
		t.Fatal(err)
	}

	takeScreenshot(t, ctx, "smoke", "landing-login-buttons.png")

	googleVisible, err := isVisible(ctx, `a[href="/api/auth/login/google"]`)
	if err != nil {
		t.Fatal(err)
	}
	if !googleVisible {
		t.Error("Google login button not visible")
	}

	githubVisible, err := isVisible(ctx, `a[href="/api/auth/login/github"]`)
	if err != nil {
		t.Fatal(err)
	}
	if !githubVisible {
		t.Error("GitHub login button not visible")
	}
}

func TestSmokeLandingBranding(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	var brand string
	err := chromedp.Run(ctx,
		chromedp.Navigate(serverURL()+"/"),
		chromedp.WaitVisible(".landing-title", chromedp.ByQuery),
		chromedp.Text(".landing-title", &brand, chromedp.ByQuery),
	)
	if err != nil {
		t.Fatal(err)
	}

	takeScreenshot(t, ctx, "smoke", "landing-branding.png")

	if !strings.Contains(brand, "VIRE") {
		t.Errorf("landing title = %q, want contains VIRE", brand)
	}
}

func TestSmokeDashboardLoads(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	if err := navigateAndWait(ctx, serverURL()+"/dashboard"); err != nil {
		t.Fatal(err)
	}

	takeScreenshot(t, ctx, "smoke", "dashboard-loads.png")

	count, err := elementCount(ctx, ".dashboard-section")
	if err != nil {
		t.Fatal(err)
	}
	if count < 1 {
		t.Errorf("dashboard sections = %d, want >= 1", count)
	}

	// Nav menu is only visible when logged in - without login, it should be hidden
	navHidden, err := isHidden(ctx, ".nav")
	if err != nil {
		t.Logf("nav check skipped (may not exist): %v", err)
	} else if !navHidden {
		t.Error("nav menu should be hidden on dashboard when not logged in")
	}
}

func TestSmokeDashboardNoJSErrors(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	errs := newJSErrorCollector(ctx)
	if err := navigateAndWait(ctx, serverURL()+"/dashboard"); err != nil {
		t.Fatal(err)
	}

	takeScreenshot(t, ctx, "smoke", "dashboard-no-js-errors.png")

	if jsErrs := errs.Errors(); len(jsErrs) > 0 {
		t.Errorf("JS errors on dashboard:\n  %s", strings.Join(jsErrs, "\n  "))
	}
}

func TestSmokeDevLoginFlow(t *testing.T) {
	if os.Getenv("PORTAL_ENV") != "dev" {
		t.Skip("PORTAL_ENV not set to 'dev'")
	}

	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatal(err)
	}

	takeScreenshot(t, ctx, "smoke", "dev-login-flow.png")

	navVisible, err := isVisible(ctx, ".nav")
	if err != nil {
		t.Fatal(err)
	}
	if !navVisible {
		t.Error("nav not visible after dev login")
	}
}

func TestSmokeCSSLoaded(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	var fontFamily string
	err := chromedp.Run(ctx,
		chromedp.Navigate(serverURL()+"/dashboard"),
		chromedp.WaitVisible("body", chromedp.ByQuery),
		chromedp.Evaluate(`getComputedStyle(document.body).fontFamily`, &fontFamily),
	)
	if err != nil {
		t.Fatal(err)
	}

	takeScreenshot(t, ctx, "smoke", "css-loaded.png")

	ff := strings.ToLower(fontFamily)
	if !strings.Contains(ff, "ibm plex mono") && !strings.Contains(ff, "monospace") {
		t.Errorf("font-family = %q, want IBM Plex Mono / monospace", fontFamily)
	}
}

func TestSmokeAlpineInitialized(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	if err := navigateAndWait(ctx, serverURL()+"/dashboard"); err != nil {
		t.Fatal(err)
	}

	takeScreenshot(t, ctx, "smoke", "alpine-initialized.png")

	alpineReady, err := common.EvalBool(ctx, `typeof Alpine !== 'undefined'`)
	if err != nil {
		t.Fatal(err)
	}

	if !alpineReady {
		t.Error("Alpine.js not initialised")
	}
}

func navigateToURL(ctx context.Context, url string) error {
	return chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.WaitVisible("body", chromedp.ByQuery),
	)
}
