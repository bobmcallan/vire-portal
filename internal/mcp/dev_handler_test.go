package mcp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bobmcallan/vire-portal/internal/config"
	common "github.com/bobmcallan/vire-portal/internal/vire/common"
)

// testConfigWithMockAPI creates a config with a mock API server that immediately
// returns 503 to avoid slow connection timeouts when vire-server is unavailable.
func testConfigWithMockAPI(t *testing.T, env, jwtSecret string) (*config.Config, *httptest.Server) {
	t.Helper()
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(mockServer.Close)

	cfg := &config.Config{
		Environment: env,
		API:         config.APIConfig{URL: mockServer.URL},
		Auth:        config.AuthConfig{JWTSecret: jwtSecret},
	}
	return cfg, mockServer
}

func TestDevHandler_EncryptDecrypt(t *testing.T) {
	logger := common.NewSilentLogger()
	jwtSecret := []byte("test-secret-key-32-bytes-long!!")

	// Create base handler with mock API to avoid slow connection timeouts
	cfg, _ := testConfigWithMockAPI(t, "dev", string(jwtSecret))
	handler := NewHandler(cfg, logger)

	// Create dev handler
	devHandler := NewDevHandler(handler, jwtSecret, true, "http://localhost:8881", logger)

	testCases := []string{
		"dev_user",
		"admin",
		"user@example.com",
		"user-with-dashes-and_underscores",
	}

	for _, tc := range testCases {
		t.Run(tc, func(t *testing.T) {
			encrypted, err := devHandler.encryptUID(tc)
			if err != nil {
				t.Fatalf("encryptUID failed: %v", err)
			}

			if encrypted == "" {
				t.Fatal("encrypted UID is empty")
			}

			// Encrypted should be base64url (no special chars that break URLs)
			if strings.ContainsAny(encrypted, "+/=") {
				t.Errorf("encrypted UID contains non-URL-safe chars: %s", encrypted)
			}

			decrypted, err := devHandler.decryptUID(encrypted)
			if err != nil {
				t.Fatalf("decryptUID failed: %v", err)
			}

			if decrypted != tc {
				t.Errorf("expected %q, got %q", tc, decrypted)
			}
		})
	}
}

func TestDevHandler_GenerateEndpoint(t *testing.T) {
	logger := common.NewSilentLogger()
	jwtSecret := []byte("test-secret-key-32-bytes-long!!")

	cfg, _ := testConfigWithMockAPI(t, "dev", string(jwtSecret))
	handler := NewHandler(cfg, logger)
	devHandler := NewDevHandler(handler, jwtSecret, true, "http://localhost:8881", logger)

	endpoint := devHandler.GenerateEndpoint("dev_user")

	if endpoint == "" {
		t.Fatal("endpoint is empty")
	}

	if !strings.HasPrefix(endpoint, "http://localhost:8881/mcp/") {
		t.Errorf("endpoint has wrong prefix: %s", endpoint)
	}

	// Extract encrypted part and verify it decrypts
	encrypted := strings.TrimPrefix(endpoint, "http://localhost:8881/mcp/")
	decrypted, err := devHandler.decryptUID(encrypted)
	if err != nil {
		t.Fatalf("failed to decrypt: %v", err)
	}
	if decrypted != "dev_user" {
		t.Errorf("expected dev_user, got %q", decrypted)
	}
}

func TestDevHandler_GenerateEndpoint_ProdMode(t *testing.T) {
	logger := common.NewSilentLogger()
	jwtSecret := []byte("test-secret-key-32-bytes-long!!")

	cfg, _ := testConfigWithMockAPI(t, "prod", string(jwtSecret))
	handler := NewHandler(cfg, logger)
	devHandler := NewDevHandler(handler, jwtSecret, false, "http://localhost:8881", logger)

	endpoint := devHandler.GenerateEndpoint("dev_user")

	if endpoint != "" {
		t.Errorf("expected empty endpoint in prod mode, got %q", endpoint)
	}
}

func TestDevHandler_ServeHTTP_InvalidEndpoint(t *testing.T) {
	logger := common.NewSilentLogger()
	jwtSecret := []byte("test-secret-key-32-bytes-long!!")

	cfg, _ := testConfigWithMockAPI(t, "dev", string(jwtSecret))
	handler := NewHandler(cfg, logger)
	devHandler := NewDevHandler(handler, jwtSecret, true, "http://localhost:8881", logger)

	// Test with invalid encrypted UID (path includes /mcp/ prefix)
	req := httptest.NewRequest("POST", "/mcp/invalid-encrypted-uid", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	devHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for invalid UID, got %d", rec.Code)
	}
}

func TestDevHandler_ServeHTTP_ProdMode(t *testing.T) {
	logger := common.NewSilentLogger()
	jwtSecret := []byte("test-secret-key-32-bytes-long!!")

	cfg, _ := testConfigWithMockAPI(t, "prod", string(jwtSecret))
	handler := NewHandler(cfg, logger)
	devHandler := NewDevHandler(handler, jwtSecret, false, "http://localhost:8881", logger)

	req := httptest.NewRequest("POST", "/mcp/some-uid", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	devHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 in prod mode, got %d", rec.Code)
	}
}
