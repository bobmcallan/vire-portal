package tests

import (
	"strings"
	"testing"
	"time"

	"github.com/bobmcallan/vire-portal/tests/common"
	"github.com/chromedp/chromedp"
)

func TestDashboardAuthLoad(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatal(err)
	}

	visible, err := isVisible(ctx, ".dashboard")
	if err != nil {
		t.Fatal(err)
	}
	if !visible {
		t.Error("dashboard not visible after login")
	}
}

func TestDashboardNavPresent(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatal(err)
	}

	navVisible, err := isVisible(ctx, ".nav")
	if err != nil {
		t.Fatal(err)
	}
	if !navVisible {
		t.Error("nav not visible after login")
	}

	containsBrand, brand, err := common.TextContains(ctx, ".nav-brand", "VIRE")
	if err != nil {
		t.Fatal(err)
	}
	if !containsBrand {
		t.Errorf("nav-brand = %q, want contains VIRE", brand)
	}
}

func TestDashboardSections(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatal(err)
	}

	count, err := elementCount(ctx, ".dashboard-section")
	if err != nil {
		t.Fatal(err)
	}
	if count < 2 {
		t.Errorf("dashboard sections = %d, want >= 2 (MCP + Config)", count)
	}
}

func TestDashboardNoJSErrors(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	errs := newJSErrorCollector(ctx)
	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatal(err)
	}

	if jsErrs := errs.Errors(); len(jsErrs) > 0 {
		t.Errorf("JS errors on dashboard:\n  %s", strings.Join(jsErrs, "\n  "))
	}
}

func TestDashboardAlpineInitialized(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatal(err)
	}

	alpineReady, err := common.EvalBool(ctx, `typeof Alpine !== 'undefined'`)
	if err != nil {
		t.Fatal(err)
	}
	if !alpineReady {
		t.Error("Alpine.js not initialised")
	}
}

func TestDashboardCSSLoaded(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatal(err)
	}

	var fontFamily string
	err = chromedp.Run(ctx, chromedp.Evaluate(`getComputedStyle(document.body).fontFamily`, &fontFamily))
	if err != nil {
		t.Fatal(err)
	}

	ff := strings.ToLower(fontFamily)
	if !strings.Contains(ff, "ibm plex mono") && !strings.Contains(ff, "monospace") {
		t.Errorf("font-family = %q, want IBM Plex Mono / monospace", fontFamily)
	}
}

func TestDashboardPanelsClosedOnLoad(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatal(err)
	}

	count, _ := elementCount(ctx, ".panel-collapse-trigger")
	if count == 0 {
		t.Skip("no collapsible panels on page")
	}

	hidden, err := isHidden(ctx, ".panel-collapse-body")
	if err != nil {
		t.Fatal(err)
	}
	if !hidden {
		t.Error("collapsible panel body is open on load — should be collapsed")
	}
}

func TestDashboardCollapseToggles(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatal(err)
	}

	count, _ := elementCount(ctx, ".panel-collapse-trigger")
	if count == 0 {
		t.Skip("no collapsible panels on page")
	}

	err = chromedp.Run(ctx,
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

func TestDashboardTabsSwitch(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatal(err)
	}

	count, _ := elementCount(ctx, ".tab")
	if count < 2 {
		t.Skip("fewer than 2 tabs on page")
	}

	var firstActive bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`document.querySelector('.tab').classList.contains('active')`, &firstActive),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !firstActive {
		t.Error("first tab not active by default")
	}

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

func TestDashboardNoTemplateMarkers(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatal(err)
	}

	var bodyText string
	err = chromedp.Run(ctx, chromedp.Evaluate(`document.body.innerText`, &bodyText))
	if err != nil {
		t.Fatal(err)
	}

	badMarkers := []string{"{{.", "<no value>", "{{template", "{{if", "{{range}"}
	for _, marker := range badMarkers {
		if strings.Contains(bodyText, marker) {
			t.Errorf("raw template marker %q found in page body", marker)
		}
	}
}

func TestDashboardDesignNoBorderRadius(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatal(err)
	}

	var violators string
	err = chromedp.Run(ctx,
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

func TestDashboardDesignNoBoxShadow(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatal(err)
	}

	var violators string
	err = chromedp.Run(ctx,
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
