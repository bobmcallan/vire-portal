package mcp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

func TestGetPageTool_Definition(t *testing.T) {
	tool := GetPageTool()
	if tool.Name != "portal_get_page" {
		t.Errorf("expected tool name portal_get_page, got %s", tool.Name)
	}
	if tool.Description == "" {
		t.Error("tool description should not be empty")
	}
	schema := tool.InputSchema
	props, ok := schema.Properties["page"]
	if !ok {
		t.Fatal("expected page parameter in schema")
	}
	propMap, ok := props.(map[string]interface{})
	if !ok {
		t.Fatal("expected page property to be a map")
	}
	if propMap["type"] != "string" {
		t.Errorf("expected page type string, got %v", propMap["type"])
	}
}

func TestGetPageToolHandler_ValidPage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/dashboard" {
			w.Write([]byte("<html><body><main class=\"page\">Dashboard Content</main></body></html>"))
			return
		}
		w.WriteHeader(404)
	}))
	defer srv.Close()

	handler := GetPageToolHandler(srv.URL, []byte("test-secret"))
	ctx := WithUserContext(context.Background(), UserContext{UserID: "user123"})

	req := mcpgo.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{"page": "dashboard"}

	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	text := result.Content[0].(mcpgo.TextContent).Text
	if !strings.Contains(text, "Dashboard Content") {
		t.Errorf("expected extracted text content, got %s", text)
	}
	// Should NOT contain raw HTML tags
	if strings.Contains(text, "<main") {
		t.Error("expected HTML tags to be stripped")
	}
}

func TestGetPageToolHandler_InvalidPage(t *testing.T) {
	handler := GetPageToolHandler("http://localhost:1", []byte("secret"))
	ctx := WithUserContext(context.Background(), UserContext{UserID: "user123"})

	req := mcpgo.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{"page": "admin"}

	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for invalid page")
	}
}

func TestGetPageToolHandler_EmptyPage(t *testing.T) {
	handler := GetPageToolHandler("http://localhost:1", []byte("secret"))
	ctx := WithUserContext(context.Background(), UserContext{UserID: "user123"})

	req := mcpgo.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{"page": ""}

	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for empty page")
	}
}

func TestGetPageToolHandler_NoUserContext(t *testing.T) {
	handler := GetPageToolHandler("http://localhost:1", []byte("secret"))

	req := mcpgo.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{"page": "dashboard"}

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for missing user context")
	}
}

func TestGetPageToolHandler_PortalUnavailable(t *testing.T) {
	handler := GetPageToolHandler("http://localhost:1", []byte("secret"))
	ctx := WithUserContext(context.Background(), UserContext{UserID: "user123"})

	req := mcpgo.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{"page": "dashboard"}

	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result when portal is unavailable")
	}
}

func TestGetPageToolHandler_NonOKResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte("internal server error"))
	}))
	defer srv.Close()

	handler := GetPageToolHandler(srv.URL, []byte("secret"))
	ctx := WithUserContext(context.Background(), UserContext{UserID: "user123"})

	req := mcpgo.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{"page": "dashboard"}

	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for non-OK response")
	}
	text := result.Content[0].(mcpgo.TextContent).Text
	if !strings.Contains(text, "500") {
		t.Errorf("expected status code in error, got %s", text)
	}
}

func TestMintLoopbackJWT_Valid(t *testing.T) {
	secret := []byte("test-secret")
	token, err := mintLoopbackJWT("user123", secret)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("expected 3 JWT parts, got %d", len(parts))
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}

	var claims struct {
		Sub string `json:"sub"`
		Iss string `json:"iss"`
		Iat int64  `json:"iat"`
		Exp int64  `json:"exp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		t.Fatalf("failed to unmarshal claims: %v", err)
	}

	if claims.Sub != "user123" {
		t.Errorf("expected sub user123, got %s", claims.Sub)
	}
	if claims.Iss != "vire-portal-loopback" {
		t.Errorf("expected iss vire-portal-loopback, got %s", claims.Iss)
	}
	if claims.Iat == 0 {
		t.Error("iat should not be zero")
	}
	if claims.Exp == 0 {
		t.Error("exp should not be zero")
	}
}

func TestMintLoopbackJWT_ShortExpiry(t *testing.T) {
	token, err := mintLoopbackJWT("user123", []byte("secret"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	parts := strings.Split(token, ".")
	payload, _ := base64.RawURLEncoding.DecodeString(parts[1])

	var claims struct {
		Iat int64 `json:"iat"`
		Exp int64 `json:"exp"`
	}
	json.Unmarshal(payload, &claims)

	if claims.Exp-claims.Iat != 30 {
		t.Errorf("expected 30s TTL, got %d", claims.Exp-claims.Iat)
	}
}

func TestMintLoopbackJWT_EmptySecret(t *testing.T) {
	token, err := mintLoopbackJWT("user123", []byte{})
	if err != nil {
		t.Fatalf("unexpected error with empty secret: %v", err)
	}
	if token == "" {
		t.Error("expected non-empty token")
	}
}

func TestGetPageToolHandler_AllStaticPages(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("<html><body><main>" + r.URL.Path + "</main></body></html>"))
	}))
	defer srv.Close()

	handler := GetPageToolHandler(srv.URL, []byte("secret"))
	ctx := WithUserContext(context.Background(), UserContext{UserID: "user123"})

	for page := range staticPages {
		req := mcpgo.CallToolRequest{}
		req.Params.Arguments = map[string]interface{}{"page": page}

		result, err := handler(ctx, req)
		if err != nil {
			t.Fatalf("page %s: unexpected error: %v", page, err)
		}
		if result.IsError {
			t.Errorf("page %s: unexpected tool error", page)
		}
	}
}

func TestGetPageToolHandler_TraversalAttempt(t *testing.T) {
	handler := GetPageToolHandler("http://localhost:1", []byte("secret"))
	ctx := WithUserContext(context.Background(), UserContext{UserID: "user123"})

	attacks := []string{"../admin", "/admin", "stock/../admin", "stock/", "stock/../../etc/passwd"}
	for _, page := range attacks {
		req := mcpgo.CallToolRequest{}
		req.Params.Arguments = map[string]interface{}{"page": page}

		result, err := handler(ctx, req)
		if err != nil {
			t.Fatalf("page %q: unexpected error: %v", page, err)
		}
		if !result.IsError {
			t.Errorf("page %q: expected error result for traversal attempt", page)
		}
	}
}

func TestGetPageToolHandler_CookieSet(t *testing.T) {
	var receivedCookie string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("vire_session")
		if err == nil {
			receivedCookie = cookie.Value
		}
		w.Write([]byte("<html><body><main>ok</main></body></html>"))
	}))
	defer srv.Close()

	handler := GetPageToolHandler(srv.URL, []byte("secret"))
	ctx := WithUserContext(context.Background(), UserContext{UserID: "user123"})

	req := mcpgo.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{"page": "dashboard"}

	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if receivedCookie == "" {
		t.Error("expected vire_session cookie to be set")
	}
	// Verify it's a valid JWT
	parts := strings.Split(receivedCookie, ".")
	if len(parts) != 3 {
		t.Errorf("expected JWT cookie with 3 parts, got %d", len(parts))
	}
}

func TestGetPageToolHandler_StockPage(t *testing.T) {
	var requestedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		w.Write([]byte("<html><body><main>Stock Detail for EHL</main></body></html>"))
	}))
	defer srv.Close()

	handler := GetPageToolHandler(srv.URL, []byte("secret"))
	ctx := WithUserContext(context.Background(), UserContext{UserID: "user123"})

	req := mcpgo.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{"page": "stock/EHL"}

	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if requestedPath != "/stock/EHL" {
		t.Errorf("expected path /stock/EHL, got %s", requestedPath)
	}
	text := result.Content[0].(mcpgo.TextContent).Text
	if !strings.Contains(text, "Stock Detail for EHL") {
		t.Errorf("expected stock content, got %s", text)
	}
}

func TestGetPageToolHandler_StockPageWithExchange(t *testing.T) {
	var requestedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		w.Write([]byte("<html><body><main>CME US</main></body></html>"))
	}))
	defer srv.Close()

	handler := GetPageToolHandler(srv.URL, []byte("secret"))
	ctx := WithUserContext(context.Background(), UserContext{UserID: "user123"})

	req := mcpgo.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{"page": "stock/CME.US"}

	result, err := handler(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected tool error: %v", result.Content)
	}
	if requestedPath != "/stock/CME.US" {
		t.Errorf("expected path /stock/CME.US, got %s", requestedPath)
	}
}

func TestGetPageToolHandler_StockPageInvalidTicker(t *testing.T) {
	handler := GetPageToolHandler("http://localhost:1", []byte("secret"))
	ctx := WithUserContext(context.Background(), UserContext{UserID: "user123"})

	invalid := []string{
		"stock/",
		"stock/../admin",
		"stock/A B C",
		"stock/../../etc/passwd",
		"stock/TOOLONGTICKERSYMBOL",
		"stock/123",
	}
	for _, page := range invalid {
		req := mcpgo.CallToolRequest{}
		req.Params.Arguments = map[string]interface{}{"page": page}

		result, err := handler(ctx, req)
		if err != nil {
			t.Fatalf("page %q: unexpected error: %v", page, err)
		}
		if !result.IsError {
			t.Errorf("page %q: expected error result for invalid ticker", page)
		}
	}
}

func TestResolvePage_StaticPages(t *testing.T) {
	for name, expectedPath := range staticPages {
		path, ok := resolvePage(name)
		if !ok {
			t.Errorf("resolvePage(%q) returned false", name)
		}
		if path != expectedPath {
			t.Errorf("resolvePage(%q) = %q, want %q", name, path, expectedPath)
		}
	}
}

func TestResolvePage_StockPages(t *testing.T) {
	tests := []struct {
		page     string
		wantPath string
		wantOK   bool
	}{
		{"stock/EHL", "/stock/EHL", true},
		{"stock/CBOE", "/stock/CBOE", true},
		{"stock/CME.US", "/stock/CME.US", true},
		{"stock/DFND.AU", "/stock/DFND.AU", true},
		{"stock/", "", false},
		{"stock/../admin", "", false},
		{"stock/123", "", false},
		{"stock/A B", "", false},
		{"stocks/EHL", "", false},
	}
	for _, tt := range tests {
		path, ok := resolvePage(tt.page)
		if ok != tt.wantOK {
			t.Errorf("resolvePage(%q) ok = %v, want %v", tt.page, ok, tt.wantOK)
		}
		if path != tt.wantPath {
			t.Errorf("resolvePage(%q) = %q, want %q", tt.page, path, tt.wantPath)
		}
	}
}

func TestExtractMainContent(t *testing.T) {
	tests := []struct {
		name     string
		html     string
		contains []string
		excludes []string
	}{
		{
			name:     "extracts main content",
			html:     `<html><head><style>body{}</style></head><body><nav>Nav</nav><main class="page">Hello World</main></body></html>`,
			contains: []string{"Hello World"},
			excludes: []string{"<main", "<nav", "Nav", "body{}"},
		},
		{
			name:     "strips scripts",
			html:     `<main><p>Content</p><script>alert('xss')</script><p>More</p></main>`,
			contains: []string{"Content", "More"},
			excludes: []string{"<script", "alert", "<p>"},
		},
		{
			name:     "strips styles",
			html:     `<main><style>.foo{color:red}</style><p>Visible</p></main>`,
			contains: []string{"Visible"},
			excludes: []string{"color:red", "<style"},
		},
		{
			name:     "strips SVG",
			html:     `<main><svg xmlns="..." viewBox="0 0 100 100"><path d="M10 10"/></svg><p>Text</p></main>`,
			contains: []string{"Text"},
			excludes: []string{"<svg", "path", "viewBox"},
		},
		{
			name:     "decodes entities",
			html:     `<main>&amp; &lt;tag&gt; &quot;quoted&quot;</main>`,
			contains: []string{`& <tag> "quoted"`},
		},
		{
			name:     "no main tag fallback",
			html:     `<html><body>Fallback content</body></html>`,
			contains: []string{"Fallback content"},
		},
		{
			name:     "collapses whitespace",
			html:     "<main><p>Line 1</p>\n\n\n\n\n<p>Line 2</p></main>",
			contains: []string{"Line 1", "Line 2"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractMainContent(tt.html)
			for _, want := range tt.contains {
				if !strings.Contains(result, want) {
					t.Errorf("expected result to contain %q, got:\n%s", want, result)
				}
			}
			for _, exclude := range tt.excludes {
				if strings.Contains(result, exclude) {
					t.Errorf("expected result NOT to contain %q, got:\n%s", exclude, result)
				}
			}
		})
	}
}
