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
			w.Write([]byte("<html>dashboard</html>"))
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
	if !strings.Contains(text, "<html>dashboard</html>") {
		t.Errorf("expected HTML content, got %s", text)
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

func TestGetPageToolHandler_AllPages(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("<html>" + r.URL.Path + "</html>"))
	}))
	defer srv.Close()

	handler := GetPageToolHandler(srv.URL, []byte("secret"))
	ctx := WithUserContext(context.Background(), UserContext{UserID: "user123"})

	for page := range allowedPages {
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

	attacks := []string{"../admin", "admin/users", "/admin"}
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
		w.Write([]byte("<html>ok</html>"))
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
