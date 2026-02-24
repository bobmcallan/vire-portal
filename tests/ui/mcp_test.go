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

	takeScreenshot(t, ctx, "mcp", "page-loads.png")

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

	takeScreenshot(t, ctx, "mcp", "connection-section.png")

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

	takeScreenshot(t, ctx, "mcp", "tools-table.png")

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

	takeScreenshot(t, ctx, "mcp", "no-js-errors.png")

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

	takeScreenshot(t, ctx, "mcp", "endpoint-url.png")

	// Verify the rendered MCPEndpoint value in the connection section <code> element.
	// Template: <p>Endpoint: <code>{{.MCPEndpoint}}</code></p>
	var endpointText string
	err = chromedp.Run(ctx, chromedp.Evaluate(`
		(() => {
			const section = document.querySelector('.panel-headed .panel-content');
			if (!section) return '';
			const code = section.querySelector('code');
			return code ? code.textContent.trim() : '';
		})()
	`, &endpointText))
	if err != nil {
		t.Fatalf("error getting endpoint text: %v", err)
	}

	if endpointText == "" {
		t.Fatal("MCP endpoint <code> element is empty or not found")
	}

	if !strings.Contains(endpointText, "/mcp") {
		t.Errorf("MCP endpoint = %q, want contains '/mcp'", endpointText)
	}
}
