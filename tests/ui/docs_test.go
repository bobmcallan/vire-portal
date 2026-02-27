package tests

import (
	"strings"
	"testing"

	commontest "github.com/bobmcallan/vire-portal/tests/common"
	"github.com/chromedp/chromedp"
)

func TestDocsPageLoads(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/docs")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	takeScreenshot(t, ctx, "docs", "page-loads.png")

	visible, err := isVisible(ctx, ".page")
	if err != nil {
		t.Fatalf("error checking .page visibility: %v", err)
	}
	if !visible {
		t.Fatal("docs .page not visible after login")
	}
}

func TestDocsPageHasContent(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/docs")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	takeScreenshot(t, ctx, "docs", "page-content.png")

	// Verify Navexa content is present
	navexaPresent, err := commontest.EvalBool(ctx, `
		(() => {
			const text = document.body.innerText.toLowerCase();
			return text.includes('navexa');
		})()
	`)
	if err != nil {
		t.Fatalf("error checking Navexa content: %v", err)
	}
	if !navexaPresent {
		t.Error("docs page does not contain 'Navexa' content")
	}
}

func TestDocsNavActiveState(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/docs")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	takeScreenshot(t, ctx, "docs", "nav-active.png")

	// Verify the Docs nav link has the active class when on /docs
	activeLink, err := commontest.EvalBool(ctx, `
		(() => {
			const a = document.querySelector('.nav-links a[href="/docs"]');
			return a && a.classList.contains('active');
		})()
	`)
	if err != nil {
		t.Fatalf("error checking docs nav active state: %v", err)
	}
	if !activeLink {
		t.Error("Docs nav link should have 'active' class when on /docs page")
	}
}

func TestDocsPageNoJSErrors(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	errs := newJSErrorCollector(ctx)
	err := loginAndNavigate(ctx, serverURL()+"/docs")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	takeScreenshot(t, ctx, "docs", "no-js-errors.png")

	if jsErrs := errs.Errors(); len(jsErrs) > 0 {
		t.Errorf("JS errors on docs page:\n  %v", jsErrs)
	}
}

func TestDocsPageHasNav(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/docs")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	takeScreenshot(t, ctx, "docs", "has-nav.png")

	visible, err := isVisible(ctx, ".nav")
	if err != nil {
		t.Fatalf("error checking nav visibility: %v", err)
	}
	if !visible {
		t.Error("nav should be visible on /docs page")
	}
}

func TestDocsPageHasSectionTitle(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/docs")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	takeScreenshot(t, ctx, "docs", "section-title.png")

	count, err := elementCount(ctx, ".section-title")
	if err != nil {
		t.Fatalf("error counting section titles: %v", err)
	}
	if count < 1 {
		t.Error("docs page should have at least one .section-title element")
	}
}

func TestDocsPageNoTemplateMarkers(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/docs")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	takeScreenshot(t, ctx, "docs", "no-template-markers.png")

	var bodyText string
	err = chromedp.Run(ctx, chromedp.Evaluate(`document.body.innerText`, &bodyText))
	if err != nil {
		t.Fatalf("error getting body text: %v", err)
	}

	badMarkers := []string{"{{.", "<no value>", "{{template", "{{if", "{{range}"}
	for _, marker := range badMarkers {
		if strings.Contains(bodyText, marker) {
			t.Fatalf("raw template marker %q found in docs page body", marker)
		}
	}
}
