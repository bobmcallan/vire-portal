package tests

import (
	"testing"
	"time"

	"github.com/bobmcallan/vire-portal/tests/common"
	"github.com/chromedp/chromedp"
)

func TestNav(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/dashboard")
	if err != nil {
		t.Fatal(err)
	}

	// --- Read-only desktop assertions (no clicks, no viewport changes) ---

	t.Run("BrandText", func(t *testing.T) {
		takeScreenshot(t, ctx, "nav", "brand-text.png")

		containsBrand, brand, err := common.TextContains(ctx, ".nav-brand", "VIRE")
		if err != nil {
			t.Fatal(err)
		}
		if !containsBrand {
			t.Errorf("nav-brand = %q, want contains VIRE", brand)
		}
	})

	t.Run("BrandHref", func(t *testing.T) {
		takeScreenshot(t, ctx, "nav", "brand-href.png")

		href, err := common.EvalBool(ctx, `
			(() => {
				const a = document.querySelector('.nav-brand');
				return a && a.getAttribute('href') === '/dashboard';
			})()
		`)
		if err != nil {
			t.Fatal(err)
		}
		if !href {
			t.Error("nav-brand href should be /dashboard (not /)")
		}
	})

	t.Run("HamburgerVisible", func(t *testing.T) {
		takeScreenshot(t, ctx, "nav", "hamburger-visible.png")

		visible, err := isVisible(ctx, ".nav-hamburger")
		if err != nil {
			t.Fatal(err)
		}
		if !visible {
			t.Error("nav-hamburger not visible on desktop")
		}
	})

	t.Run("DropdownHiddenByDefault", func(t *testing.T) {
		takeScreenshot(t, ctx, "nav", "dropdown-hidden.png")

		hidden, err := isHidden(ctx, ".nav-dropdown")
		if err != nil {
			t.Fatal(err)
		}
		if !hidden {
			t.Error("nav-dropdown should be hidden by default")
		}
	})

	t.Run("LinksPresent", func(t *testing.T) {
		takeScreenshot(t, ctx, "nav", "links-present.png")

		count, err := elementCount(ctx, ".nav-links li")
		if err != nil {
			t.Fatal(err)
		}
		if count < 1 {
			t.Errorf("nav-links items = %d, want >= 1", count)
		}
	})

	t.Run("StrategyLinkPresent", func(t *testing.T) {
		takeScreenshot(t, ctx, "nav", "strategy-link-present.png")

		exists, err := common.Exists(ctx, `.nav-links a[href="/strategy"]`)
		if err != nil {
			t.Fatal(err)
		}
		if !exists {
			t.Error("Strategy link (a[href='/strategy']) not found in .nav-links")
		}
	})

	t.Run("MCPLinkPresent", func(t *testing.T) {
		takeScreenshot(t, ctx, "nav", "mcp-link-present.png")

		exists, err := common.Exists(ctx, `.nav-links a[href="/mcp-info"]`)
		if err != nil {
			t.Fatal(err)
		}
		if !exists {
			t.Error("MCP link (a[href='/mcp-info']) not found in .nav-links")
		}
	})

	t.Run("HelpLinkPresent", func(t *testing.T) {
		takeScreenshot(t, ctx, "nav", "help-link-present.png")

		exists, err := common.Exists(ctx, `.nav-links a[href="/help"]`)
		if err != nil {
			t.Fatal(err)
		}
		if !exists {
			t.Error("Help link (a[href='/help']) not found in .nav-links")
		}
	})

	// --- Desktop dropdown tests (click hamburger — navigate to reset state) ---

	t.Run("DropdownOpensOnClick", func(t *testing.T) {
		err := chromedp.Run(ctx,
			chromedp.Navigate(serverURL()+"/dashboard"),
			chromedp.WaitVisible(".nav", chromedp.ByQuery),
		)
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

		takeScreenshot(t, ctx, "nav", "dropdown-opens.png")

		visible, err := isVisible(ctx, ".nav-dropdown")
		if err != nil {
			t.Fatal(err)
		}
		if !visible {
			t.Error("nav-dropdown should be visible after hamburger click")
		}
	})

	t.Run("ProfileInDropdown", func(t *testing.T) {
		err := chromedp.Run(ctx,
			chromedp.Navigate(serverURL()+"/dashboard"),
			chromedp.WaitVisible(".nav", chromedp.ByQuery),
		)
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

		takeScreenshot(t, ctx, "nav", "profile-in-dropdown.png")

		exists, err := common.Exists(ctx, ".nav-dropdown a[href='/profile']")
		if err != nil {
			t.Fatal(err)
		}
		if !exists {
			t.Error("profile link not found in dropdown")
		}
	})

	t.Run("LogoutInDropdown", func(t *testing.T) {
		err := chromedp.Run(ctx,
			chromedp.Navigate(serverURL()+"/dashboard"),
			chromedp.WaitVisible(".nav", chromedp.ByQuery),
		)
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

		takeScreenshot(t, ctx, "nav", "logout-in-dropdown.png")

		visible, err := isVisible(ctx, ".nav-dropdown-logout")
		if err != nil {
			t.Fatal(err)
		}
		if !visible {
			t.Error("logout button not visible in dropdown")
		}
	})

	t.Run("DropdownPromptsLink", func(t *testing.T) {
		err := chromedp.Run(ctx,
			chromedp.Navigate(serverURL()+"/dashboard"),
			chromedp.WaitVisible(".nav", chromedp.ByQuery),
		)
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

		takeScreenshot(t, ctx, "nav", "dropdown-prompts-link.png")

		exists, err := common.Exists(ctx, `.nav-dropdown a[href="/admin/prompts"]`)
		if err != nil {
			t.Fatal(err)
		}
		if !exists {
			t.Skip("Prompts link not found in dropdown (user may not have admin role)")
		}
	})

	t.Run("DropdownUsersLink", func(t *testing.T) {
		err := chromedp.Run(ctx,
			chromedp.Navigate(serverURL()+"/dashboard"),
			chromedp.WaitVisible(".nav", chromedp.ByQuery),
		)
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

		takeScreenshot(t, ctx, "nav", "dropdown-users-link.png")

		exists, err := common.Exists(ctx, `.nav-dropdown a[href="/admin/users"]`)
		if err != nil {
			t.Fatal(err)
		}
		if !exists {
			t.Skip("Users link not found in dropdown (user may not have admin role)")
		}
	})

	t.Run("DropdownHelpLink", func(t *testing.T) {
		err := chromedp.Run(ctx,
			chromedp.Navigate(serverURL()+"/dashboard"),
			chromedp.WaitVisible(".nav", chromedp.ByQuery),
		)
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

		takeScreenshot(t, ctx, "nav", "dropdown-help-link.png")

		exists, err := common.Exists(ctx, `.nav-dropdown a[href="/help"]`)
		if err != nil {
			t.Fatal(err)
		}
		if !exists {
			t.Error("Help link (a[href='/help']) not found in .nav-dropdown")
		}
	})

	// --- Desktop viewport test ---

	t.Run("DesktopLinksVisible", func(t *testing.T) {
		err := chromedp.Run(ctx,
			chromedp.EmulateViewport(1280, 800),
			chromedp.Navigate(serverURL()+"/dashboard"),
			chromedp.WaitVisible(".nav", chromedp.ByQuery),
			chromedp.Sleep(300*time.Millisecond),
		)
		if err != nil {
			t.Fatal(err)
		}

		takeScreenshot(t, ctx, "nav", "desktop-links-visible.png")

		visible, err := isVisible(ctx, ".nav-links")
		if err != nil {
			t.Fatal(err)
		}
		if !visible {
			t.Error("nav-links should be visible on desktop viewport")
		}
	})

	// --- Mobile viewport tests (set viewport + re-navigate) ---

	t.Run("MobileNavLinksHidden", func(t *testing.T) {
		err := chromedp.Run(ctx,
			chromedp.EmulateViewport(375, 812),
			chromedp.Navigate(serverURL()+"/dashboard"),
			chromedp.WaitVisible(".nav", chromedp.ByQuery),
			chromedp.Sleep(300*time.Millisecond),
		)
		if err != nil {
			t.Fatal(err)
		}

		takeScreenshot(t, ctx, "nav", "mobile-links-hidden.png")

		hidden, err := isHidden(ctx, ".nav-links")
		if err != nil {
			t.Fatal(err)
		}
		if !hidden {
			t.Error("nav-links should be hidden on mobile viewport")
		}
	})

	t.Run("MobileHamburgerVisible", func(t *testing.T) {
		err := chromedp.Run(ctx,
			chromedp.EmulateViewport(375, 812),
			chromedp.Navigate(serverURL()+"/dashboard"),
			chromedp.WaitVisible(".nav", chromedp.ByQuery),
			chromedp.Sleep(300*time.Millisecond),
		)
		if err != nil {
			t.Fatal(err)
		}

		takeScreenshot(t, ctx, "nav", "mobile-hamburger-visible.png")

		visible, err := isVisible(ctx, ".nav-hamburger")
		if err != nil {
			t.Fatal(err)
		}
		if !visible {
			t.Error("nav-hamburger should be visible on mobile viewport")
		}
	})

	t.Run("MobileMenuClosedOnLoad", func(t *testing.T) {
		err := chromedp.Run(ctx,
			chromedp.EmulateViewport(375, 812),
			chromedp.Navigate(serverURL()+"/dashboard"),
			chromedp.WaitVisible("body", chromedp.ByQuery),
			chromedp.Sleep(800*time.Millisecond),
		)
		if err != nil {
			t.Fatal(err)
		}

		takeScreenshot(t, ctx, "nav", "mobile-menu-closed.png")

		hidden, err := isHidden(ctx, ".mobile-menu")
		if err != nil {
			t.Fatal(err)
		}
		if !hidden {
			t.Error("mobile menu is visible on page load — should be closed")
		}
	})

	t.Run("MobileHelpLink", func(t *testing.T) {
		err := chromedp.Run(ctx,
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

		takeScreenshot(t, ctx, "nav", "mobile-help-link.png")

		exists, err := common.Exists(ctx, `.mobile-menu a[href="/help"]`)
		if err != nil {
			t.Fatal(err)
		}
		if !exists {
			t.Error("Help link (a[href='/help']) not found in .mobile-menu")
		}
	})

	t.Run("MobilePromptsLink", func(t *testing.T) {
		err := chromedp.Run(ctx,
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

		takeScreenshot(t, ctx, "nav", "mobile-prompts-link.png")

		exists, err := common.Exists(ctx, `.mobile-menu a[href="/admin/prompts"]`)
		if err != nil {
			t.Fatal(err)
		}
		if !exists {
			t.Skip("Prompts link not found in mobile menu (user may not have admin role)")
		}
	})

	t.Run("MobileUsersLink", func(t *testing.T) {
		err := chromedp.Run(ctx,
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

		takeScreenshot(t, ctx, "nav", "mobile-users-link.png")

		exists, err := common.Exists(ctx, `.mobile-menu a[href="/admin/users"]`)
		if err != nil {
			t.Fatal(err)
		}
		if !exists {
			t.Skip("Users link not found in mobile menu (user may not have admin role)")
		}
	})

	// --- State-modifying mobile tests (open/close menu) LAST ---

	t.Run("MobileMenuOpensCloses", func(t *testing.T) {
		err := chromedp.Run(ctx,
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

		takeScreenshot(t, ctx, "nav", "mobile-menu-open.png")

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

		takeScreenshot(t, ctx, "nav", "mobile-menu-closed-after.png")

		hidden, err := isHidden(ctx, ".mobile-menu")
		if err != nil {
			t.Fatal(err)
		}
		if !hidden {
			t.Error("mobile menu did not close")
		}
	})
}
