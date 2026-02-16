package auth

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// --- Method enforcement ---

func TestAuthorizationServer_AllMethodsExceptGET(t *testing.T) {
	h := NewDiscoveryHandler("http://localhost:4241")
	methods := []string{
		http.MethodPost, http.MethodPut, http.MethodDelete,
		http.MethodPatch, http.MethodOptions,
	}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/.well-known/oauth-authorization-server", nil)
			rec := httptest.NewRecorder()
			h.HandleAuthorizationServer(rec, req)

			if rec.Code != http.StatusMethodNotAllowed {
				t.Errorf("%s: expected 405, got %d", method, rec.Code)
			}
		})
	}
}

func TestProtectedResource_AllMethodsExceptGET(t *testing.T) {
	h := NewDiscoveryHandler("http://localhost:4241")
	methods := []string{
		http.MethodPost, http.MethodPut, http.MethodDelete,
		http.MethodPatch, http.MethodOptions,
	}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/.well-known/oauth-protected-resource", nil)
			rec := httptest.NewRecorder()
			h.HandleProtectedResource(rec, req)

			if rec.Code != http.StatusMethodNotAllowed {
				t.Errorf("%s: expected 405, got %d", method, rec.Code)
			}
		})
	}
}

func TestAuthorizationServer_HEADRequest(t *testing.T) {
	// HEAD should be accepted by the handler since it is registered as "GET /..."
	// on the mux, and Go's ServeMux routes HEAD requests to GET handlers.
	// The handler must allow HEAD to avoid 405 when served through the mux.
	h := NewDiscoveryHandler("http://localhost:4241")
	req := httptest.NewRequest(http.MethodHead, "/.well-known/oauth-authorization-server", nil)
	rec := httptest.NewRecorder()
	h.HandleAuthorizationServer(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("HEAD direct call: expected 200, got %d", rec.Code)
	}
}

// --- URL injection via baseURL ---

func TestAuthorizationServer_MaliciousBaseURL_JavascriptProtocol(t *testing.T) {
	// If VIRE_PORTAL_URL is set to a javascript: URI, the JSON response
	// should contain it verbatim. This isn't a direct XSS vector (it's JSON,
	// not HTML), but document that the value passes through unvalidated.
	h := NewDiscoveryHandler("javascript:alert(1)")
	req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", nil)
	rec := httptest.NewRecorder()
	h.HandleAuthorizationServer(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(rec.Result().Body).Decode(&body); err != nil {
		t.Fatalf("JSON decode failed: %v", err)
	}

	// The issuer should contain the injected value verbatim —
	// this test documents the behavior so we can add validation later if needed.
	if body["issuer"] != "javascript:alert(1)" {
		t.Errorf("expected injected issuer verbatim, got %v", body["issuer"])
	}
	if body["authorization_endpoint"] != "javascript:alert(1)/authorize" {
		t.Errorf("expected injected endpoint, got %v", body["authorization_endpoint"])
	}
}

func TestAuthorizationServer_BaseURLWithQueryParams(t *testing.T) {
	// A baseURL containing query params could create malformed endpoint URLs.
	h := NewDiscoveryHandler("http://evil.com?redirect=http://attacker.com")
	req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", nil)
	rec := httptest.NewRecorder()
	h.HandleAuthorizationServer(rec, req)

	var body map[string]interface{}
	if err := json.NewDecoder(rec.Result().Body).Decode(&body); err != nil {
		t.Fatalf("JSON decode failed: %v", err)
	}

	// The endpoint becomes "http://evil.com?redirect=http://attacker.com/authorize"
	// which is malformed (path after query). Document this behavior.
	endpoint := body["authorization_endpoint"].(string)
	if !strings.Contains(endpoint, "/authorize") {
		t.Errorf("endpoint should still contain /authorize suffix, got %s", endpoint)
	}
}

func TestAuthorizationServer_BaseURLWithFragment(t *testing.T) {
	h := NewDiscoveryHandler("http://example.com#fragment")
	req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", nil)
	rec := httptest.NewRecorder()
	h.HandleAuthorizationServer(rec, req)

	var body map[string]interface{}
	if err := json.NewDecoder(rec.Result().Body).Decode(&body); err != nil {
		t.Fatalf("JSON decode failed: %v", err)
	}

	// "/authorize" is appended after the fragment, which is malformed
	endpoint := body["authorization_endpoint"].(string)
	if endpoint != "http://example.com#fragment/authorize" {
		t.Errorf("unexpected endpoint: %s", endpoint)
	}
}

func TestAuthorizationServer_BaseURLWithUserinfo(t *testing.T) {
	// Userinfo in URL: http://admin:password@host — should pass through
	h := NewDiscoveryHandler("http://admin:password@example.com")
	req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", nil)
	rec := httptest.NewRecorder()
	h.HandleAuthorizationServer(rec, req)

	var body map[string]interface{}
	if err := json.NewDecoder(rec.Result().Body).Decode(&body); err != nil {
		t.Fatalf("JSON decode failed: %v", err)
	}

	if body["issuer"] != "http://admin:password@example.com" {
		t.Errorf("expected userinfo to pass through, got %v", body["issuer"])
	}
}

func TestAuthorizationServer_BaseURLWithHTMLEntities(t *testing.T) {
	// If someone sets baseURL to contain HTML, json.Encoder should escape it
	h := NewDiscoveryHandler(`http://example.com/<script>alert(1)</script>`)
	req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", nil)
	rec := httptest.NewRecorder()
	h.HandleAuthorizationServer(rec, req)

	// json.Encoder escapes < > & by default
	bodyBytes := rec.Body.Bytes()
	if bytes.Contains(bodyBytes, []byte("<script>")) {
		t.Error("JSON response contains unescaped <script> tag — json.Encoder should escape angle brackets")
	}
	// Verify it's still valid JSON
	var body map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
}

// --- Empty / degenerate baseURL ---

func TestAuthorizationServer_EmptyBaseURL(t *testing.T) {
	h := NewDiscoveryHandler("")
	req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", nil)
	rec := httptest.NewRecorder()
	h.HandleAuthorizationServer(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(rec.Result().Body).Decode(&body); err != nil {
		t.Fatalf("JSON decode failed: %v", err)
	}

	// Empty baseURL means endpoints become "/authorize", "/token", etc.
	if body["issuer"] != "" {
		t.Errorf("expected empty issuer, got %v", body["issuer"])
	}
	if body["authorization_endpoint"] != "/authorize" {
		t.Errorf("expected /authorize, got %v", body["authorization_endpoint"])
	}
}

func TestProtectedResource_EmptyBaseURL(t *testing.T) {
	h := NewDiscoveryHandler("")
	req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-protected-resource", nil)
	rec := httptest.NewRecorder()
	h.HandleProtectedResource(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(rec.Result().Body).Decode(&body); err != nil {
		t.Fatalf("JSON decode failed: %v", err)
	}

	if body["resource"] != "" {
		t.Errorf("expected empty resource, got %v", body["resource"])
	}
}

func TestAuthorizationServer_BaseURLTrailingSlash(t *testing.T) {
	// If BaseURL() doesn't strip trailing slash, endpoints become
	// "http://localhost:4241//authorize" (double slash).
	// BaseURL() is supposed to TrimRight("/") but test the handler directly.
	h := NewDiscoveryHandler("http://localhost:4241/")
	req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", nil)
	rec := httptest.NewRecorder()
	h.HandleAuthorizationServer(rec, req)

	var body map[string]interface{}
	if err := json.NewDecoder(rec.Result().Body).Decode(&body); err != nil {
		t.Fatalf("JSON decode failed: %v", err)
	}

	// Double slash is incorrect — this documents the handler's behavior.
	// The fix belongs in BaseURL() or NewDiscoveryHandler.
	endpoint := body["authorization_endpoint"].(string)
	if strings.Contains(endpoint, "//authorize") {
		t.Errorf("trailing slash in baseURL creates double-slash endpoint: %s", endpoint)
	}
}

func TestAuthorizationServer_BaseURLWhitespace(t *testing.T) {
	h := NewDiscoveryHandler("  http://localhost:4241  ")
	req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", nil)
	rec := httptest.NewRecorder()
	h.HandleAuthorizationServer(rec, req)

	var body map[string]interface{}
	if err := json.NewDecoder(rec.Result().Body).Decode(&body); err != nil {
		t.Fatalf("JSON decode failed: %v", err)
	}

	// Whitespace in issuer is incorrect per RFC 8414
	issuer := body["issuer"].(string)
	if strings.TrimSpace(issuer) != issuer {
		t.Errorf("issuer has leading/trailing whitespace: %q", issuer)
	}
}

// --- Large request body on GET ---

func TestAuthorizationServer_LargeRequestBody(t *testing.T) {
	// A client should not be able to OOM the server by sending a huge body
	// on a GET request. The handler doesn't read the body, so this should
	// be fine, but verify it doesn't panic or hang.
	h := NewDiscoveryHandler("http://localhost:4241")
	largeBody := bytes.NewReader(make([]byte, 10*1024*1024)) // 10MB
	req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", largeBody)
	rec := httptest.NewRecorder()

	h.HandleAuthorizationServer(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 despite large body, got %d", rec.Code)
	}
}

func TestProtectedResource_LargeRequestBody(t *testing.T) {
	h := NewDiscoveryHandler("http://localhost:4241")
	largeBody := bytes.NewReader(make([]byte, 10*1024*1024))
	req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-protected-resource", largeBody)
	rec := httptest.NewRecorder()

	h.HandleProtectedResource(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 despite large body, got %d", rec.Code)
	}
}

// --- Concurrent access ---

func TestAuthorizationServer_ConcurrentAccess(t *testing.T) {
	h := NewDiscoveryHandler("http://localhost:4241")
	const goroutines = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)

	errors := make(chan string, goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", nil)
			rec := httptest.NewRecorder()
			h.HandleAuthorizationServer(rec, req)

			if rec.Code != http.StatusOK {
				errors <- "non-200 status"
				return
			}

			var body map[string]interface{}
			if err := json.NewDecoder(rec.Result().Body).Decode(&body); err != nil {
				errors <- "invalid JSON"
				return
			}

			if body["issuer"] != "http://localhost:4241" {
				errors <- "wrong issuer"
			}
		}()
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("concurrent access error: %s", err)
	}
}

func TestProtectedResource_ConcurrentAccess(t *testing.T) {
	h := NewDiscoveryHandler("http://localhost:4241")
	const goroutines = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)

	errors := make(chan string, goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-protected-resource", nil)
			rec := httptest.NewRecorder()
			h.HandleProtectedResource(rec, req)

			if rec.Code != http.StatusOK {
				errors <- "non-200 status"
				return
			}

			var body map[string]interface{}
			if err := json.NewDecoder(rec.Result().Body).Decode(&body); err != nil {
				errors <- "invalid JSON"
				return
			}

			if body["resource"] != "http://localhost:4241" {
				errors <- "wrong resource"
			}
		}()
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("concurrent access error: %s", err)
	}
}

// --- RFC 8414 field completeness ---

func TestAuthorizationServer_RFC8414RequiredFields(t *testing.T) {
	h := NewDiscoveryHandler("http://localhost:4241")
	req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", nil)
	rec := httptest.NewRecorder()
	h.HandleAuthorizationServer(rec, req)

	var body map[string]interface{}
	if err := json.NewDecoder(rec.Result().Body).Decode(&body); err != nil {
		t.Fatalf("JSON decode failed: %v", err)
	}

	// RFC 8414 Section 2: REQUIRED fields
	requiredFields := []string{
		"issuer",
		"authorization_endpoint",
		"token_endpoint",
		"response_types_supported",
	}

	for _, field := range requiredFields {
		if _, ok := body[field]; !ok {
			t.Errorf("missing RFC 8414 REQUIRED field: %s", field)
		}
	}

	// Fields that the spec includes (OPTIONAL but expected by Claude MCP)
	expectedFields := []string{
		"registration_endpoint",
		"grant_types_supported",
		"code_challenge_methods_supported",
		"token_endpoint_auth_methods_supported",
		"scopes_supported",
	}

	for _, field := range expectedFields {
		if _, ok := body[field]; !ok {
			t.Errorf("missing expected field: %s", field)
		}
	}
}

func TestProtectedResource_RFC9470RequiredFields(t *testing.T) {
	h := NewDiscoveryHandler("http://localhost:4241")
	req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-protected-resource", nil)
	rec := httptest.NewRecorder()
	h.HandleProtectedResource(rec, req)

	var body map[string]interface{}
	if err := json.NewDecoder(rec.Result().Body).Decode(&body); err != nil {
		t.Fatalf("JSON decode failed: %v", err)
	}

	requiredFields := []string{
		"resource",
		"authorization_servers",
		"scopes_supported",
	}

	for _, field := range requiredFields {
		if _, ok := body[field]; !ok {
			t.Errorf("missing required field: %s", field)
		}
	}
}

// --- JSON validity and encoding ---

func TestAuthorizationServer_ValidJSON(t *testing.T) {
	h := NewDiscoveryHandler("http://localhost:4241")
	req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", nil)
	rec := httptest.NewRecorder()
	h.HandleAuthorizationServer(rec, req)

	bodyBytes := rec.Body.Bytes()

	// Must be valid JSON
	if !json.Valid(bodyBytes) {
		t.Error("response is not valid JSON")
	}

	// json.Encoder adds a trailing newline; verify it's there
	if len(bodyBytes) == 0 {
		t.Fatal("empty response body")
	}
	if bodyBytes[len(bodyBytes)-1] != '\n' {
		t.Error("json.Encoder should append trailing newline")
	}

	// There should be exactly one JSON object (no extra data after the newline)
	trimmed := bytes.TrimSpace(bodyBytes)
	if !json.Valid(trimmed) {
		t.Error("trimmed response is not valid JSON")
	}
}

func TestProtectedResource_ValidJSON(t *testing.T) {
	h := NewDiscoveryHandler("http://localhost:4241")
	req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-protected-resource", nil)
	rec := httptest.NewRecorder()
	h.HandleProtectedResource(rec, req)

	bodyBytes := rec.Body.Bytes()

	if !json.Valid(bodyBytes) {
		t.Error("response is not valid JSON")
	}
}

// --- 405 response quality ---

func TestAuthorizationServer_405NoBody(t *testing.T) {
	// 405 responses should ideally be empty or minimal.
	// Verify the handler doesn't write a confusing body on 405.
	h := NewDiscoveryHandler("http://localhost:4241")
	req := httptest.NewRequest(http.MethodPost, "/.well-known/oauth-authorization-server", nil)
	rec := httptest.NewRecorder()
	h.HandleAuthorizationServer(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}

	// Body should be empty for a bare 405
	if rec.Body.Len() != 0 {
		t.Logf("405 response has body: %q (not necessarily wrong, but worth noting)", rec.Body.String())
	}
}

// --- Mux-level integration test ---

func TestDiscovery_MuxIntegration(t *testing.T) {
	// Test the endpoints as they would be registered on a real mux.
	// The "GET /..." pattern means the mux rejects non-GET methods with 405
	// before the handler even runs.
	h := NewDiscoveryHandler("http://localhost:4241")

	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/oauth-authorization-server", h.HandleAuthorizationServer)
	mux.HandleFunc("GET /.well-known/oauth-protected-resource", h.HandleProtectedResource)

	server := httptest.NewServer(mux)
	defer server.Close()

	// GET should work
	resp, err := http.Get(server.URL + "/.well-known/oauth-authorization-server")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET: expected 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("GET: expected Content-Type application/json, got %s", ct)
	}

	// POST should be rejected at mux level
	postResp, err := http.Post(server.URL+"/.well-known/oauth-authorization-server", "application/json", nil)
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	defer postResp.Body.Close()

	if postResp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("POST via mux: expected 405, got %d", postResp.StatusCode)
	}

	// HEAD should work (mux handles HEAD for GET routes)
	headReq, _ := http.NewRequest(http.MethodHead, server.URL+"/.well-known/oauth-authorization-server", nil)
	headResp, err := http.DefaultClient.Do(headReq)
	if err != nil {
		t.Fatalf("HEAD failed: %v", err)
	}
	defer headResp.Body.Close()

	if headResp.StatusCode != http.StatusOK {
		t.Errorf("HEAD via mux: expected 200, got %d", headResp.StatusCode)
	}
}

func TestDiscovery_MuxIntegration_ProtectedResource(t *testing.T) {
	h := NewDiscoveryHandler("http://localhost:4241")

	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/oauth-protected-resource", h.HandleProtectedResource)

	server := httptest.NewServer(mux)
	defer server.Close()

	resp, err := http.Get(server.URL + "/.well-known/oauth-protected-resource")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET: expected 200, got %d", resp.StatusCode)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("JSON decode failed: %v", err)
	}

	if body["resource"] != "http://localhost:4241" {
		t.Errorf("expected resource http://localhost:4241, got %v", body["resource"])
	}
}

// --- Scope consistency ---

func TestDiscovery_ScopeConsistency(t *testing.T) {
	// Protected resource scopes should be a subset of authorization server scopes.
	h := NewDiscoveryHandler("http://localhost:4241")

	// Get auth server scopes
	authReq := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", nil)
	authRec := httptest.NewRecorder()
	h.HandleAuthorizationServer(authRec, authReq)

	var authBody map[string]interface{}
	json.NewDecoder(authRec.Result().Body).Decode(&authBody)
	authScopes := toStringSlice(t, authBody, "scopes_supported")

	// Get protected resource scopes
	resReq := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-protected-resource", nil)
	resRec := httptest.NewRecorder()
	h.HandleProtectedResource(resRec, resReq)

	var resBody map[string]interface{}
	json.NewDecoder(resRec.Result().Body).Decode(&resBody)
	resScopes := toStringSlice(t, resBody, "scopes_supported")

	// Every scope in protected resource should exist in auth server
	authScopeSet := make(map[string]bool)
	for _, s := range authScopes {
		authScopeSet[s] = true
	}

	for _, scope := range resScopes {
		if !authScopeSet[scope] {
			t.Errorf("protected resource scope %q not present in authorization server scopes", scope)
		}
	}
}

// --- JSON key ordering stability (not required, but nice for caching) ---

func TestAuthorizationServer_JSONDeterministic(t *testing.T) {
	// map[string]interface{} does NOT guarantee key order in Go.
	// This test documents that repeated calls may produce different byte output.
	// If cache-friendliness matters, the handler should use a struct instead.
	h := NewDiscoveryHandler("http://localhost:4241")

	var outputs []string
	for i := 0; i < 20; i++ {
		req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", nil)
		rec := httptest.NewRecorder()
		h.HandleAuthorizationServer(rec, req)
		outputs = append(outputs, rec.Body.String())
	}

	// Check all outputs parse to the same logical content
	var reference map[string]interface{}
	json.Unmarshal([]byte(outputs[0]), &reference)

	for i, out := range outputs[1:] {
		var current map[string]interface{}
		json.Unmarshal([]byte(out), &current)

		// Compare field count
		if len(current) != len(reference) {
			t.Errorf("iteration %d: field count mismatch: %d vs %d", i+1, len(current), len(reference))
		}
	}

	// Check if byte-level output varies (non-deterministic map iteration)
	allSame := true
	for _, out := range outputs[1:] {
		if out != outputs[0] {
			allSame = false
			break
		}
	}

	if !allSame {
		t.Log("NOTE: JSON output is not byte-deterministic due to map[string]interface{} usage. " +
			"Consider using a struct for stable output if caching matters.")
	}
}
