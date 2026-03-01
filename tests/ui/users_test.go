package tests

import (
	"strings"
	"testing"

	commontest "github.com/bobmcallan/vire-portal/tests/common"
)

func TestUsersPageLayout(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/admin/users")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	takeScreenshot(t, ctx, "users", "page-layout.png")

	// Check .page class for flex layout
	pageVisible, err := isVisible(ctx, ".page")
	if err != nil {
		t.Fatalf("error checking page visibility: %v", err)
	}
	if !pageVisible {
		t.Fatal(".page class not found")
	}

	// Check .page-body class for max-width container
	bodyVisible, err := isVisible(ctx, ".page-body")
	if err != nil {
		t.Fatalf("error checking page-body visibility: %v", err)
	}
	if !bodyVisible {
		t.Fatal(".page-body class not found")
	}
}

func TestUsersPageNavVisible(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/admin/users")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	takeScreenshot(t, ctx, "users", "nav-visible.png")

	navVisible, err := isVisible(ctx, ".nav")
	if err != nil {
		t.Fatalf("error checking nav visibility: %v", err)
	}
	if !navVisible {
		t.Fatal("nav not visible on users page")
	}
}

func TestUsersPagePanelHeader(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/admin/users")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	takeScreenshot(t, ctx, "users", "panel-header.png")

	headerExists, err := commontest.EvalBool(ctx, `
		(() => {
			const headers = document.querySelectorAll('.panel-header');
			return Array.from(headers).some(h => h.textContent.includes('USERS'));
		})()
	`)
	if err != nil {
		t.Fatalf("error checking panel header: %v", err)
	}
	if !headerExists {
		t.Fatal("panel header does not contain 'USERS'")
	}
}

func TestUsersPageTableHeaders(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/admin/users")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	takeScreenshot(t, ctx, "users", "table-headers.png")

	tableVisible, err := isVisible(ctx, ".tool-table")
	if err != nil {
		t.Fatalf("error checking table visibility: %v", err)
	}
	if !tableVisible {
		t.Skip("users table not visible (no users data available)")
	}

	// Verify expected table headers
	headersCorrect, err := commontest.EvalBool(ctx, `
		(() => {
			const headers = document.querySelectorAll('.tool-table thead th');
			if (headers.length !== 5) return false;
			const expected = ['Email', 'Name', 'Role', 'Provider', 'Joined'];
			for (let i = 0; i < 5; i++) {
				if (!headers[i].textContent.includes(expected[i])) return false;
			}
			return true;
		})()
	`)
	if err != nil {
		t.Fatalf("error checking table headers: %v", err)
	}
	if !headersCorrect {
		t.Fatal("table headers do not match expected: Email, Name, Role, Provider, Joined")
	}
}

func TestUsersPageFooterVisible(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/admin/users")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	takeScreenshot(t, ctx, "users", "footer-visible.png")

	footerVisible, err := isVisible(ctx, ".footer")
	if err != nil {
		t.Fatalf("error checking footer visibility: %v", err)
	}
	if !footerVisible {
		t.Fatal("footer not visible on users page")
	}
}

func TestUsersPageNoJSErrors(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	errs := newJSErrorCollector(ctx)
	err := loginAndNavigate(ctx, serverURL()+"/admin/users")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	takeScreenshot(t, ctx, "users", "no-js-errors.png")

	if jsErrs := errs.Errors(); len(jsErrs) > 0 {
		t.Errorf("JS errors on users page:\n  %s", strings.Join(jsErrs, "\n  "))
	}
}
