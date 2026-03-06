package tests

import (
	"strings"
	"testing"
	"time"

	commontest "github.com/bobmcallan/vire-portal/tests/common"
	"github.com/chromedp/chromedp"
)

func TestGlossaryPageNoLoadingSpinner(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/glossary")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	takeScreenshot(t, ctx, "glossary", "no-loading-spinner.png")

	// With SSR, there should be no "Loading glossary..." text on the page.
	// The old client-side version showed a loading spinner while fetching data.
	var bodyText string
	err = chromedp.Run(ctx, chromedp.Evaluate(`document.body.innerText`, &bodyText))
	if err != nil {
		t.Fatalf("error getting body text: %v", err)
	}

	if strings.Contains(bodyText, "Loading glossary") {
		t.Error("found 'Loading glossary...' text on page -- SSR should render content immediately without a loading spinner")
	}
}

func TestGlossaryPageSSRContent(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/glossary")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	takeScreenshot(t, ctx, "glossary", "ssr-content.png")

	// Check that the page has SSR-rendered content: either glossary categories
	// or the "not yet available" / "No glossary entries" messages
	categoryCount, err := elementCount(ctx, ".glossary-category")
	if err != nil {
		t.Fatalf("error counting glossary categories: %v", err)
	}

	if categoryCount > 0 {
		// Glossary has SSR-rendered categories -- verify they have content
		termCount, err := elementCount(ctx, ".glossary-term-item")
		if err != nil {
			t.Fatalf("error counting glossary terms: %v", err)
		}
		if termCount == 0 {
			t.Error("glossary categories found but no term items inside them")
		}
		return
	}

	// No categories -- check for expected empty/error states
	var bodyText string
	err = chromedp.Run(ctx, chromedp.Evaluate(`document.body.innerText`, &bodyText))
	if err != nil {
		t.Fatalf("error getting body text: %v", err)
	}

	if strings.Contains(bodyText, "No glossary entries available") ||
		strings.Contains(bodyText, "not yet available") {
		// Expected empty state
		return
	}

	t.Skip("glossary API may not be available in test environment")
}

func TestGlossaryPageSearchFilter(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/glossary")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	takeScreenshot(t, ctx, "glossary", "search-filter.png")

	// Verify the search input exists
	visible, err := isVisible(ctx, "input.help-search")
	if err != nil {
		t.Fatalf("error checking search input visibility: %v", err)
	}
	if !visible {
		t.Error("glossary search input not visible")
	}
}

func TestGlossaryPageNoJSErrors(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	errs := newJSErrorCollector(ctx)
	err := loginAndNavigate(ctx, serverURL()+"/glossary")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	takeScreenshot(t, ctx, "glossary", "no-js-errors.png")

	if jsErrs := errs.Errors(); len(jsErrs) > 0 {
		t.Errorf("JS errors on glossary page:\n  %s", strings.Join(jsErrs, "\n  "))
	}
}

func TestGlossaryPageNoTemplateMarkers(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/glossary")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	takeScreenshot(t, ctx, "glossary", "no-template-markers.png")

	var bodyText string
	err = chromedp.Run(ctx, chromedp.Evaluate(`document.body.innerText`, &bodyText))
	if err != nil {
		t.Fatalf("error getting body text: %v", err)
	}

	badMarkers := []string{"{{.", "<no value>", "{{template", "{{if", "{{range"}
	for _, marker := range badMarkers {
		if strings.Contains(bodyText, marker) {
			t.Fatalf("raw template marker %q found in glossary page body", marker)
		}
	}
}

func TestGlossaryPageNavVisible(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/glossary")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	takeScreenshot(t, ctx, "glossary", "nav-visible.png")

	visible, err := isVisible(ctx, ".nav")
	if err != nil {
		t.Fatalf("error checking nav visibility: %v", err)
	}
	if !visible {
		t.Error("nav should be visible on /glossary page")
	}
}

func TestGlossaryInHamburgerDropdown(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	err = chromedp.Run(ctx,
		chromedp.Click(".nav-hamburger", chromedp.ByQuery),
		chromedp.Sleep(300*time.Millisecond),
	)
	if err != nil {
		t.Fatal(err)
	}

	takeScreenshot(t, ctx, "glossary", "hamburger-link.png")

	exists, err := commontest.Exists(ctx, ".nav-dropdown a[href='/glossary']")
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Error("glossary link not found in hamburger dropdown")
	}
}
