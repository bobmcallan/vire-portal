package tests

import (
	"strings"
	"testing"
	"time"

	commontest "github.com/bobmcallan/vire-portal/tests/common"
	"github.com/chromedp/chromedp"
)

func TestChangelogPageLoads(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/changelog")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	takeScreenshot(t, ctx, "changelog", "page-loads.png")

	visible, err := isVisible(ctx, ".page")
	if err != nil {
		t.Fatalf("error checking .page visibility: %v", err)
	}
	if !visible {
		t.Fatal("changelog .page not visible after login")
	}
}

func TestChangelogPageNavVisible(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/changelog")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	takeScreenshot(t, ctx, "changelog", "nav-visible.png")

	visible, err := isVisible(ctx, ".nav")
	if err != nil {
		t.Fatalf("error checking nav visibility: %v", err)
	}
	if !visible {
		t.Error("nav should be visible on /changelog page")
	}
}

func TestChangelogPageNoJSErrors(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	errs := newJSErrorCollector(ctx)
	err := loginAndNavigate(ctx, serverURL()+"/changelog")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	takeScreenshot(t, ctx, "changelog", "no-js-errors.png")

	if jsErrs := errs.Errors(); len(jsErrs) > 0 {
		t.Errorf("JS errors on changelog page:\n  %v", jsErrs)
	}
}

func TestChangelogPageNoTemplateMarkers(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/changelog")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	takeScreenshot(t, ctx, "changelog", "no-template-markers.png")

	var bodyText string
	err = chromedp.Run(ctx, chromedp.Evaluate(`document.body.innerText`, &bodyText))
	if err != nil {
		t.Fatalf("error getting body text: %v", err)
	}

	badMarkers := []string{"{{.", "<no value>", "{{template", "{{if", "{{range"}
	for _, marker := range badMarkers {
		if strings.Contains(bodyText, marker) {
			t.Fatalf("raw template marker %q found in changelog page body", marker)
		}
	}
}

func TestChangelogPageContent(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/changelog")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	// Wait 2s for initial content load
	err = chromedp.Run(ctx, chromedp.Sleep(2*time.Second))
	if err != nil {
		t.Fatalf("error during wait: %v", err)
	}

	takeScreenshot(t, ctx, "changelog", "content.png")

	// Check if either changelog entries exist OR empty state is shown
	entryCount, err := elementCount(ctx, ".changelog-entry")
	if err != nil {
		t.Fatalf("error counting changelog entries: %v", err)
	}

	if entryCount > 0 {
		// Changelog has entries
		return
	}

	var bodyText string
	err = chromedp.Run(ctx, chromedp.Evaluate(`document.body.innerText`, &bodyText))
	if err != nil {
		t.Fatalf("error getting body text: %v", err)
	}

	if strings.Contains(bodyText, "No changelog entries") {
		// Empty state — valid
		return
	}

	// API error in test environment (no changelog endpoint on stub server) — skip
	if strings.Contains(bodyText, "Failed to load changelog") {
		t.Skip("changelog API not available in test environment")
	}

	t.Error("unexpected page state: no entries, no empty state, no error message")
}

func TestChangelogInHamburgerDropdown(t *testing.T) {
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

	takeScreenshot(t, ctx, "changelog", "hamburger-link.png")

	exists, err := commontest.Exists(ctx, ".nav-dropdown a[href='/changelog']")
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Error("changelog link not found in hamburger dropdown")
	}
}

func TestChangelogInMobileMenu(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(375, 812),
		chromedp.Navigate(serverURL()+"/dashboard"),
		chromedp.WaitVisible("body", chromedp.ByQuery),
		chromedp.Sleep(800*time.Millisecond),
	)
	if err != nil {
		t.Fatal(err)
	}

	// Open mobile menu
	count, _ := elementCount(ctx, ".nav-hamburger")
	if count == 0 {
		t.Skip("no nav-hamburger found")
	}

	err = chromedp.Run(ctx,
		chromedp.Click(".nav-hamburger", chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),
	)
	if err != nil {
		t.Fatal(err)
	}

	takeScreenshot(t, ctx, "changelog", "mobile-link.png")

	exists, err := commontest.Exists(ctx, `.mobile-menu a[href='/changelog']`)
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Error("changelog link not found in mobile menu")
	}
}
