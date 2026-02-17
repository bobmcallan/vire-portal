package tests

import (
	"testing"
	"time"

	"github.com/bobmcallan/vire-portal/tests/common"
	"github.com/chromedp/chromedp"
)

func TestNavBrandText(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatal(err)
	}

	containsBrand, brand, err := common.TextContains(ctx, ".nav-brand", "VIRE")
	if err != nil {
		t.Fatal(err)
	}
	if !containsBrand {
		t.Errorf("nav-brand = %q, want contains VIRE", brand)
	}
}

func TestNavHamburgerVisible(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatal(err)
	}

	visible, err := isVisible(ctx, ".nav-hamburger")
	if err != nil {
		t.Fatal(err)
	}
	if !visible {
		t.Error("nav-hamburger not visible on desktop")
	}
}

func TestNavDropdownHiddenByDefault(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatal(err)
	}

	hidden, err := isHidden(ctx, ".nav-dropdown")
	if err != nil {
		t.Fatal(err)
	}
	if !hidden {
		t.Error("nav-dropdown should be hidden by default")
	}
}

func TestNavDropdownOpensOnClick(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatal(err)
	}

	err = chromedp.Run(ctx,
		chromedp.Click(".nav-hamburger", chromedp.ByQuery),
		chromedp.Sleep(300*time.Millisecond),
	)
	if err != nil {
		t.Fatal(err)
	}

	visible, err := isVisible(ctx, ".nav-dropdown")
	if err != nil {
		t.Fatal(err)
	}
	if !visible {
		t.Error("nav-dropdown should be visible after hamburger click")
	}
}

func TestNavLinksPresent(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatal(err)
	}

	count, err := elementCount(ctx, ".nav-links li")
	if err != nil {
		t.Fatal(err)
	}
	if count < 1 {
		t.Errorf("nav-links items = %d, want >= 1", count)
	}
}

func TestNavSettingsInDropdown(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatal(err)
	}

	err = chromedp.Run(ctx,
		chromedp.Click(".nav-hamburger", chromedp.ByQuery),
		chromedp.Sleep(300*time.Millisecond),
	)
	if err != nil {
		t.Fatal(err)
	}

	exists, err := common.Exists(ctx, ".nav-dropdown a[href='/settings']")
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Error("settings link not found in dropdown")
	}
}

func TestNavLogoutInDropdown(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatal(err)
	}

	err = chromedp.Run(ctx,
		chromedp.Click(".nav-hamburger", chromedp.ByQuery),
		chromedp.Sleep(300*time.Millisecond),
	)
	if err != nil {
		t.Fatal(err)
	}

	visible, err := isVisible(ctx, ".nav-dropdown-logout")
	if err != nil {
		t.Fatal(err)
	}
	if !visible {
		t.Error("logout button not visible in dropdown")
	}
}

func TestNavMobileNavLinksHidden(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatal(err)
	}

	err = chromedp.Run(ctx,
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

func TestNavMobileHamburgerVisible(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatal(err)
	}

	err = chromedp.Run(ctx,
		chromedp.EmulateViewport(375, 812),
		chromedp.Navigate(serverURL()+"/dashboard"),
		chromedp.WaitVisible(".nav", chromedp.ByQuery),
		chromedp.Sleep(300*time.Millisecond),
	)
	if err != nil {
		t.Fatal(err)
	}

	visible, err := isVisible(ctx, ".nav-hamburger")
	if err != nil {
		t.Fatal(err)
	}
	if !visible {
		t.Error("nav-hamburger should be visible on mobile viewport")
	}
}

func TestNavMobileMenuClosedOnLoad(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatal(err)
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

	hidden, err := isHidden(ctx, ".mobile-menu")
	if err != nil {
		t.Fatal(err)
	}
	if !hidden {
		t.Error("mobile menu is visible on page load — should be closed")
	}
}

func TestNavMobileMenuOpensCloses(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatal(err)
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

	visible, err := isVisible(ctx, ".mobile-menu")
	if err != nil {
		t.Fatal(err)
	}
	if !visible {
		t.Error("mobile menu did not open on hamburger click")
	}

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

func TestNavDesktopLinksVisible(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatal(err)
	}

	err = chromedp.Run(ctx,
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
