package tests

import (
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// ════════════════════════════════════════════════════════════
// JS ERRORS — the #1 thing Claude Code breaks
// ════════════════════════════════════════════════════════════

func TestUILandingNoJSErrors(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	errs := newJSErrorCollector(ctx)
	if err := navigateAndWait(ctx, serverURL()+"/"); err != nil {
		t.Fatal(err)
	}

	if jsErrs := errs.Errors(); len(jsErrs) > 0 {
		t.Errorf("JS errors on landing page:\n  %s", strings.Join(jsErrs, "\n  "))
	}
}

func TestUIDashboardNoJSErrors(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	errs := newJSErrorCollector(ctx)
	if err := navigateAndWait(ctx, serverURL()+"/dashboard"); err != nil {
		t.Fatal(err)
	}

	if jsErrs := errs.Errors(); len(jsErrs) > 0 {
		t.Errorf("JS errors on dashboard:\n  %s", strings.Join(jsErrs, "\n  "))
	}
}

// ════════════════════════════════════════════════════════════
// PAGE RENDERS — does it return content, not a blank/error page
// ════════════════════════════════════════════════════════════

func TestUILandingRenders(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	var title, brand string
	err := chromedp.Run(ctx,
		chromedp.Navigate(serverURL()+"/"),
		chromedp.WaitVisible(".landing-title", chromedp.ByQuery),
		chromedp.Title(&title),
		chromedp.Text(".landing-title", &brand, chromedp.ByQuery),
	)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(title, "VIRE") {
		t.Errorf("title = %q, want contains VIRE", title)
	}
	if !strings.Contains(brand, "VIRE") {
		t.Errorf("landing heading = %q, want VIRE", brand)
	}
}

func TestUIDashboardRenders(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	var title string
	var sectionCount int

	err := chromedp.Run(ctx,
		chromedp.Navigate(serverURL()+"/dashboard"),
		chromedp.WaitVisible(".dashboard", chromedp.ByQuery),
		chromedp.Title(&title),
	)
	if err != nil {
		t.Fatal(err)
	}

	sectionCount, err = elementCount(ctx, ".dashboard-section")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(title, "DASHBOARD") {
		t.Errorf("title = %q, want contains DASHBOARD", title)
	}
	if sectionCount < 2 {
		t.Errorf("dashboard sections = %d, want >= 2 (MCP + Config)", sectionCount)
	}
}

// ════════════════════════════════════════════════════════════
// CSS + ALPINE LOADED
// ════════════════════════════════════════════════════════════

func TestUICSSLoaded(t *testing.T) {
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

	ff := strings.ToLower(fontFamily)
	if !strings.Contains(ff, "ibm plex mono") && !strings.Contains(ff, "monospace") {
		t.Errorf("font-family = %q, want IBM Plex Mono / monospace", fontFamily)
	}
}

func TestUIAlpineInitialised(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	var alpineReady bool
	err := chromedp.Run(ctx,
		chromedp.Navigate(serverURL()+"/dashboard"),
		chromedp.WaitVisible("body", chromedp.ByQuery),
		chromedp.Sleep(1*time.Second),
		chromedp.Evaluate(`typeof Alpine !== 'undefined'`, &alpineReady),
	)
	if err != nil {
		t.Fatal(err)
	}

	if !alpineReady {
		t.Error("Alpine.js not initialised")
	}
}

// ════════════════════════════════════════════════════════════
// DROPDOWN — not stuck open on page load
// ════════════════════════════════════════════════════════════

func TestUINavLinksPresent(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	if err := loginAndNavigate(ctx, serverURL()+"/dashboard"); err != nil {
		t.Fatal(err)
	}

	// Nav should have only Dashboard as a flat link (Settings + Logout moved to dropdown)
	count, err := elementCount(ctx, ".nav-links li")
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("expected exactly 1 flat nav item (Dashboard), got %d", count)
	}

	// Click hamburger to open dropdown, then verify Settings + Logout are there
	err = chromedp.Run(ctx,
		chromedp.Click(".nav-hamburger", chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),
	)
	if err != nil {
		t.Fatal(err)
	}

	dropdownVisible, err := isVisible(ctx, ".nav-dropdown")
	if err != nil {
		t.Fatal(err)
	}
	if !dropdownVisible {
		t.Error("expected nav-dropdown to be visible after hamburger click on desktop")
	}

	// Verify Settings link in dropdown
	var settingsExists bool
	err = chromedp.Run(ctx, chromedp.Evaluate(
		`document.querySelector(".nav-dropdown a[href='/settings']") !== null`, &settingsExists))
	if err != nil {
		t.Fatal(err)
	}
	if !settingsExists {
		t.Error("expected Settings link in dropdown")
	}

	// Verify Logout button in dropdown
	logoutCount, err := elementCount(ctx, ".nav-dropdown-logout")
	if err != nil {
		t.Fatal(err)
	}
	if logoutCount == 0 {
		t.Error("expected Logout button in dropdown")
	}
}

// ════════════════════════════════════════════════════════════
// MOBILE MENU — not stuck open, opens/closes correctly
// ════════════════════════════════════════════════════════════

func TestUIMobileMenuClosedOnLoad(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := chromedp.Run(ctx,
		chromedp.EmulateViewport(375, 812),
		chromedp.Navigate(serverURL()+"/dashboard"),
		chromedp.WaitVisible("body", chromedp.ByQuery),
		chromedp.Sleep(800*time.Millisecond),
	)
	if err != nil {
		t.Fatal(err)
	}

	hidden, err := isHidden(ctx, ".mobile-menu")
	if err != nil {
		t.Fatal(err)
	}
	if !hidden {
		t.Error("mobile menu is visible on page load — should be closed")
	}
}

func TestUIMobileMenuOpensCloses(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := chromedp.Run(ctx,
		chromedp.EmulateViewport(375, 812),
		chromedp.Navigate(serverURL()+"/dashboard"),
		chromedp.WaitVisible("body", chromedp.ByQuery),
		chromedp.Sleep(800*time.Millisecond),
	)
	if err != nil {
		t.Fatal(err)
	}

	count, err := elementCount(ctx, ".nav-hamburger")
	if err != nil {
		t.Fatal(err)
	}
	if count == 0 {
		t.Skip("no nav-hamburger found")
	}

	// Open
	err = chromedp.Run(ctx,
		chromedp.Click(".nav-hamburger", chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),
	)
	if err != nil {
		t.Fatal(err)
	}

	visible, err := isVisible(ctx, ".mobile-menu")
	if err != nil {
		t.Fatal(err)
	}
	if !visible {
		t.Error("mobile menu did not open on hamburger click")
	}

	// Close
	err = chromedp.Run(ctx,
		chromedp.Click(".mobile-menu-close", chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),
	)
	if err != nil {
		t.Fatal(err)
	}

	hidden, err := isHidden(ctx, ".mobile-menu")
	if err != nil {
		t.Fatal(err)
	}
	if !hidden {
		t.Error("mobile menu did not close")
	}
}

// ════════════════════════════════════════════════════════════
// RESPONSIVE — desktop/mobile layout switches
// ════════════════════════════════════════════════════════════

func TestUINavLinksHiddenOnMobile(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	// Authenticate first, then set mobile viewport and navigate
	if err := loginAndNavigate(ctx, serverURL()+"/dashboard"); err != nil {
		t.Fatal(err)
	}

	err := chromedp.Run(ctx,
		chromedp.EmulateViewport(375, 812),
		chromedp.Navigate(serverURL()+"/dashboard"),
		chromedp.WaitVisible(".nav", chromedp.ByQuery),
		chromedp.Sleep(300*time.Millisecond),
	)
	if err != nil {
		t.Fatal(err)
	}

	hidden, err := isHidden(ctx, ".nav-links")
	if err != nil {
		t.Fatal(err)
	}
	if !hidden {
		t.Error("nav-links should be hidden on mobile viewport")
	}
}

func TestUINavLinksVisibleOnDesktop(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	// Authenticate first, then navigate at desktop viewport
	if err := loginAndNavigate(ctx, serverURL()+"/dashboard"); err != nil {
		t.Fatal(err)
	}

	err := chromedp.Run(ctx,
		chromedp.EmulateViewport(1280, 800),
		chromedp.Navigate(serverURL()+"/dashboard"),
		chromedp.WaitVisible(".nav", chromedp.ByQuery),
		chromedp.Sleep(300*time.Millisecond),
	)
	if err != nil {
		t.Fatal(err)
	}

	visible, err := isVisible(ctx, ".nav-links")
	if err != nil {
		t.Fatal(err)
	}
	if !visible {
		t.Error("nav-links should be visible on desktop viewport")
	}
}

// ════════════════════════════════════════════════════════════
// COLLAPSIBLE PANELS — if present, not stuck open
// ════════════════════════════════════════════════════════════

func TestUICollapsePanelsClosedOnLoad(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	if err := navigateAndWait(ctx, serverURL()+"/dashboard"); err != nil {
		t.Fatal(err)
	}

	count, _ := elementCount(ctx, ".panel-collapse-trigger")
	if count == 0 {
		t.Skip("no collapsible panels on page")
	}

	var anyOpen bool
	err := chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const bodies = document.querySelectorAll('.panel-collapse-body');
				for (const b of bodies) {
					if (getComputedStyle(b).display !== 'none') return true;
				}
				return false;
			})()
		`, &anyOpen),
	)
	if err != nil {
		t.Fatal(err)
	}

	if anyOpen {
		t.Error("collapsible panel body is open on load — should be collapsed")
	}
}

func TestUICollapseToggles(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	if err := navigateAndWait(ctx, serverURL()+"/dashboard"); err != nil {
		t.Fatal(err)
	}

	count, _ := elementCount(ctx, ".panel-collapse-trigger")
	if count == 0 {
		t.Skip("no collapsible panels on page")
	}

	// Click to open
	err := chromedp.Run(ctx,
		chromedp.Click(".panel-collapse-trigger", chromedp.ByQuery),
		chromedp.Sleep(400*time.Millisecond),
	)
	if err != nil {
		t.Fatal(err)
	}

	visible, err := isVisible(ctx, ".panel-collapse-body")
	if err != nil {
		t.Fatal(err)
	}
	if !visible {
		t.Error("collapsible panel did not expand on click")
	}

	// Click to close
	err = chromedp.Run(ctx,
		chromedp.Click(".panel-collapse-trigger", chromedp.ByQuery),
		chromedp.Sleep(400*time.Millisecond),
	)
	if err != nil {
		t.Fatal(err)
	}

	hidden, err := isHidden(ctx, ".panel-collapse-body")
	if err != nil {
		t.Fatal(err)
	}
	if !hidden {
		t.Error("collapsible panel did not collapse on second click")
	}
}

// ════════════════════════════════════════════════════════════
// TABS — if present, switch correctly
// ════════════════════════════════════════════════════════════

func TestUITabsSwitch(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	if err := navigateAndWait(ctx, serverURL()+"/dashboard"); err != nil {
		t.Fatal(err)
	}

	count, _ := elementCount(ctx, ".tab")
	if count < 2 {
		t.Skip("fewer than 2 tabs on page")
	}

	// First tab should be active
	var firstActive bool
	err := chromedp.Run(ctx,
		chromedp.Evaluate(`document.querySelector('.tab').classList.contains('active')`, &firstActive),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !firstActive {
		t.Error("first tab not active by default")
	}

	// Click second tab
	var secondActive bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`document.querySelectorAll('.tab')[1].click()`, nil),
		chromedp.Sleep(300*time.Millisecond),
		chromedp.Evaluate(`document.querySelectorAll('.tab')[1].classList.contains('active')`, &secondActive),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !secondActive {
		t.Error("second tab not active after click")
	}
}

// ════════════════════════════════════════════════════════════
// TOASTS — dispatch works
// ════════════════════════════════════════════════════════════

func TestUIToastFires(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	if err := navigateAndWait(ctx, serverURL()+"/dashboard"); err != nil {
		t.Fatal(err)
	}

	count, _ := elementCount(ctx, ".toast-container")
	if count == 0 {
		t.Skip("no toast-container on page")
	}

	var toastCount int
	err := chromedp.Run(ctx,
		chromedp.Evaluate(`window.dispatchEvent(new CustomEvent('toast', { detail: { msg: 'test' } }))`, nil),
		chromedp.Sleep(500*time.Millisecond),
		chromedp.Evaluate(`document.querySelectorAll('.toast').length`, &toastCount),
	)
	if err != nil {
		t.Fatal(err)
	}

	if toastCount < 1 {
		t.Error("toast did not appear after dispatch")
	}
}

// ════════════════════════════════════════════════════════════
// DESIGN RULES — enforce B&W aesthetic
// ════════════════════════════════════════════════════════════

func TestUIDesignNoBorderRadius(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	if err := navigateAndWait(ctx, serverURL()+"/dashboard"); err != nil {
		t.Fatal(err)
	}

	var violators string
	err := chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const bad = [];
				document.querySelectorAll('*').forEach(el => {
					const r = getComputedStyle(el).borderRadius;
					if (r && r !== '0px') {
						const id = el.id ? '#'+el.id : '';
						const cls = el.className ? '.'+el.className.split(' ')[0] : '';
						bad.push(el.tagName.toLowerCase() + id + cls + ' (' + r + ')');
					}
				});
				return bad.slice(0, 5).join(', ');
			})()
		`, &violators),
	)
	if err != nil {
		t.Fatal(err)
	}

	if violators != "" {
		t.Errorf("border-radius found on: %s", violators)
	}
}

func TestUIDesignNoBoxShadow(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	if err := navigateAndWait(ctx, serverURL()+"/dashboard"); err != nil {
		t.Fatal(err)
	}

	var violators string
	err := chromedp.Run(ctx,
		chromedp.Evaluate(`
			(() => {
				const bad = [];
				document.querySelectorAll('*').forEach(el => {
					const s = getComputedStyle(el).boxShadow;
					if (s && s !== 'none') {
						const id = el.id ? '#'+el.id : '';
						const cls = el.className ? '.'+el.className.split(' ')[0] : '';
						bad.push(el.tagName.toLowerCase() + id + cls);
					}
				});
				return bad.slice(0, 5).join(', ');
			})()
		`, &violators),
	)
	if err != nil {
		t.Fatal(err)
	}

	if violators != "" {
		t.Errorf("box-shadow found on: %s", violators)
	}
}

// ════════════════════════════════════════════════════════════
// TEMPLATE DATA — Go template vars actually rendered
// ════════════════════════════════════════════════════════════

func TestUIDashboardTemplateData(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	if err := navigateAndWait(ctx, serverURL()+"/dashboard"); err != nil {
		t.Fatal(err)
	}

	// Check page doesn't contain raw Go template markers
	var bodyText string
	err := chromedp.Run(ctx,
		chromedp.Evaluate(`document.body.innerText`, &bodyText),
	)
	if err != nil {
		t.Fatal(err)
	}

	badMarkers := []string{"{{.", "<no value>", "{{template", "{{if", "{{range"}
	for _, marker := range badMarkers {
		if strings.Contains(bodyText, marker) {
			t.Errorf("raw template marker %q found in page body", marker)
		}
	}
}
