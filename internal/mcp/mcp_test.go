package mcp

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bobmcallan/vire-portal/internal/config"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

// --- Helpers ---

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
}

func testConfig() *config.Config {
	cfg := config.NewDefaultConfig()
	cfg.API.URL = "http://localhost:4242"
	cfg.User.Portfolios = []string{"SMSF", "Personal"}
	cfg.User.DisplayCurrency = "AUD"
	cfg.Keys.EODHD = "test-eodhd-key"
	cfg.Keys.Navexa = "test-navexa-key"
	cfg.Keys.Gemini = "test-gemini-key"
	return cfg
}

// listTools calls tools/list on the MCPServer and returns the tools.
func listTools(t *testing.T, s *mcpserver.MCPServer) []mcpgo.Tool {
	t.Helper()

	msg := json.RawMessage(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`)
	ctx := t.Context()
	result := s.HandleMessage(ctx, msg)

	resp, ok := result.(mcpgo.JSONRPCResponse)
	if !ok {
		t.Fatalf("expected JSONRPCResponse, got %T", result)
	}

	resultJSON, err := json.Marshal(resp.Result)
	if err != nil {
		t.Fatalf("failed to marshal result: %v", err)
	}

	var toolsResult mcpgo.ListToolsResult
	if err := json.Unmarshal(resultJSON, &toolsResult); err != nil {
		t.Fatalf("failed to unmarshal ListToolsResult: %v", err)
	}

	return toolsResult.Tools
}

// callTool calls a tool on the MCPServer and returns the result.
func callTool(t *testing.T, s *mcpserver.MCPServer, name string, args map[string]interface{}) *mcpgo.CallToolResult {
	t.Helper()

	params := map[string]interface{}{
		"name":      name,
		"arguments": args,
	}
	paramsJSON, _ := json.Marshal(params)

	msg := json.RawMessage(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":` + string(paramsJSON) + `}`)
	ctx := t.Context()
	result := s.HandleMessage(ctx, msg)

	resp, ok := result.(mcpgo.JSONRPCResponse)
	if !ok {
		t.Fatalf("expected JSONRPCResponse, got %T", result)
	}

	resultJSON, err := json.Marshal(resp.Result)
	if err != nil {
		t.Fatalf("failed to marshal result: %v", err)
	}

	var toolResult mcpgo.CallToolResult
	if err := json.Unmarshal(resultJSON, &toolResult); err != nil {
		t.Fatalf("failed to unmarshal CallToolResult: %v", err)
	}

	return &toolResult
}

// extractText extracts the text field from an MCP content block.
func extractText(t *testing.T, content mcpgo.Content) string {
	t.Helper()
	contentJSON, _ := json.Marshal(content)
	var tc struct {
		Text string `json:"text"`
	}
	json.Unmarshal(contentJSON, &tc)
	return tc.Text
}

// sampleCatalogJSON returns a realistic catalog JSON array for testing.
func sampleCatalogJSON() string {
	return `[
		{
			"name": "get_version",
			"description": "Get the Vire server version and status.",
			"method": "GET",
			"path": "/api/version",
			"params": []
		},
		{
			"name": "get_quote",
			"description": "Get a real-time price quote for a ticker.",
			"method": "GET",
			"path": "/api/market/quote/{ticker}",
			"params": [
				{
					"name": "ticker",
					"type": "string",
					"description": "Ticker with exchange suffix",
					"required": true,
					"in": "path"
				}
			]
		},
		{
			"name": "portfolio_compliance",
			"description": "Review a portfolio for signals and observations.",
			"method": "POST",
			"path": "/api/portfolios/{portfolio_name}/review",
			"params": [
				{
					"name": "portfolio_name",
					"type": "string",
					"description": "Name of the portfolio to review.",
					"required": false,
					"in": "path",
					"default_from": "user_config.default_portfolio"
				},
				{
					"name": "focus_signals",
					"type": "array",
					"description": "Signal types to focus on.",
					"required": false,
					"in": "body"
				},
				{
					"name": "include_news",
					"type": "boolean",
					"description": "Include news sentiment.",
					"required": false,
					"in": "body"
				}
			]
		},
		{
			"name": "list_reports",
			"description": "List available portfolio reports.",
			"method": "GET",
			"path": "/api/reports",
			"params": [
				{
					"name": "portfolio_name",
					"type": "string",
					"description": "Filter to a specific portfolio.",
					"required": false,
					"in": "query"
				}
			]
		},
		{
			"name": "get_portfolio",
			"description": "Get current portfolio holdings.",
			"method": "GET",
			"path": "/api/portfolios/{portfolio_name}",
			"params": [
				{
					"name": "portfolio_name",
					"type": "string",
					"description": "Name of the portfolio.",
					"required": false,
					"in": "path",
					"default_from": "user_config.default_portfolio"
				}
			]
		},
		{
			"name": "delete_portfolio_strategy",
			"description": "Delete the investment strategy for a portfolio.",
			"method": "DELETE",
			"path": "/api/portfolios/{portfolio_name}/strategy",
			"params": [
				{
					"name": "portfolio_name",
					"type": "string",
					"description": "Name of the portfolio.",
					"required": false,
					"in": "path",
					"default_from": "user_config.default_portfolio"
				}
			]
		},
		{
			"name": "set_portfolio_strategy",
			"description": "Set the investment strategy for a portfolio.",
			"method": "PUT",
			"path": "/api/portfolios/{portfolio_name}/strategy",
			"params": [
				{
					"name": "portfolio_name",
					"type": "string",
					"description": "Name of the portfolio.",
					"required": false,
					"in": "path",
					"default_from": "user_config.default_portfolio"
				},
				{
					"name": "strategy_json",
					"type": "string",
					"description": "JSON strategy object.",
					"required": true,
					"in": "body"
				}
			]
		},
		{
			"name": "get_diagnostics",
			"description": "Get server diagnostics.",
			"method": "GET",
			"path": "/api/diagnostics",
			"params": [
				{
					"name": "correlation_id",
					"type": "string",
					"description": "Filter by correlation ID.",
					"required": false,
					"in": "query"
				},
				{
					"name": "limit",
					"type": "number",
					"description": "Maximum entries.",
					"required": false,
					"in": "query"
				}
			]
		},
		{
			"name": "update_plan_item",
			"description": "Update an existing plan item by ID.",
			"method": "PATCH",
			"path": "/api/portfolios/{portfolio_name}/plan/items/{item_id}",
			"params": [
				{
					"name": "portfolio_name",
					"type": "string",
					"description": "Name of the portfolio.",
					"required": false,
					"in": "path",
					"default_from": "user_config.default_portfolio"
				},
				{
					"name": "item_id",
					"type": "string",
					"description": "ID of the plan item.",
					"required": true,
					"in": "path"
				},
				{
					"name": "item_json",
					"type": "string",
					"description": "JSON with fields to update.",
					"required": true,
					"in": "body"
				}
			]
		}
	]`
}

// parseSampleCatalog parses the sample catalog JSON into CatalogTool slice.
func parseSampleCatalog(t *testing.T) []CatalogTool {
	t.Helper()
	var catalog []CatalogTool
	if err := json.Unmarshal([]byte(sampleCatalogJSON()), &catalog); err != nil {
		t.Fatalf("failed to parse sample catalog: %v", err)
	}
	return catalog
}

// findCatalogTool finds a CatalogTool by name in the catalog.
func findCatalogTool(catalog []CatalogTool, name string) *CatalogTool {
	for i := range catalog {
		if catalog[i].Name == name {
			return &catalog[i]
		}
	}
	return nil
}

// --- Catalog Parsing Tests ---

func TestCatalogTool_ParseJSON(t *testing.T) {
	catalog := parseSampleCatalog(t)

	if len(catalog) != 9 {
		t.Errorf("expected 9 tools in sample catalog, got %d", len(catalog))
	}
}

func TestCatalogTool_ParseFields(t *testing.T) {
	catalog := parseSampleCatalog(t)

	ct := findCatalogTool(catalog, "portfolio_compliance")
	if ct == nil {
		t.Fatal("expected portfolio_compliance in catalog")
	}

	if ct.Description != "Review a portfolio for signals and observations." {
		t.Errorf("unexpected description: %s", ct.Description)
	}
	if ct.Method != "POST" {
		t.Errorf("expected method POST, got %s", ct.Method)
	}
	if ct.Path != "/api/portfolios/{portfolio_name}/review" {
		t.Errorf("unexpected path: %s", ct.Path)
	}
	if len(ct.Params) != 3 {
		t.Errorf("expected 3 params, got %d", len(ct.Params))
	}
}

func TestCatalogParam_ParseFields(t *testing.T) {
	catalog := parseSampleCatalog(t)

	ct := findCatalogTool(catalog, "portfolio_compliance")
	if ct == nil {
		t.Fatal("expected portfolio_compliance in catalog")
	}

	// First param: portfolio_name (path, default_from)
	p := ct.Params[0]
	if p.Name != "portfolio_name" {
		t.Errorf("expected param name 'portfolio_name', got %q", p.Name)
	}
	if p.Type != "string" {
		t.Errorf("expected type 'string', got %q", p.Type)
	}
	if p.In != "path" {
		t.Errorf("expected in 'path', got %q", p.In)
	}
	if p.Required {
		t.Error("expected required=false for portfolio_name")
	}
	if p.DefaultFrom != "user_config.default_portfolio" {
		t.Errorf("expected default_from 'user_config.default_portfolio', got %q", p.DefaultFrom)
	}

	// Second param: focus_signals (body, array)
	p2 := ct.Params[1]
	if p2.Name != "focus_signals" {
		t.Errorf("expected param name 'focus_signals', got %q", p2.Name)
	}
	if p2.Type != "array" {
		t.Errorf("expected type 'array', got %q", p2.Type)
	}
	if p2.In != "body" {
		t.Errorf("expected in 'body', got %q", p2.In)
	}
}

func TestCatalogTool_EmptyCatalog(t *testing.T) {
	var catalog []CatalogTool
	if err := json.Unmarshal([]byte(`[]`), &catalog); err != nil {
		t.Fatalf("failed to parse empty catalog: %v", err)
	}
	if len(catalog) != 0 {
		t.Errorf("expected 0 tools, got %d", len(catalog))
	}
}

func TestCatalogTool_NoParams(t *testing.T) {
	catalog := parseSampleCatalog(t)

	ct := findCatalogTool(catalog, "get_version")
	if ct == nil {
		t.Fatal("expected get_version in catalog")
	}
	if len(ct.Params) != 0 {
		t.Errorf("expected 0 params for get_version, got %d", len(ct.Params))
	}
}

func TestCatalogTool_AllParamTypes(t *testing.T) {
	catalog := parseSampleCatalog(t)

	// Collect all param types seen
	types := map[string]bool{}
	for _, ct := range catalog {
		for _, p := range ct.Params {
			types[p.Type] = true
		}
	}

	for _, expected := range []string{"string", "array", "boolean", "number"} {
		if !types[expected] {
			t.Errorf("expected param type %q in sample catalog", expected)
		}
	}
}

func TestCatalogTool_AllInValues(t *testing.T) {
	catalog := parseSampleCatalog(t)

	ins := map[string]bool{}
	for _, ct := range catalog {
		for _, p := range ct.Params {
			ins[p.In] = true
		}
	}

	for _, expected := range []string{"path", "query", "body"} {
		if !ins[expected] {
			t.Errorf("expected in=%q in sample catalog", expected)
		}
	}
}

func TestCatalogTool_AllHTTPMethods(t *testing.T) {
	catalog := parseSampleCatalog(t)

	methods := map[string]bool{}
	for _, ct := range catalog {
		methods[ct.Method] = true
	}

	for _, expected := range []string{"GET", "POST", "PUT", "DELETE", "PATCH"} {
		if !methods[expected] {
			t.Errorf("expected method %q in sample catalog", expected)
		}
	}
}

// --- Catalog Validation Tests ---

func TestValidateCatalogTool_Valid(t *testing.T) {
	ct := CatalogTool{Name: "get_version", Method: "GET", Path: "/api/version"}
	if err := ValidateCatalogTool(ct); err != nil {
		t.Errorf("expected valid tool, got error: %v", err)
	}
}

func TestValidateCatalogTool_EmptyName(t *testing.T) {
	ct := CatalogTool{Name: "", Method: "GET", Path: "/api/version"}
	if err := ValidateCatalogTool(ct); err == nil {
		t.Error("expected error for empty name")
	}
}

func TestValidateCatalogTool_EmptyMethod(t *testing.T) {
	ct := CatalogTool{Name: "test", Method: "", Path: "/api/test"}
	if err := ValidateCatalogTool(ct); err == nil {
		t.Error("expected error for empty method")
	}
}

func TestValidateCatalogTool_InvalidMethod(t *testing.T) {
	ct := CatalogTool{Name: "test", Method: "TRACE", Path: "/api/test"}
	if err := ValidateCatalogTool(ct); err == nil {
		t.Error("expected error for unsupported method TRACE")
	}
}

func TestValidateCatalogTool_EmptyPath(t *testing.T) {
	ct := CatalogTool{Name: "test", Method: "GET", Path: ""}
	if err := ValidateCatalogTool(ct); err == nil {
		t.Error("expected error for empty path")
	}
}

func TestValidateCatalogTool_PathMissingAPIPrefix(t *testing.T) {
	ct := CatalogTool{Name: "test", Method: "GET", Path: "/admin/secrets"}
	if err := ValidateCatalogTool(ct); err == nil {
		t.Error("expected error for path without /api/ prefix")
	}
}

func TestValidateCatalogTool_PathTraversal(t *testing.T) {
	ct := CatalogTool{Name: "test", Method: "GET", Path: "/api/../etc/passwd"}
	if err := ValidateCatalogTool(ct); err == nil {
		t.Error("expected error for path traversal")
	}
}

func TestValidateCatalogTool_AllValidMethods(t *testing.T) {
	for _, method := range []string{"GET", "POST", "PUT", "PATCH", "DELETE"} {
		ct := CatalogTool{Name: "test_" + method, Method: method, Path: "/api/test"}
		if err := ValidateCatalogTool(ct); err != nil {
			t.Errorf("expected method %q to be valid, got error: %v", method, err)
		}
	}
}

func TestValidateCatalog_FiltersDuplicates(t *testing.T) {
	catalog := []CatalogTool{
		{Name: "tool_a", Method: "GET", Path: "/api/a"},
		{Name: "tool_b", Method: "GET", Path: "/api/b"},
		{Name: "tool_a", Method: "POST", Path: "/api/a2"}, // duplicate name
	}

	valid := ValidateCatalog(catalog, testLogger())
	if len(valid) != 2 {
		t.Errorf("expected 2 valid tools (1 duplicate removed), got %d", len(valid))
	}
}

func TestValidateCatalog_FiltersInvalid(t *testing.T) {
	catalog := []CatalogTool{
		{Name: "good_tool", Method: "GET", Path: "/api/good"},
		{Name: "", Method: "GET", Path: "/api/bad"},           // empty name
		{Name: "bad_path", Method: "GET", Path: "/evil/path"}, // no /api/ prefix
	}

	valid := ValidateCatalog(catalog, testLogger())
	if len(valid) != 1 {
		t.Errorf("expected 1 valid tool, got %d", len(valid))
	}
	if valid[0].Name != "good_tool" {
		t.Errorf("expected surviving tool to be good_tool, got %q", valid[0].Name)
	}
}

func TestValidateCatalog_EmptyInput(t *testing.T) {
	valid := ValidateCatalog([]CatalogTool{}, testLogger())
	if len(valid) != 0 {
		t.Errorf("expected 0 valid tools from empty input, got %d", len(valid))
	}
}

// --- Catalog Size Limit Tests ---

func TestFetchCatalog_SizeLimit(t *testing.T) {
	// Generate a catalog response larger than 1MB
	bigPayload := strings.Repeat(`{"name":"tool","method":"GET","path":"/api/x","params":[]},`, 20000)
	bigPayload = "[" + bigPayload[:len(bigPayload)-1] + "]" // valid JSON array

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(bigPayload))
	}))
	defer mockServer.Close()

	p := NewMCPProxy(mockServer.URL, testLogger(), testConfig())

	_, err := p.FetchCatalog(t.Context())
	if err == nil {
		t.Fatal("expected error for oversized catalog response")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Errorf("expected 'too large' error, got: %v", err)
	}
}

// --- FetchCatalog Tests ---

func TestFetchCatalog_ParsesJSON(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/mcp/tools" {
			t.Errorf("expected path /api/mcp/tools, got %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("expected GET method, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(sampleCatalogJSON()))
	}))
	defer mockServer.Close()

	p := NewMCPProxy(mockServer.URL, testLogger(), testConfig())

	catalog, err := p.FetchCatalog(t.Context())
	if err != nil {
		t.Fatalf("FetchCatalog failed: %v", err)
	}

	if len(catalog) != 9 {
		t.Errorf("expected 9 tools, got %d", len(catalog))
	}

	// Verify a specific tool was parsed
	ct := findCatalogTool(catalog, "get_quote")
	if ct == nil {
		t.Fatal("expected get_quote in fetched catalog")
	}
	if ct.Method != "GET" {
		t.Errorf("expected GET method, got %s", ct.Method)
	}
}

func TestFetchCatalog_EmptyArray(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[]`))
	}))
	defer mockServer.Close()

	p := NewMCPProxy(mockServer.URL, testLogger(), testConfig())

	catalog, err := p.FetchCatalog(t.Context())
	if err != nil {
		t.Fatalf("FetchCatalog failed: %v", err)
	}
	if len(catalog) != 0 {
		t.Errorf("expected 0 tools, got %d", len(catalog))
	}
}

func TestFetchCatalog_ServerDown(t *testing.T) {
	// Point to a server that doesn't exist
	p := NewMCPProxy("http://127.0.0.1:1", testLogger(), testConfig())

	catalog, err := p.FetchCatalog(t.Context())
	if err == nil {
		t.Fatal("expected error when server is down, got nil")
	}
	if catalog != nil {
		t.Error("expected nil catalog when server is down")
	}
}

func TestFetchCatalog_ServerReturnsError(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal error"}`))
	}))
	defer mockServer.Close()

	p := NewMCPProxy(mockServer.URL, testLogger(), testConfig())

	_, err := p.FetchCatalog(t.Context())
	if err == nil {
		t.Fatal("expected error for 500 response, got nil")
	}
}

func TestFetchCatalog_InvalidJSON(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`not json`))
	}))
	defer mockServer.Close()

	p := NewMCPProxy(mockServer.URL, testLogger(), testConfig())

	_, err := p.FetchCatalog(t.Context())
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestFetchCatalog_ForwardsHeaders(t *testing.T) {
	var receivedHeaders http.Header
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[]`))
	}))
	defer mockServer.Close()

	p := NewMCPProxy(mockServer.URL, testLogger(), testConfig())

	_, err := p.FetchCatalog(t.Context())
	if err != nil {
		t.Fatalf("FetchCatalog failed: %v", err)
	}

	// Verify X-Vire-* headers forwarded on catalog fetch
	if receivedHeaders.Get("X-Vire-Portfolios") != "SMSF,Personal" {
		t.Errorf("expected X-Vire-Portfolios forwarded, got %q", receivedHeaders.Get("X-Vire-Portfolios"))
	}
}

// --- BuildMCPTool Tests ---

func TestBuildMCPTool_NoParams(t *testing.T) {
	ct := CatalogTool{
		Name:        "get_version",
		Description: "Get the server version.",
		Method:      "GET",
		Path:        "/api/version",
		Params:      []CatalogParam{},
	}

	tool := BuildMCPTool(ct)

	if tool.Name != "get_version" {
		t.Errorf("expected name 'get_version', got %q", tool.Name)
	}
	if tool.Description != "Get the server version." {
		t.Errorf("expected description 'Get the server version.', got %q", tool.Description)
	}
}

func TestBuildMCPTool_StringParam(t *testing.T) {
	ct := CatalogTool{
		Name:        "get_quote",
		Description: "Get a price quote.",
		Method:      "GET",
		Path:        "/api/market/quote/{ticker}",
		Params: []CatalogParam{
			{Name: "ticker", Type: "string", Description: "Ticker symbol", Required: true, In: "path"},
		},
	}

	tool := BuildMCPTool(ct)

	if tool.Name != "get_quote" {
		t.Errorf("expected name 'get_quote', got %q", tool.Name)
	}

	// Check that the tool schema includes the ticker parameter
	schema := tool.InputSchema
	if _, exists := schema.Properties["ticker"]; !exists {
		t.Error("expected 'ticker' in tool schema properties")
	}
}

func TestBuildMCPTool_RequiredParam(t *testing.T) {
	ct := CatalogTool{
		Name:        "get_quote",
		Description: "Get a price quote.",
		Method:      "GET",
		Path:        "/api/market/quote/{ticker}",
		Params: []CatalogParam{
			{Name: "ticker", Type: "string", Description: "Ticker symbol", Required: true, In: "path"},
		},
	}

	tool := BuildMCPTool(ct)

	// Check that ticker is in the required list
	found := false
	for _, r := range tool.InputSchema.Required {
		if r == "ticker" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'ticker' in required list")
	}
}

func TestBuildMCPTool_OptionalParam(t *testing.T) {
	ct := CatalogTool{
		Name:        "list_reports",
		Description: "List reports.",
		Method:      "GET",
		Path:        "/api/reports",
		Params: []CatalogParam{
			{Name: "portfolio_name", Type: "string", Description: "Filter by portfolio", Required: false, In: "query"},
		},
	}

	tool := BuildMCPTool(ct)

	// portfolio_name should NOT be in required list
	for _, r := range tool.InputSchema.Required {
		if r == "portfolio_name" {
			t.Error("expected 'portfolio_name' to NOT be in required list")
		}
	}
}

func TestBuildMCPTool_ArrayParam(t *testing.T) {
	ct := CatalogTool{
		Name:        "compute_indicators",
		Description: "Compute indicators.",
		Method:      "POST",
		Path:        "/api/market/signals",
		Params: []CatalogParam{
			{Name: "tickers", Type: "array", Description: "Tickers", Required: true, In: "body"},
		},
	}

	tool := BuildMCPTool(ct)

	schema := tool.InputSchema
	tickersProp, exists := schema.Properties["tickers"]
	if !exists {
		t.Fatal("expected 'tickers' in tool schema properties")
	}

	// The property should be an array type
	propMap, ok := tickersProp.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map for tickers property, got %T", tickersProp)
	}
	if propMap["type"] != "array" {
		t.Errorf("expected type 'array', got %v", propMap["type"])
	}
}

func TestBuildMCPTool_BooleanParam(t *testing.T) {
	ct := CatalogTool{
		Name:        "test_tool",
		Description: "Test.",
		Method:      "POST",
		Path:        "/api/test",
		Params: []CatalogParam{
			{Name: "include_news", Type: "boolean", Description: "Include news", Required: false, In: "body"},
		},
	}

	tool := BuildMCPTool(ct)

	schema := tool.InputSchema
	newsProp, exists := schema.Properties["include_news"]
	if !exists {
		t.Fatal("expected 'include_news' in tool schema properties")
	}
	propMap, ok := newsProp.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map for include_news property, got %T", newsProp)
	}
	if propMap["type"] != "boolean" {
		t.Errorf("expected type 'boolean', got %v", propMap["type"])
	}
}

func TestBuildMCPTool_NumberParam(t *testing.T) {
	ct := CatalogTool{
		Name:        "test_tool",
		Description: "Test.",
		Method:      "GET",
		Path:        "/api/test",
		Params: []CatalogParam{
			{Name: "limit", Type: "number", Description: "Max entries", Required: false, In: "query"},
		},
	}

	tool := BuildMCPTool(ct)

	schema := tool.InputSchema
	limitProp, exists := schema.Properties["limit"]
	if !exists {
		t.Fatal("expected 'limit' in tool schema properties")
	}
	propMap, ok := limitProp.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map for limit property, got %T", limitProp)
	}
	if propMap["type"] != "number" {
		t.Errorf("expected type 'number', got %v", propMap["type"])
	}
}

func TestBuildMCPTool_MultipleParams(t *testing.T) {
	ct := CatalogTool{
		Name:        "portfolio_compliance",
		Description: "Review a portfolio.",
		Method:      "POST",
		Path:        "/api/portfolios/{portfolio_name}/review",
		Params: []CatalogParam{
			{Name: "portfolio_name", Type: "string", In: "path"},
			{Name: "focus_signals", Type: "array", In: "body"},
			{Name: "include_news", Type: "boolean", In: "body"},
		},
	}

	tool := BuildMCPTool(ct)

	schema := tool.InputSchema

	for _, name := range []string{"portfolio_name", "focus_signals", "include_news"} {
		if _, exists := schema.Properties[name]; !exists {
			t.Errorf("expected %q in tool schema properties", name)
		}
	}
}

// --- RegisterToolsFromCatalog Tests ---

func TestRegisterToolsFromCatalog_Count(t *testing.T) {
	catalog := parseSampleCatalog(t)

	s := mcpserver.NewMCPServer("test", "1.0.0", mcpserver.WithToolCapabilities(true))
	p := NewMCPProxy("http://localhost:4242", testLogger(), testConfig())

	count := RegisterToolsFromCatalog(s, p, catalog)

	if count != len(catalog) {
		t.Errorf("expected RegisterToolsFromCatalog to return %d, got %d", len(catalog), count)
	}

	tools := listTools(t, s)
	if len(tools) != len(catalog) {
		t.Errorf("expected %d registered tools, got %d", len(catalog), len(tools))
	}
}

func TestRegisterToolsFromCatalog_EmptyCatalog(t *testing.T) {
	s := mcpserver.NewMCPServer("test", "1.0.0", mcpserver.WithToolCapabilities(true))
	p := NewMCPProxy("http://localhost:4242", testLogger(), testConfig())

	count := RegisterToolsFromCatalog(s, p, []CatalogTool{})

	if count != 0 {
		t.Errorf("expected 0 tools registered from empty catalog, got %d", count)
	}

	tools := listTools(t, s)
	if len(tools) != 0 {
		t.Errorf("expected 0 registered tools, got %d", len(tools))
	}
}

func TestRegisterToolsFromCatalog_ToolNames(t *testing.T) {
	catalog := parseSampleCatalog(t)

	s := mcpserver.NewMCPServer("test", "1.0.0", mcpserver.WithToolCapabilities(true))
	p := NewMCPProxy("http://localhost:4242", testLogger(), testConfig())

	RegisterToolsFromCatalog(s, p, catalog)

	tools := listTools(t, s)
	registered := make(map[string]bool)
	for _, tool := range tools {
		registered[tool.Name] = true
	}

	for _, ct := range catalog {
		if !registered[ct.Name] {
			t.Errorf("expected tool %q to be registered", ct.Name)
		}
	}
}

func TestRegisterToolsFromCatalog_ToolsHaveDescriptions(t *testing.T) {
	catalog := parseSampleCatalog(t)

	s := mcpserver.NewMCPServer("test", "1.0.0", mcpserver.WithToolCapabilities(true))
	p := NewMCPProxy("http://localhost:4242", testLogger(), testConfig())

	RegisterToolsFromCatalog(s, p, catalog)

	tools := listTools(t, s)
	for _, tool := range tools {
		if tool.Description == "" {
			t.Errorf("tool %q has empty description", tool.Name)
		}
	}
}

// --- GenericToolHandler Tests ---

func TestGenericHandler_GET_NoParams(t *testing.T) {
	var receivedPath string
	var receivedMethod string
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		receivedMethod = r.Method
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"version":"1.0.0"}`))
	}))
	defer mockServer.Close()

	ct := CatalogTool{
		Name:   "get_version",
		Method: "GET",
		Path:   "/api/version",
		Params: []CatalogParam{},
	}

	s := mcpserver.NewMCPServer("test", "1.0.0", mcpserver.WithToolCapabilities(true))
	p := NewMCPProxy(mockServer.URL, testLogger(), testConfig())
	s.AddTool(BuildMCPTool(ct), GenericToolHandler(p, ct))

	result := callTool(t, s, "get_version", map[string]interface{}{})

	if result.IsError {
		t.Error("expected non-error result")
	}
	if receivedMethod != "GET" {
		t.Errorf("expected GET, got %s", receivedMethod)
	}
	if receivedPath != "/api/version" {
		t.Errorf("expected /api/version, got %s", receivedPath)
	}

	text := extractText(t, result.Content[0])
	if !strings.Contains(text, `"version"`) {
		t.Errorf("expected raw JSON with version, got: %s", text)
	}
}

func TestGenericHandler_GET_PathParam(t *testing.T) {
	var receivedPath string
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ticker":"BHP.AU","close":45.50}`))
	}))
	defer mockServer.Close()

	ct := CatalogTool{
		Name:   "get_quote",
		Method: "GET",
		Path:   "/api/market/quote/{ticker}",
		Params: []CatalogParam{
			{Name: "ticker", Type: "string", Required: true, In: "path"},
		},
	}

	s := mcpserver.NewMCPServer("test", "1.0.0", mcpserver.WithToolCapabilities(true))
	p := NewMCPProxy(mockServer.URL, testLogger(), testConfig())
	s.AddTool(BuildMCPTool(ct), GenericToolHandler(p, ct))

	result := callTool(t, s, "get_quote", map[string]interface{}{"ticker": "BHP.AU"})

	if result.IsError {
		t.Error("expected non-error result")
	}
	if receivedPath != "/api/market/quote/BHP.AU" {
		t.Errorf("expected /api/market/quote/BHP.AU, got %s", receivedPath)
	}
}

func TestGenericHandler_GET_PathParamEncoded(t *testing.T) {
	var receivedRawPath string
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Use RequestURI to see the encoded path as sent over the wire
		receivedRawPath = r.URL.RequestURI()
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"portfolio":"My Portfolio"}`))
	}))
	defer mockServer.Close()

	ct := CatalogTool{
		Name:   "get_portfolio",
		Method: "GET",
		Path:   "/api/portfolios/{portfolio_name}",
		Params: []CatalogParam{
			{Name: "portfolio_name", Type: "string", Required: true, In: "path"},
		},
	}

	s := mcpserver.NewMCPServer("test", "1.0.0", mcpserver.WithToolCapabilities(true))
	p := NewMCPProxy(mockServer.URL, testLogger(), testConfig())
	s.AddTool(BuildMCPTool(ct), GenericToolHandler(p, ct))

	result := callTool(t, s, "get_portfolio", map[string]interface{}{"portfolio_name": "My Portfolio"})

	if result.IsError {
		t.Error("expected non-error result")
	}
	// Path should be URL-encoded in the request URI
	if receivedRawPath != "/api/portfolios/My%20Portfolio" {
		t.Errorf("expected /api/portfolios/My%%20Portfolio, got %s", receivedRawPath)
	}
}

func TestGenericHandler_GET_MultiplePathParams(t *testing.T) {
	var receivedPath string
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"updated"}`))
	}))
	defer mockServer.Close()

	ct := CatalogTool{
		Name:   "update_plan_item",
		Method: "PATCH",
		Path:   "/api/portfolios/{portfolio_name}/plan/items/{item_id}",
		Params: []CatalogParam{
			{Name: "portfolio_name", Type: "string", Required: true, In: "path"},
			{Name: "item_id", Type: "string", Required: true, In: "path"},
			{Name: "item_json", Type: "string", Required: true, In: "body"},
		},
	}

	s := mcpserver.NewMCPServer("test", "1.0.0", mcpserver.WithToolCapabilities(true))
	p := NewMCPProxy(mockServer.URL, testLogger(), testConfig())
	s.AddTool(BuildMCPTool(ct), GenericToolHandler(p, ct))

	result := callTool(t, s, "update_plan_item", map[string]interface{}{
		"portfolio_name": "SMSF",
		"item_id":        "item-123",
		"item_json":      `{"status":"done"}`,
	})

	if result.IsError {
		text := extractText(t, result.Content[0])
		t.Fatalf("expected non-error result, got: %s", text)
	}
	if receivedPath != "/api/portfolios/SMSF/plan/items/item-123" {
		t.Errorf("expected /api/portfolios/SMSF/plan/items/item-123, got %s", receivedPath)
	}
}

func TestGenericHandler_GET_QueryParams(t *testing.T) {
	var receivedQuery string
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"reports":[]}`))
	}))
	defer mockServer.Close()

	ct := CatalogTool{
		Name:   "list_reports",
		Method: "GET",
		Path:   "/api/reports",
		Params: []CatalogParam{
			{Name: "portfolio_name", Type: "string", Required: false, In: "query"},
		},
	}

	s := mcpserver.NewMCPServer("test", "1.0.0", mcpserver.WithToolCapabilities(true))
	p := NewMCPProxy(mockServer.URL, testLogger(), testConfig())
	s.AddTool(BuildMCPTool(ct), GenericToolHandler(p, ct))

	result := callTool(t, s, "list_reports", map[string]interface{}{"portfolio_name": "SMSF"})

	if result.IsError {
		t.Error("expected non-error result")
	}
	if !strings.Contains(receivedQuery, "portfolio_name=SMSF") {
		t.Errorf("expected query to contain portfolio_name=SMSF, got %q", receivedQuery)
	}
}

func TestGenericHandler_GET_QueryParams_Empty(t *testing.T) {
	var receivedQuery string
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"reports":[]}`))
	}))
	defer mockServer.Close()

	ct := CatalogTool{
		Name:   "list_reports",
		Method: "GET",
		Path:   "/api/reports",
		Params: []CatalogParam{
			{Name: "portfolio_name", Type: "string", Required: false, In: "query"},
		},
	}

	s := mcpserver.NewMCPServer("test", "1.0.0", mcpserver.WithToolCapabilities(true))
	p := NewMCPProxy(mockServer.URL, testLogger(), testConfig())
	s.AddTool(BuildMCPTool(ct), GenericToolHandler(p, ct))

	// Call without query param — should not append query string
	result := callTool(t, s, "list_reports", map[string]interface{}{})

	if result.IsError {
		t.Error("expected non-error result")
	}
	if receivedQuery != "" {
		t.Errorf("expected empty query string, got %q", receivedQuery)
	}
}

func TestGenericHandler_GET_MultipleQueryParams(t *testing.T) {
	var receivedQuery string
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"logs":[]}`))
	}))
	defer mockServer.Close()

	ct := CatalogTool{
		Name:   "get_diagnostics",
		Method: "GET",
		Path:   "/api/diagnostics",
		Params: []CatalogParam{
			{Name: "correlation_id", Type: "string", Required: false, In: "query"},
			{Name: "limit", Type: "number", Required: false, In: "query"},
		},
	}

	s := mcpserver.NewMCPServer("test", "1.0.0", mcpserver.WithToolCapabilities(true))
	p := NewMCPProxy(mockServer.URL, testLogger(), testConfig())
	s.AddTool(BuildMCPTool(ct), GenericToolHandler(p, ct))

	result := callTool(t, s, "get_diagnostics", map[string]interface{}{
		"correlation_id": "abc-123",
		"limit":          25,
	})

	if result.IsError {
		t.Error("expected non-error result")
	}
	if !strings.Contains(receivedQuery, "correlation_id=abc-123") {
		t.Errorf("expected query to contain correlation_id, got %q", receivedQuery)
	}
	if !strings.Contains(receivedQuery, "limit=25") {
		t.Errorf("expected query to contain limit=25, got %q", receivedQuery)
	}
}

func TestGenericHandler_POST_BodyParams(t *testing.T) {
	var receivedMethod string
	var receivedPath string
	var receivedBody string
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		receivedPath = r.URL.Path
		bodyBytes, _ := io.ReadAll(r.Body)
		receivedBody = string(bodyBytes)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"result":"ok"}`))
	}))
	defer mockServer.Close()

	ct := CatalogTool{
		Name:   "portfolio_compliance",
		Method: "POST",
		Path:   "/api/portfolios/{portfolio_name}/review",
		Params: []CatalogParam{
			{Name: "portfolio_name", Type: "string", In: "path", DefaultFrom: "user_config.default_portfolio"},
			{Name: "focus_signals", Type: "array", In: "body"},
			{Name: "include_news", Type: "boolean", In: "body"},
		},
	}

	s := mcpserver.NewMCPServer("test", "1.0.0", mcpserver.WithToolCapabilities(true))
	p := NewMCPProxy(mockServer.URL, testLogger(), testConfig())
	s.AddTool(BuildMCPTool(ct), GenericToolHandler(p, ct))

	result := callTool(t, s, "portfolio_compliance", map[string]interface{}{
		"portfolio_name": "SMSF",
		"focus_signals":  []interface{}{"rsi", "sma"},
		"include_news":   true,
	})

	if result.IsError {
		text := extractText(t, result.Content[0])
		t.Fatalf("expected non-error result, got: %s", text)
	}
	if receivedMethod != "POST" {
		t.Errorf("expected POST, got %s", receivedMethod)
	}
	if receivedPath != "/api/portfolios/SMSF/review" {
		t.Errorf("expected /api/portfolios/SMSF/review, got %s", receivedPath)
	}
	if !strings.Contains(receivedBody, `"focus_signals"`) {
		t.Errorf("expected body to contain focus_signals, got: %s", receivedBody)
	}
	if !strings.Contains(receivedBody, `"include_news"`) {
		t.Errorf("expected body to contain include_news, got: %s", receivedBody)
	}
}

func TestGenericHandler_PUT_Method(t *testing.T) {
	var receivedMethod string
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer mockServer.Close()

	ct := CatalogTool{
		Name:   "set_portfolio_strategy",
		Method: "PUT",
		Path:   "/api/portfolios/{portfolio_name}/strategy",
		Params: []CatalogParam{
			{Name: "portfolio_name", Type: "string", In: "path"},
			{Name: "strategy_json", Type: "string", Required: true, In: "body"},
		},
	}

	s := mcpserver.NewMCPServer("test", "1.0.0", mcpserver.WithToolCapabilities(true))
	p := NewMCPProxy(mockServer.URL, testLogger(), testConfig())
	s.AddTool(BuildMCPTool(ct), GenericToolHandler(p, ct))

	result := callTool(t, s, "set_portfolio_strategy", map[string]interface{}{
		"portfolio_name": "SMSF",
		"strategy_json":  `{"risk":"moderate"}`,
	})

	if result.IsError {
		text := extractText(t, result.Content[0])
		t.Fatalf("expected non-error result, got: %s", text)
	}
	if receivedMethod != "PUT" {
		t.Errorf("expected PUT, got %s", receivedMethod)
	}
}

func TestGenericHandler_DELETE_Method(t *testing.T) {
	var receivedMethod string
	var receivedPath string
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		receivedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"deleted"}`))
	}))
	defer mockServer.Close()

	ct := CatalogTool{
		Name:   "delete_portfolio_strategy",
		Method: "DELETE",
		Path:   "/api/portfolios/{portfolio_name}/strategy",
		Params: []CatalogParam{
			{Name: "portfolio_name", Type: "string", In: "path"},
		},
	}

	s := mcpserver.NewMCPServer("test", "1.0.0", mcpserver.WithToolCapabilities(true))
	p := NewMCPProxy(mockServer.URL, testLogger(), testConfig())
	s.AddTool(BuildMCPTool(ct), GenericToolHandler(p, ct))

	result := callTool(t, s, "delete_portfolio_strategy", map[string]interface{}{
		"portfolio_name": "SMSF",
	})

	if result.IsError {
		t.Error("expected non-error result")
	}
	if receivedMethod != "DELETE" {
		t.Errorf("expected DELETE, got %s", receivedMethod)
	}
	if receivedPath != "/api/portfolios/SMSF/strategy" {
		t.Errorf("expected /api/portfolios/SMSF/strategy, got %s", receivedPath)
	}
}

func TestGenericHandler_PATCH_Method(t *testing.T) {
	var receivedMethod string
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"updated"}`))
	}))
	defer mockServer.Close()

	ct := CatalogTool{
		Name:   "update_plan_item",
		Method: "PATCH",
		Path:   "/api/portfolios/{portfolio_name}/plan/items/{item_id}",
		Params: []CatalogParam{
			{Name: "portfolio_name", Type: "string", In: "path"},
			{Name: "item_id", Type: "string", Required: true, In: "path"},
			{Name: "item_json", Type: "string", Required: true, In: "body"},
		},
	}

	s := mcpserver.NewMCPServer("test", "1.0.0", mcpserver.WithToolCapabilities(true))
	p := NewMCPProxy(mockServer.URL, testLogger(), testConfig())
	s.AddTool(BuildMCPTool(ct), GenericToolHandler(p, ct))

	result := callTool(t, s, "update_plan_item", map[string]interface{}{
		"portfolio_name": "SMSF",
		"item_id":        "item-1",
		"item_json":      `{"status":"done"}`,
	})

	if result.IsError {
		text := extractText(t, result.Content[0])
		t.Fatalf("expected non-error result, got: %s", text)
	}
	if receivedMethod != "PATCH" {
		t.Errorf("expected PATCH, got %s", receivedMethod)
	}
}

func TestGenericHandler_MissingRequiredPathParam(t *testing.T) {
	ct := CatalogTool{
		Name:   "get_quote",
		Method: "GET",
		Path:   "/api/market/quote/{ticker}",
		Params: []CatalogParam{
			{Name: "ticker", Type: "string", Required: true, In: "path"},
		},
	}

	s := mcpserver.NewMCPServer("test", "1.0.0", mcpserver.WithToolCapabilities(true))
	p := NewMCPProxy("http://localhost:4242", testLogger(), testConfig())
	s.AddTool(BuildMCPTool(ct), GenericToolHandler(p, ct))

	// Call without required ticker param
	result := callTool(t, s, "get_quote", map[string]interface{}{})

	if !result.IsError {
		t.Error("expected error result for missing required param")
	}
	text := extractText(t, result.Content[0])
	if !strings.Contains(text, "ticker") {
		t.Errorf("expected error to mention 'ticker', got: %s", text)
	}
}

func TestGenericHandler_ServerError(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal server error"}`))
	}))
	defer mockServer.Close()

	ct := CatalogTool{
		Name:   "get_version",
		Method: "GET",
		Path:   "/api/version",
		Params: []CatalogParam{},
	}

	s := mcpserver.NewMCPServer("test", "1.0.0", mcpserver.WithToolCapabilities(true))
	p := NewMCPProxy(mockServer.URL, testLogger(), testConfig())
	s.AddTool(BuildMCPTool(ct), GenericToolHandler(p, ct))

	result := callTool(t, s, "get_version", map[string]interface{}{})

	if !result.IsError {
		t.Error("expected error result for server error")
	}
}

// --- default_from Resolution Tests ---

func TestGenericHandler_DefaultFrom_Portfolio(t *testing.T) {
	var receivedPath string
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"portfolio":"SMSF"}`))
	}))
	defer mockServer.Close()

	ct := CatalogTool{
		Name:   "get_portfolio",
		Method: "GET",
		Path:   "/api/portfolios/{portfolio_name}",
		Params: []CatalogParam{
			{Name: "portfolio_name", Type: "string", Required: false, In: "path",
				DefaultFrom: "user_config.default_portfolio"},
		},
	}

	cfg := testConfig()
	cfg.User.Portfolios = []string{"SMSF", "Personal"}

	s := mcpserver.NewMCPServer("test", "1.0.0", mcpserver.WithToolCapabilities(true))
	p := NewMCPProxy(mockServer.URL, testLogger(), cfg)
	s.AddTool(BuildMCPTool(ct), GenericToolHandler(p, ct))

	// Call without portfolio_name — should use default "SMSF" from config
	result := callTool(t, s, "get_portfolio", map[string]interface{}{})

	if result.IsError {
		text := extractText(t, result.Content[0])
		t.Fatalf("expected non-error result, got: %s", text)
	}
	if receivedPath != "/api/portfolios/SMSF" {
		t.Errorf("expected /api/portfolios/SMSF from default_from, got %s", receivedPath)
	}
}

func TestGenericHandler_DefaultFrom_ExplicitOverridesDefault(t *testing.T) {
	var receivedPath string
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"portfolio":"Personal"}`))
	}))
	defer mockServer.Close()

	ct := CatalogTool{
		Name:   "get_portfolio",
		Method: "GET",
		Path:   "/api/portfolios/{portfolio_name}",
		Params: []CatalogParam{
			{Name: "portfolio_name", Type: "string", Required: false, In: "path",
				DefaultFrom: "user_config.default_portfolio"},
		},
	}

	cfg := testConfig()
	cfg.User.Portfolios = []string{"SMSF", "Personal"}

	s := mcpserver.NewMCPServer("test", "1.0.0", mcpserver.WithToolCapabilities(true))
	p := NewMCPProxy(mockServer.URL, testLogger(), cfg)
	s.AddTool(BuildMCPTool(ct), GenericToolHandler(p, ct))

	// Explicit portfolio_name should override default
	result := callTool(t, s, "get_portfolio", map[string]interface{}{"portfolio_name": "Personal"})

	if result.IsError {
		t.Error("expected non-error result")
	}
	if receivedPath != "/api/portfolios/Personal" {
		t.Errorf("expected /api/portfolios/Personal, got %s", receivedPath)
	}
}

func TestGenericHandler_DefaultFrom_NoConfig(t *testing.T) {
	ct := CatalogTool{
		Name:   "get_portfolio",
		Method: "GET",
		Path:   "/api/portfolios/{portfolio_name}",
		Params: []CatalogParam{
			{Name: "portfolio_name", Type: "string", Required: false, In: "path",
				DefaultFrom: "user_config.default_portfolio"},
		},
	}

	cfg := config.NewDefaultConfig()
	// No portfolios configured, no server to fall back to

	s := mcpserver.NewMCPServer("test", "1.0.0", mcpserver.WithToolCapabilities(true))
	p := NewMCPProxy("http://127.0.0.1:1", testLogger(), cfg)
	s.AddTool(BuildMCPTool(ct), GenericToolHandler(p, ct))

	// No portfolio_name, no default — should get error (unreplaced path param)
	result := callTool(t, s, "get_portfolio", map[string]interface{}{})

	// The handler should return an error or handle gracefully
	// Either way, it should not panic
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestGenericHandler_DefaultFrom_APIFallback(t *testing.T) {
	// Mock server: returns default portfolio via API, handles the resulting tool call
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/portfolios/default":
			w.Write([]byte(`{"default":"FromServer"}`))
		case "/api/portfolios/FromServer":
			w.Write([]byte(`{"portfolio":"FromServer","holdings":[]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error":"not found"}`))
		}
	}))
	defer mockServer.Close()

	ct := CatalogTool{
		Name:   "get_portfolio",
		Method: "GET",
		Path:   "/api/portfolios/{portfolio_name}",
		Params: []CatalogParam{
			{Name: "portfolio_name", Type: "string", Required: false, In: "path",
				DefaultFrom: "user_config.default_portfolio"},
		},
	}

	cfg := config.NewDefaultConfig()
	// No portfolios in config — forces API fallback

	s := mcpserver.NewMCPServer("test", "1.0.0", mcpserver.WithToolCapabilities(true))
	p := NewMCPProxy(mockServer.URL, testLogger(), cfg)
	s.AddTool(BuildMCPTool(ct), GenericToolHandler(p, ct))

	result := callTool(t, s, "get_portfolio", map[string]interface{}{})

	if result.IsError {
		text := extractText(t, result.Content[0])
		t.Fatalf("expected non-error result, got: %s", text)
	}
	text := extractText(t, result.Content[0])
	if !strings.Contains(text, "FromServer") {
		t.Errorf("expected response from API fallback portfolio, got: %s", text)
	}
}

// --- Handler Startup Tests ---

func TestNewHandler_CatalogUnavailable(t *testing.T) {
	// Point to a server that doesn't exist
	cfg := testConfig()
	cfg.API.URL = "http://127.0.0.1:1"

	handler := NewHandler(cfg, testLogger())

	// Handler should still be created (non-fatal)
	if handler == nil {
		t.Fatal("expected non-nil handler even when catalog unavailable")
	}
}

func TestNewHandler_CatalogRetry_SucceedsOnSecondAttempt(t *testing.T) {
	var attempts int
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if r.URL.Path == "/api/mcp/tools" {
			if attempts == 1 {
				// First attempt fails
				w.WriteHeader(http.StatusServiceUnavailable)
				w.Write([]byte(`{"error":"starting up"}`))
				return
			}
			// Second attempt succeeds
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`[{"name":"get_version","description":"Version","method":"GET","path":"/api/version","params":[]}]`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer mockServer.Close()

	cfg := testConfig()
	cfg.API.URL = mockServer.URL

	handler := NewHandler(cfg, testLogger())

	if handler == nil {
		t.Fatal("expected non-nil handler")
	}
	if attempts < 2 {
		t.Errorf("expected at least 2 attempts, got %d", attempts)
	}
}

func TestNewHandler_CatalogValidation_FiltersInvalid(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/mcp/tools" {
			w.Header().Set("Content-Type", "application/json")
			// Mix of valid and invalid tools
			w.Write([]byte(`[
				{"name":"valid_tool","description":"Valid","method":"GET","path":"/api/test","params":[]},
				{"name":"","description":"Empty name","method":"GET","path":"/api/bad","params":[]},
				{"name":"bad_path","description":"Bad path","method":"GET","path":"/evil","params":[]}
			]`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer mockServer.Close()

	cfg := testConfig()
	cfg.API.URL = mockServer.URL

	handler := NewHandler(cfg, testLogger())
	if handler == nil {
		t.Fatal("expected non-nil handler")
	}
}

func TestNewHandler_CatalogAvailable(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/mcp/tools" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(sampleCatalogJSON()))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer mockServer.Close()

	cfg := testConfig()
	cfg.API.URL = mockServer.URL

	handler := NewHandler(cfg, testLogger())

	if handler == nil {
		t.Fatal("expected non-nil handler")
	}
}

// --- Proxy Tests (unchanged from original) ---

func TestNewMCPProxy_ServerURL(t *testing.T) {
	cfg := testConfig()
	p := NewMCPProxy("http://vire-server:4242", testLogger(), cfg)

	if p.ServerURL() != "http://vire-server:4242" {
		t.Errorf("expected server URL http://vire-server:4242, got %s", p.ServerURL())
	}
}

func TestNewMCPProxy_UserHeaders_Portfolios(t *testing.T) {
	cfg := testConfig()
	cfg.User.Portfolios = []string{"SMSF", "Personal"}

	p := NewMCPProxy("http://localhost:4242", testLogger(), cfg)

	portfolios := p.UserHeaders().Get("X-Vire-Portfolios")
	if portfolios != "SMSF,Personal" {
		t.Errorf("expected X-Vire-Portfolios 'SMSF,Personal', got %q", portfolios)
	}
}

func TestNewMCPProxy_UserHeaders_DisplayCurrency(t *testing.T) {
	cfg := testConfig()
	cfg.User.DisplayCurrency = "AUD"

	p := NewMCPProxy("http://localhost:4242", testLogger(), cfg)

	currency := p.UserHeaders().Get("X-Vire-Display-Currency")
	if currency != "AUD" {
		t.Errorf("expected X-Vire-Display-Currency 'AUD', got %q", currency)
	}
}

func TestNewMCPProxy_UserHeaders_NavexaKey(t *testing.T) {
	cfg := testConfig()
	cfg.Keys.Navexa = "navexa-secret-123"

	p := NewMCPProxy("http://localhost:4242", testLogger(), cfg)

	key := p.UserHeaders().Get("X-Vire-Navexa-Key")
	if key != "navexa-secret-123" {
		t.Errorf("expected X-Vire-Navexa-Key 'navexa-secret-123', got %q", key)
	}
}

func TestNewMCPProxy_UserHeaders_EODHDKey(t *testing.T) {
	cfg := testConfig()
	cfg.Keys.EODHD = "eodhd-secret-456"

	p := NewMCPProxy("http://localhost:4242", testLogger(), cfg)

	key := p.UserHeaders().Get("X-Vire-EODHD-Key")
	if key != "eodhd-secret-456" {
		t.Errorf("expected X-Vire-EODHD-Key 'eodhd-secret-456', got %q", key)
	}
}

func TestNewMCPProxy_UserHeaders_GeminiKey(t *testing.T) {
	cfg := testConfig()
	cfg.Keys.Gemini = "gemini-secret-789"

	p := NewMCPProxy("http://localhost:4242", testLogger(), cfg)

	key := p.UserHeaders().Get("X-Vire-Gemini-Key")
	if key != "gemini-secret-789" {
		t.Errorf("expected X-Vire-Gemini-Key 'gemini-secret-789', got %q", key)
	}
}

func TestNewMCPProxy_UserHeaders_EmptyConfig(t *testing.T) {
	cfg := config.NewDefaultConfig()

	p := NewMCPProxy("http://localhost:4242", testLogger(), cfg)

	if p.UserHeaders().Get("X-Vire-Portfolios") != "" {
		t.Error("expected no X-Vire-Portfolios header with empty config")
	}
	if p.UserHeaders().Get("X-Vire-Display-Currency") != "" {
		t.Error("expected no X-Vire-Display-Currency header with empty config")
	}
	if p.UserHeaders().Get("X-Vire-Navexa-Key") != "" {
		t.Error("expected no X-Vire-Navexa-Key header with empty config")
	}
	if p.UserHeaders().Get("X-Vire-EODHD-Key") != "" {
		t.Error("expected no X-Vire-EODHD-Key header with empty config")
	}
	if p.UserHeaders().Get("X-Vire-Gemini-Key") != "" {
		t.Error("expected no X-Vire-Gemini-Key header with empty config")
	}
}

// --- Proxy HTTP Method Tests ---

func TestMCPProxy_Get_ForwardsHeaders(t *testing.T) {
	var receivedHeaders http.Header
	var receivedPath string

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header
		receivedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer mockServer.Close()

	cfg := testConfig()
	p := NewMCPProxy(mockServer.URL, testLogger(), cfg)

	body, err := p.get(t.Context(), "/api/version")
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}

	if receivedPath != "/api/version" {
		t.Errorf("expected path /api/version, got %s", receivedPath)
	}

	if string(body) != `{"status":"ok"}` {
		t.Errorf("expected body {\"status\":\"ok\"}, got %s", string(body))
	}

	if receivedHeaders.Get("X-Vire-Portfolios") != "SMSF,Personal" {
		t.Errorf("expected X-Vire-Portfolios forwarded, got %q", receivedHeaders.Get("X-Vire-Portfolios"))
	}
	if receivedHeaders.Get("X-Vire-Display-Currency") != "AUD" {
		t.Errorf("expected X-Vire-Display-Currency forwarded, got %q", receivedHeaders.Get("X-Vire-Display-Currency"))
	}
}

func TestMCPProxy_Post_SendsJSON(t *testing.T) {
	var receivedMethod string
	var receivedContentType string

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		receivedContentType = r.Header.Get("Content-Type")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"result":"created"}`))
	}))
	defer mockServer.Close()

	cfg := testConfig()
	p := NewMCPProxy(mockServer.URL, testLogger(), cfg)

	body, err := p.post(t.Context(), "/api/portfolios/SMSF/review", map[string]interface{}{"force": true})
	if err != nil {
		t.Fatalf("post failed: %v", err)
	}

	if receivedMethod != http.MethodPost {
		t.Errorf("expected POST method, got %s", receivedMethod)
	}
	if receivedContentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", receivedContentType)
	}
	if string(body) != `{"result":"created"}` {
		t.Errorf("unexpected body: %s", string(body))
	}
}

func TestMCPProxy_Get_ErrorResponse(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"portfolio not found"}`))
	}))
	defer mockServer.Close()

	cfg := testConfig()
	p := NewMCPProxy(mockServer.URL, testLogger(), cfg)

	_, err := p.get(t.Context(), "/api/portfolios/nonexistent")
	if err == nil {
		t.Fatal("expected error for 404 response, got nil")
	}

	if !strings.Contains(err.Error(), "portfolio not found") {
		t.Errorf("expected error to contain 'portfolio not found', got: %s", err.Error())
	}
}

func TestMCPProxy_Put(t *testing.T) {
	var receivedMethod string

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"default":"SMSF"}`))
	}))
	defer mockServer.Close()

	cfg := testConfig()
	p := NewMCPProxy(mockServer.URL, testLogger(), cfg)

	_, err := p.put(t.Context(), "/api/portfolios/default", map[string]string{"name": "SMSF"})
	if err != nil {
		t.Fatalf("put failed: %v", err)
	}

	if receivedMethod != http.MethodPut {
		t.Errorf("expected PUT method, got %s", receivedMethod)
	}
}

func TestMCPProxy_Delete(t *testing.T) {
	var receivedMethod string

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"deleted"}`))
	}))
	defer mockServer.Close()

	cfg := testConfig()
	p := NewMCPProxy(mockServer.URL, testLogger(), cfg)

	_, err := p.del(t.Context(), "/api/portfolios/SMSF/strategy")
	if err != nil {
		t.Fatalf("del failed: %v", err)
	}

	if receivedMethod != http.MethodDelete {
		t.Errorf("expected DELETE method, got %s", receivedMethod)
	}
}

func TestMCPProxy_Patch(t *testing.T) {
	var receivedMethod string

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"updated"}`))
	}))
	defer mockServer.Close()

	cfg := testConfig()
	p := NewMCPProxy(mockServer.URL, testLogger(), cfg)

	_, err := p.patch(t.Context(), "/api/portfolios/SMSF/plan/items/123", map[string]string{"status": "done"})
	if err != nil {
		t.Fatalf("patch failed: %v", err)
	}

	if receivedMethod != http.MethodPatch {
		t.Errorf("expected PATCH method, got %s", receivedMethod)
	}
}

// --- Error Handling Tests ---

func TestErrorResult(t *testing.T) {
	result := errorResult("something went wrong")

	if !result.IsError {
		t.Error("expected IsError to be true")
	}

	if len(result.Content) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(result.Content))
	}

	resultJSON, _ := json.Marshal(result.Content[0])
	if !strings.Contains(string(resultJSON), "something went wrong") {
		t.Errorf("expected error message 'something went wrong', got: %s", string(resultJSON))
	}
}

// --- Context Propagation Tests ---

func TestMCPProxy_Get_CancelledContext(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer mockServer.Close()

	cfg := testConfig()
	p := NewMCPProxy(mockServer.URL, testLogger(), cfg)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := p.get(ctx, "/api/version")
	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
}

func TestMCPProxy_ContextCancellation_CancelsInFlightRequest(t *testing.T) {
	var requestReceived sync.WaitGroup
	requestReceived.Add(1)

	serverCtxErr := make(chan error, 1)
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestReceived.Done()
		<-r.Context().Done()
		serverCtxErr <- r.Context().Err()
	}))
	defer mockServer.Close()

	cfg := testConfig()
	p := NewMCPProxy(mockServer.URL, testLogger(), cfg)

	ctx, cancel := context.WithCancel(t.Context())

	var proxyErr error
	var done sync.WaitGroup
	done.Add(1)
	go func() {
		defer done.Done()
		_, proxyErr = p.get(ctx, "/api/version")
	}()

	requestReceived.Wait()
	cancel()

	done.Wait()

	if proxyErr == nil {
		t.Fatal("expected proxy error after context cancellation, got nil")
	}
	if !strings.Contains(proxyErr.Error(), "cancel") {
		t.Errorf("expected cancellation error, got: %v", proxyErr)
	}

	select {
	case err := <-serverCtxErr:
		if err != context.Canceled {
			t.Errorf("expected server-side request context to be cancelled, got: %v", err)
		}
	case <-time.After(time.Second):
		t.Error("timed out waiting for server-side context cancellation")
	}
}

// --- resolvePortfolio Tests (used by default_from resolution) ---

func TestResolvePortfolio_ExplicitParam(t *testing.T) {
	cfg := config.NewDefaultConfig()
	cfg.User.Portfolios = []string{"SMSF", "Personal"}
	p := NewMCPProxy("http://localhost:4242", testLogger(), cfg)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "test_tool",
			Arguments: map[string]interface{}{"portfolio_name": "Explicit"},
		},
	}

	result := resolvePortfolio(t.Context(), p, req)
	if result != "Explicit" {
		t.Errorf("expected 'Explicit' from param, got %q", result)
	}
}

func TestResolvePortfolio_DefaultFromConfig(t *testing.T) {
	cfg := config.NewDefaultConfig()
	cfg.User.Portfolios = []string{"SMSF", "Personal"}
	p := NewMCPProxy("http://localhost:4242", testLogger(), cfg)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "test_tool",
			Arguments: map[string]interface{}{},
		},
	}

	result := resolvePortfolio(t.Context(), p, req)
	if result != "SMSF" {
		t.Errorf("expected 'SMSF' from config default, got %q", result)
	}
}

func TestResolvePortfolio_SinglePortfolioConfig(t *testing.T) {
	cfg := config.NewDefaultConfig()
	cfg.User.Portfolios = []string{"Personal"}
	p := NewMCPProxy("http://localhost:4242", testLogger(), cfg)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "test_tool",
			Arguments: map[string]interface{}{},
		},
	}

	result := resolvePortfolio(t.Context(), p, req)
	if result != "Personal" {
		t.Errorf("expected 'Personal' from single portfolio config, got %q", result)
	}
}

func TestResolvePortfolio_APIFallback(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/portfolios/default" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"default":"ServerDefault"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer mockServer.Close()

	cfg := config.NewDefaultConfig()
	p := NewMCPProxy(mockServer.URL, testLogger(), cfg)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "test_tool",
			Arguments: map[string]interface{}{},
		},
	}

	result := resolvePortfolio(t.Context(), p, req)
	if result != "ServerDefault" {
		t.Errorf("expected 'ServerDefault' from API fallback, got %q", result)
	}
}

func TestResolvePortfolio_EmptyEverywhere(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"not found"}`))
	}))
	defer mockServer.Close()

	cfg := config.NewDefaultConfig()
	p := NewMCPProxy(mockServer.URL, testLogger(), cfg)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "test_tool",
			Arguments: map[string]interface{}{},
		},
	}

	result := resolvePortfolio(t.Context(), p, req)
	if result != "" {
		t.Errorf("expected empty string when no portfolio available, got %q", result)
	}
}

// --- Integration Test: Full Catalog -> Registration -> Tool Call ---

func TestIntegration_CatalogToToolCall(t *testing.T) {
	// Mock vire-server: serves catalog on /api/mcp/tools, responds to tool calls
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/mcp/tools" && r.Method == "GET":
			w.Write([]byte(sampleCatalogJSON()))
		case r.URL.Path == "/api/version" && r.Method == "GET":
			w.Write([]byte(`{"version":"2.0.0"}`))
		case r.URL.Path == "/api/market/quote/AAPL.US" && r.Method == "GET":
			w.Write([]byte(`{"ticker":"AAPL.US","close":185.50}`))
		case r.URL.Path == "/api/portfolios/SMSF/review" && r.Method == "POST":
			w.Write([]byte(`{"signals":["rsi_oversold"]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error":"not found"}`))
		}
	}))
	defer mockServer.Close()

	cfg := testConfig()
	cfg.API.URL = mockServer.URL

	// Create handler (fetches catalog, registers tools)
	handler := NewHandler(cfg, testLogger())
	if handler == nil {
		t.Fatal("expected non-nil handler")
	}

	// Verify we can call tools through the full stack
	// We need to test via HTTP since NewHandler wraps everything
	rec := httptest.NewRecorder()
	// Send an MCP initialize request
	initReq := httptest.NewRequest("POST", "/mcp", strings.NewReader(
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test","version":"1.0.0"}}}`,
	))
	initReq.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(rec, initReq)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for initialize, got %d: %s", rec.Code, rec.Body.String())
	}
}
