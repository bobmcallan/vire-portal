package tests

import (
	"strings"
	"testing"

	commontest "github.com/bobmcallan/vire-portal/tests/common"
	"github.com/chromedp/chromedp"
)

func TestMCPPageLoads(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/mcp-info")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	visible, err := isVisible(ctx, ".page")
	if err != nil {
		t.Fatalf("error checking page visibility: %v", err)
	}
	if !visible {
		t.Fatal("MCP page .page not visible")
	}
}

func TestMCPConnectionSection(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/mcp-info")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	found, err := commontest.EvalBool(ctx, `
		(() => {
			const headers = document.querySelectorAll('.panel-header');
			return Array.from(headers).some(h => h.textContent.includes('MCP CONNECTION'));
		})()
	`)
	if err != nil {
		t.Fatalf("error checking MCP CONNECTION header: %v", err)
	}
	if !found {
		t.Fatal("MCP CONNECTION panel header not found")
	}
}

func TestMCPToolsTable(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/mcp-info")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	count, err := elementCount(ctx, ".tool-table")
	if err != nil {
		t.Fatalf("error checking tools table: %v", err)
	}
	if count < 1 {
		t.Fatal("tools table (.tool-table) not found")
	}

	rows, err := elementCount(ctx, ".tool-table tbody tr")
	if err != nil {
		t.Fatalf("error counting tool rows: %v", err)
	}
	if rows < 1 {
		t.Skip("tools table has no rows (vire-server may not be running)")
	}
}

func TestMCPNoJSErrors(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	errs := newJSErrorCollector(ctx)
	err := loginAndNavigate(ctx, serverURL()+"/mcp-info")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	if jsErrs := errs.Errors(); len(jsErrs) > 0 {
		t.Errorf("JS errors on MCP page:\n  %s", strings.Join(jsErrs, "\n  "))
	}
}

func TestMCPEndpointURL(t *testing.T) {
	ctx, cancel := newBrowser(t)
	defer cancel()

	err := loginAndNavigate(ctx, serverURL()+"/mcp-info")
	if err != nil {
		t.Fatalf("login and navigate failed: %v", err)
	}

	var bodyText string
	err = chromedp.Run(ctx, chromedp.Evaluate(`document.body.innerText`, &bodyText))
	if err != nil {
		t.Fatalf("error getting body text: %v", err)
	}

	if !strings.Contains(bodyText, "/mcp") {
		t.Fatal("page does not contain /mcp endpoint text")
	}
}
