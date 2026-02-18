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

	// AFTER screenshot - state after login
	takeScreenshot(t, ctx, "TestDashboardAuthLoad_01_after")

	visible, err := isVisible(ctx, ".dashboard")
	if err != nil {
		t.Fatalf("error checking dashboard visibility: %v", err)
	}
	if !visible {
		t.Fatal("dashboard not visible after login")
	}
}

func TestDashboardNavPresent(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	// AFTER screenshot - state after login
	takeScreenshot(t, ctx, "TestDashboardNav_01_after")

	navVisible, err := isVisible(ctx, ".nav")
	if err != nil {
		t.Fatalf("error checking nav visibility: %v", err)
	}
	if !navVisible {
		t.Fatal("nav not visible after login")
	}

	containsBrand, brand, err := commontest.TextContains(ctx, ".nav-brand", "VIRE")
	if err != nil {
		t.Fatalf("error checking nav-brand text: %v", err)
	}
	if !containsBrand {
		t.Fatalf("nav-brand = %q, want contains VIRE", brand)
	}
}

func TestDashboardSections(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	// AFTER screenshot - state after login
	takeScreenshot(t, ctx, "TestDashboardSections_01_after")

	count, err := elementCount(ctx, ".dashboard-section")
	if err != nil {
		t.Fatalf("error counting dashboard sections: %v", err)
	}
	if count < 2 {
		t.Fatalf("dashboard sections = %d, want >= 2 (MCP + Config)", count)
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

	// AFTER screenshot - state after login
	takeScreenshot(t, ctx, "TestDashboardNoJSErrors_01_after")

	if jsErrs := errs.Errors(); len(jsErrs) > 0 {
		t.Errorf("JS errors on dashboard:\n  %s", strings.Join(jsErrs, "\n  "))
	}
}

func TestDashboardAlpineInitialized(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	// AFTER screenshot - state after login
	takeScreenshot(t, ctx, "TestDashboardAlpine_01_after")

	alpineReady, err := commontest.EvalBool(ctx, `typeof Alpine !== 'undefined'`)
	if err != nil {
		t.Fatalf("error evaluating Alpine check: %v", err)
	}
	if !alpineReady {
		t.Fatal("Alpine.js not initialised")
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

	// AFTER screenshot - state after login
	takeScreenshot(t, ctx, "TestDashboardDesign_01_after")

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

func TestDashboardPanels(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	// AFTER screenshot - state after login
	takeScreenshot(t, ctx, "TestDashboardPanels_01_after")

	count, _ := elementCount(ctx, ".panel-collapse-trigger")
	if count == 0 {
		t.Skip("no collapsible panels on page")
	}

	// Check panels are collapsed on load
	hidden, err := isHidden(ctx, ".panel-collapse-body")
	if err != nil {
		t.Fatalf("error checking panel visibility: %v", err)
	}
	if !hidden {
		t.Fatal("collapsible panel body is open on load — should be collapsed")
	}

	// Click to expand
	err = chromedp.Run(ctx,
		chromedp.Click(".panel-collapse-trigger", chromedp.ByQuery),
		chromedp.Sleep(400*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("error clicking panel trigger: %v", err)
	}

	// AFTER click screenshot
	takeScreenshot(t, ctx, "TestDashboardPanels_02_expanded")

	visible, err := isVisible(ctx, ".panel-collapse-body")
	if err != nil {
		t.Fatalf("error checking panel body visibility: %v", err)
	}
	if !visible {
		t.Fatal("collapsible panel did not expand on click")
	}

	// Click to collapse
	err = chromedp.Run(ctx,
		chromedp.Click(".panel-collapse-trigger", chromedp.ByQuery),
		chromedp.Sleep(400*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("error clicking panel trigger again: %v", err)
	}

	// AFTER second click screenshot
	takeScreenshot(t, ctx, "TestDashboardPanels_03_collapsed")

	hidden, err = isHidden(ctx, ".panel-collapse-body")
	if err != nil {
		t.Fatalf("error checking panel body visibility: %v", err)
	}
	if !hidden {
		t.Fatal("collapsible panel did not collapse on second click")
	}
}

func TestDashboardTabs(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	// AFTER screenshot - state after login
	takeScreenshot(t, ctx, "TestDashboardTabs_01_after")

	count, _ := elementCount(ctx, ".tab")
	if count < 2 {
		t.Skip("fewer than 2 tabs on page")
	}

	var firstActive bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`document.querySelector('.tab').classList.contains('active')`, &firstActive),
	)
	if err != nil {
		t.Fatalf("error checking first tab state: %v", err)
	}
	if !firstActive {
		t.Fatal("first tab not active by default")
	}

	err = chromedp.Run(ctx,
		chromedp.Evaluate(`document.querySelectorAll('.tab')[1].click()`, nil),
		chromedp.Sleep(300*time.Millisecond),
		chromedp.Evaluate(`document.querySelectorAll('.tab')[1].classList.contains('active')`, &firstActive),
	)
	if err != nil {
		t.Fatalf("error clicking second tab: %v", err)
	}

	// AFTER click screenshot
	takeScreenshot(t, ctx, "TestDashboardTabs_02_clicked")

	if !firstActive {
		t.Fatal("second tab not active after click")
	}
}

func TestDashboardNoTemplateMarkers(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	// AFTER screenshot - state after login
	takeScreenshot(t, ctx, "TestDashboardNoTemplateMarkers_01_after")

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
