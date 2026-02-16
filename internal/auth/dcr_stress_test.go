package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func newStressOAuthServer() *OAuthServer {
	return NewOAuthServer("http://localhost:4241", []byte("test-secret-32-bytes-long!!!!!!"), nil)
}

// =============================================================================
// DCR (Dynamic Client Registration) Stress Tests
// =============================================================================

// --- DCR abuse: unlimited client registrations ---

func TestDCR_StressUnlimitedRegistrations(t *testing.T) {
	srv := newStressOAuthServer()

	const registrations = 500
	for i := 0; i < registrations; i++ {
		body := fmt.Sprintf(`{"client_name":"bot-%d","redirect_uris":["http://localhost/%d/callback"]}`, i, i)
		req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(body))
		rec := httptest.NewRecorder()
		srv.HandleRegister(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("registration %d failed: %d", i, rec.Code)
		}
	}

	t.Logf("FINDING: %d clients registered without rate limiting. "+
		"POST /register has no IP-based rate limit, no CAPTCHA, and no max-client cap. "+
		"An attacker can register thousands of clients to exhaust memory.", registrations)
}

// --- DCR: hostile redirect URIs ---

func TestDCR_StressHostileRedirectURIs(t *testing.T) {
	srv := newStressOAuthServer()

	hostile := []struct {
		name string
		uris []string
	}{
		{"javascript protocol", []string{"javascript:alert(document.cookie)"}},
		{"data URI", []string{"data:text/html,<script>alert(1)</script>"}},
		{"file protocol", []string{"file:///etc/passwd"}},
		{"empty string", []string{""}},
		{"very long URI", []string{"http://localhost/" + strings.Repeat("a", 1<<16)}},
		{"with fragment", []string{"http://localhost/callback#fragment"}},
		{"with query", []string{"http://localhost/callback?extra=param"}},
		{"ftp protocol", []string{"ftp://attacker.com/steal"}},
		{"ssh protocol", []string{"ssh://attacker.com"}},
		{"protocol-relative", []string{"//attacker.com/steal"}},
		{"null byte in URI", []string{"http://localhost/callback\x00evil"}},
		{"newline injection", []string{"http://localhost/callback\r\nX-Injected: evil"}},
		{"many URIs", generateManyURIs(100)},
	}

	for _, tc := range hostile {
		t.Run(tc.name, func(t *testing.T) {
			uris, _ := json.Marshal(tc.uris)
			body := fmt.Sprintf(`{"client_name":"hostile","redirect_uris":%s}`, string(uris))
			req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(body))
			rec := httptest.NewRecorder()

			// Must not panic
			srv.HandleRegister(rec, req)

			if rec.Code == http.StatusCreated {
				t.Logf("FINDING: DCR accepted hostile redirect_uri %q. "+
					"The /register handler should validate redirect URIs: "+
					"reject non-http(s) schemes, fragments, and dangerous protocols.", tc.name)
			}
		})
	}
}

func generateManyURIs(n int) []string {
	uris := make([]string, n)
	for i := 0; i < n; i++ {
		uris[i] = fmt.Sprintf("http://localhost/%d/callback", i)
	}
	return uris
}

// --- DCR: oversized JSON body ---

func TestDCR_StressOversizedBody(t *testing.T) {
	srv := newStressOAuthServer()

	// 5MB JSON body
	largeName := strings.Repeat("A", 5<<20)
	body := fmt.Sprintf(`{"client_name":"%s","redirect_uris":["http://localhost/cb"]}`, largeName)
	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(body))
	rec := httptest.NewRecorder()

	// Must not panic or OOM
	srv.HandleRegister(rec, req)

	t.Logf("FINDING: DCR handler does not limit request body size. "+
		"An attacker can send a multi-MB body to exhaust memory. "+
		"Use io.LimitReader or http.MaxBytesReader. Status: %d", rec.Code)
}

// --- DCR: concurrent registrations ---

func TestDCR_StressConcurrentRegistrations(t *testing.T) {
	srv := newStressOAuthServer()

	var wg sync.WaitGroup
	errors := make(chan string, 200)

	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			body := fmt.Sprintf(`{"client_name":"concurrent-%d","redirect_uris":["http://localhost/%d/cb"]}`, id, id)
			req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(body))
			rec := httptest.NewRecorder()
			srv.HandleRegister(rec, req)

			if rec.Code != http.StatusCreated {
				errors <- fmt.Sprintf("registration %d got status %d", id, rec.Code)
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("concurrent DCR error: %s", err)
	}
}

// --- DCR: client_name with injection payloads ---

func TestDCR_StressHostileClientNames(t *testing.T) {
	srv := newStressOAuthServer()

	names := []string{
		"<script>alert(1)</script>",
		"'; DROP TABLE clients; --",
		"{{template}}",
		"${7*7}",
		"client\nX-Injected: evil",
		"client\x00name",
		strings.Repeat("A", 1<<20), // 1MB name
	}

	for _, name := range names {
		t.Run("name_"+safeSubstr(name, 30), func(t *testing.T) {
			nameJSON, _ := json.Marshal(name)
			body := fmt.Sprintf(`{"client_name":%s,"redirect_uris":["http://localhost/cb"]}`, string(nameJSON))
			req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(body))
			rec := httptest.NewRecorder()

			srv.HandleRegister(rec, req)

			if rec.Code == http.StatusCreated {
				var client OAuthClient
				json.NewDecoder(rec.Body).Decode(&client)
				if client.ClientName != name {
					t.Errorf("client_name mangled: expected %q, got %q", name, client.ClientName)
				}
			}
		})
	}

	t.Log("FINDING: DCR accepts arbitrary client_name values without length limit or sanitization. " +
		"If client_name is displayed in a web UI, XSS is possible.")
}

// --- DCR: response contains client_secret in cleartext ---

func TestDCR_StressSecretExposure(t *testing.T) {
	srv := newStressOAuthServer()

	body := `{"client_name":"test","redirect_uris":["http://localhost/cb"]}`
	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(body))
	rec := httptest.NewRecorder()
	srv.HandleRegister(rec, req)

	var client OAuthClient
	json.NewDecoder(rec.Body).Decode(&client)

	if client.ClientSecret != "" {
		t.Log("NOTE: DCR response includes client_secret in cleartext. " +
			"This is expected per RFC 7591, but the secret should be " +
			"stored hashed in the ClientStore for defense in depth.")
	}

	// Verify the stored secret is NOT hashed (currently stored as plaintext)
	stored, ok := srv.clients.Get(client.ClientID)
	if !ok {
		t.Fatal("client not found in store")
	}
	if stored.ClientSecret == client.ClientSecret {
		t.Log("FINDING: client_secret stored in plaintext in ClientStore. " +
			"If the store is compromised, all client secrets are exposed. " +
			"Consider hashing with bcrypt/argon2 (at the cost of performance on /token).")
	}
}

// --- DCR: unsupported grant types ---

func TestDCR_StressUnsupportedGrantTypes(t *testing.T) {
	srv := newStressOAuthServer()

	// Client requests grant types the server doesn't support
	body := `{"client_name":"test","redirect_uris":["http://localhost/cb"],"grant_types":["client_credentials","implicit","password"]}`
	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(body))
	rec := httptest.NewRecorder()
	srv.HandleRegister(rec, req)

	if rec.Code == http.StatusCreated {
		var client OAuthClient
		json.NewDecoder(rec.Body).Decode(&client)
		t.Logf("FINDING: DCR accepted unsupported grant_types: %v. "+
			"Server only supports authorization_code and refresh_token "+
			"but doesn't validate requested grant_types against supported ones.", client.GrantTypes)
	}
}

// --- DCR: request body is not JSON ---

func TestDCR_StressNonJSONBodies(t *testing.T) {
	srv := newStressOAuthServer()

	bodies := []struct {
		name        string
		contentType string
		body        string
	}{
		{"XML", "application/xml", "<client><name>test</name></client>"},
		{"form encoded", "application/x-www-form-urlencoded", "client_name=test&redirect_uris=http://localhost/cb"},
		{"multipart", "multipart/form-data", "------\r\nContent-Disposition: form-data; name=\"client_name\"\r\n\r\ntest\r\n------"},
		{"empty", "application/json", ""},
		{"null", "application/json", "null"},
		{"array", "application/json", "[1,2,3]"},
		{"number", "application/json", "42"},
		{"boolean", "application/json", "true"},
	}

	for _, tc := range bodies {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", tc.contentType)
			rec := httptest.NewRecorder()

			// Must not panic
			srv.HandleRegister(rec, req)

			if rec.Code == http.StatusCreated {
				t.Errorf("FINDING: DCR accepted non-JSON body (%s)", tc.name)
			}
		})
	}
}

// --- DCR: response leaks server internals ---

func TestDCR_StressErrorResponseContent(t *testing.T) {
	srv := newStressOAuthServer()

	// Send invalid JSON to trigger error
	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader("{invalid"))
	rec := httptest.NewRecorder()
	srv.HandleRegister(rec, req)

	body := rec.Body.String()

	// Check that error response doesn't leak stack traces, file paths, or Go errors
	leakPatterns := []string{
		"goroutine",
		".go:",
		"runtime.",
		"panic",
		"/home/",
		"internal/",
	}

	for _, pattern := range leakPatterns {
		if strings.Contains(body, pattern) {
			t.Errorf("SECURITY: error response leaks internal info: contains %q", pattern)
		}
	}
}

func safeSubstr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
