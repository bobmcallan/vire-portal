package tests

import (
	"strings"
	"testing"
	"time"

	commontest "github.com/bobmcallan/vire-portal/tests/common"
	"github.com/chromedp/chromedp"
)

func TestPrompts(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	errs := newJSErrorCollector(ctx)
	err := loginAndNavigate(ctx, serverURL()+"/admin/prompts")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	_ = chromedp.Run(ctx, chromedp.Sleep(2*time.Second))

	t.Run("NoJSErrors", func(t *testing.T) {
		takeScreenshot(t, ctx, "prompts", "no-js-errors.png")

		if jsErrs := errs.Errors(); len(jsErrs) > 0 {
			t.Errorf("JS errors on prompts page:\n  %s", strings.Join(jsErrs, "\n  "))
		}
	})

	t.Run("PageLayout", func(t *testing.T) {
		takeScreenshot(t, ctx, "prompts", "page-layout.png")

		pageVisible, err := isVisible(ctx, ".page")
		if err != nil {
			t.Fatalf("error checking page visibility: %v", err)
		}
		if !pageVisible {
			t.Fatal(".page class not found")
		}

		bodyVisible, err := isVisible(ctx, ".page-body")
		if err != nil {
			t.Fatalf("error checking page-body visibility: %v", err)
		}
		if !bodyVisible {
			t.Fatal(".page-body class not found")
		}
	})

	t.Run("PageNavVisible", func(t *testing.T) {
		takeScreenshot(t, ctx, "prompts", "nav-visible.png")

		navVisible, err := isVisible(ctx, ".nav")
		if err != nil {
			t.Fatalf("error checking nav visibility: %v", err)
		}
		if !navVisible {
			t.Fatal("nav not visible on prompts page")
		}
	})

	t.Run("PagePanelHeader", func(t *testing.T) {
		takeScreenshot(t, ctx, "prompts", "panel-header.png")

		panelVisible, err := isVisible(ctx, ".panel-headed")
		if err != nil {
			t.Fatalf("error checking panel visibility: %v", err)
		}
		if !panelVisible {
			t.Skip("prompts panel not found (user may not have admin role, redirected to dashboard)")
		}

		headerExists, err := commontest.EvalBool(ctx, `
			(() => {
				const headers = document.querySelectorAll('.panel-header');
				return Array.from(headers).some(h => h.textContent.includes('PROMPTS'));
			})()
		`)
		if err != nil {
			t.Fatalf("error checking panel header: %v", err)
		}
		if !headerExists {
			t.Error("panel header does not contain 'PROMPTS'")
		}
	})

	t.Run("PageTableHeaders", func(t *testing.T) {
		takeScreenshot(t, ctx, "prompts", "table-headers.png")

		panelVisible, err := isVisible(ctx, ".panel-headed")
		if err != nil {
			t.Fatalf("error checking panel visibility: %v", err)
		}
		if !panelVisible {
			t.Skip("prompts panel not found (user may not have admin role, redirected to dashboard)")
		}

		tableVisible, err := isVisible(ctx, ".tool-table")
		if err != nil {
			t.Fatalf("error checking table visibility: %v", err)
		}
		if !tableVisible {
			t.Skip("prompts table not visible (no prompts data available)")
		}

		headersCorrect, err := commontest.EvalBool(ctx, `
			(() => {
				const headers = document.querySelectorAll('.tool-table thead th');
				if (headers.length !== 4) return false;
				const expected = ['Name', 'Description', 'Override', 'Updated'];
				for (let i = 0; i < 4; i++) {
					if (!headers[i].textContent.includes(expected[i])) return false;
				}
				return true;
			})()
		`)
		if err != nil {
			t.Fatalf("error checking table headers: %v", err)
		}
		if !headersCorrect {
			t.Error("table headers do not match expected: Name, Description, Override, Updated")
		}
	})

	t.Run("PageFooterVisible", func(t *testing.T) {
		takeScreenshot(t, ctx, "prompts", "footer-visible.png")

		footerVisible, err := isVisible(ctx, ".footer")
		if err != nil {
			t.Fatalf("error checking footer visibility: %v", err)
		}
		if !footerVisible {
			t.Fatal("footer not visible on prompts page")
		}
	})

	// Edit page subtests navigate away from /admin/prompts -- keep last
	t.Run("EditPageLayout", func(t *testing.T) {
		// Navigate back to prompts list first
		err := chromedp.Run(ctx,
			chromedp.Navigate(serverURL()+"/admin/prompts"),
			chromedp.WaitVisible(".page-body", chromedp.ByQuery),
			chromedp.Sleep(1*time.Second),
		)
		if err != nil {
			t.Fatalf("error navigating to prompts list: %v", err)
		}

		// Find the first prompt link to navigate to an edit page
		hasLink, err := commontest.EvalBool(ctx, `
			(() => {
				const link = document.querySelector('.tool-table tbody a');
				return !!link;
			})()
		`)
		if err != nil {
			t.Fatalf("error checking for prompt links: %v", err)
		}
		if !hasLink {
			t.Skip("no prompt links found in table (no prompts data available)")
		}

		// Click the first prompt link
		err = chromedp.Run(ctx,
			chromedp.Click(".tool-table tbody a", chromedp.ByQuery),
			chromedp.WaitVisible(".page-body", chromedp.ByQuery),
			chromedp.Sleep(1*time.Second),
		)
		if err != nil {
			t.Fatalf("error navigating to prompt edit: %v", err)
		}

		takeScreenshot(t, ctx, "prompts", "edit-page-layout.png")

		pageVisible, err := isVisible(ctx, ".page")
		if err != nil {
			t.Fatalf("error checking page visibility: %v", err)
		}
		if !pageVisible {
			t.Fatal(".page class not found on edit page")
		}

		panelVisible, err := isVisible(ctx, ".panel-headed")
		if err != nil {
			t.Fatalf("error checking panel visibility: %v", err)
		}
		if !panelVisible {
			t.Fatal(".panel-headed not found on edit page")
		}

		headerExists, err := commontest.EvalBool(ctx, `
			(() => {
				const headers = document.querySelectorAll('.panel-header');
				return Array.from(headers).some(h => h.textContent.includes('PROMPT:'));
			})()
		`)
		if err != nil {
			t.Fatalf("error checking panel header: %v", err)
		}
		if !headerExists {
			t.Error("panel header does not contain 'PROMPT:'")
		}
	})

	t.Run("EditPageElements", func(t *testing.T) {
		// Navigate back to prompts list first
		err := chromedp.Run(ctx,
			chromedp.Navigate(serverURL()+"/admin/prompts"),
			chromedp.WaitVisible(".page-body", chromedp.ByQuery),
			chromedp.Sleep(1*time.Second),
		)
		if err != nil {
			t.Fatalf("error navigating to prompts list: %v", err)
		}

		hasLink, err := commontest.EvalBool(ctx, `!!document.querySelector('.tool-table tbody a')`)
		if err != nil {
			t.Fatalf("error checking for prompt links: %v", err)
		}
		if !hasLink {
			t.Skip("no prompt links found in table (no prompts data available)")
		}

		err = chromedp.Run(ctx,
			chromedp.Click(".tool-table tbody a", chromedp.ByQuery),
			chromedp.WaitVisible(".page-body", chromedp.ByQuery),
			chromedp.Sleep(1*time.Second),
		)
		if err != nil {
			t.Fatalf("error navigating to prompt edit: %v", err)
		}

		takeScreenshot(t, ctx, "prompts", "edit-page-elements.png")

		// Check back link
		backLink, err := commontest.EvalBool(ctx, `!!document.querySelector('a[href="/admin/prompts"]')`)
		if err != nil {
			t.Fatalf("error checking back link: %v", err)
		}
		if !backLink {
			t.Error("back link to /admin/prompts not found")
		}

		// Check textarea
		textareaVisible, err := isVisible(ctx, ".prompt-textarea")
		if err != nil {
			t.Fatalf("error checking textarea visibility: %v", err)
		}
		if !textareaVisible {
			t.Error("prompt-textarea not visible")
		}

		// Check save button
		saveVisible, err := isVisible(ctx, ".btn-save")
		if err != nil {
			t.Fatalf("error checking save button visibility: %v", err)
		}
		if !saveVisible {
			t.Error("btn-save not visible")
		}

		// Check prompt meta section
		metaVisible, err := isVisible(ctx, ".prompt-meta")
		if err != nil {
			t.Fatalf("error checking meta visibility: %v", err)
		}
		if !metaVisible {
			t.Error("prompt-meta not visible")
		}
	})

	t.Run("EditPageNoJSErrors", func(t *testing.T) {
		// Navigate back to prompts list first
		editErrs := newJSErrorCollector(ctx)

		err := chromedp.Run(ctx,
			chromedp.Navigate(serverURL()+"/admin/prompts"),
			chromedp.WaitVisible(".page-body", chromedp.ByQuery),
			chromedp.Sleep(1*time.Second),
		)
		if err != nil {
			t.Fatalf("error navigating to prompts list: %v", err)
		}

		hasLink, err := commontest.EvalBool(ctx, `!!document.querySelector('.tool-table tbody a')`)
		if err != nil {
			t.Fatalf("error checking for prompt links: %v", err)
		}
		if !hasLink {
			t.Skip("no prompt links found in table (no prompts data available)")
		}

		err = chromedp.Run(ctx,
			chromedp.Click(".tool-table tbody a", chromedp.ByQuery),
			chromedp.WaitVisible(".page-body", chromedp.ByQuery),
			chromedp.Sleep(2*time.Second),
		)
		if err != nil {
			t.Fatalf("error navigating to prompt edit: %v", err)
		}

		takeScreenshot(t, ctx, "prompts", "edit-no-js-errors.png")

		if jsErrs := editErrs.Errors(); len(jsErrs) > 0 {
			t.Errorf("JS errors on prompt edit page:\n  %s", strings.Join(jsErrs, "\n  "))
		}
	})
}
