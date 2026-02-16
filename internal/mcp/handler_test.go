package mcp

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// buildTestJWT creates an unsigned JWT for testing (alg:none, no signature).
func buildTestJWT(sub string) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	claims, _ := json.Marshal(map[string]interface{}{
		"sub": sub,
		"iss": "vire-dev",
		"exp": time.Now().Add(24 * time.Hour).Unix(),
	})
	payload := base64.RawURLEncoding.EncodeToString(claims)
	return header + "." + payload + "."
}

// --- withUserContext Tests ---

func TestWithUserContext_ValidCookie(t *testing.T) {
	jwt := buildTestJWT("user42")

	req := httptest.NewRequest("GET", "/mcp", nil)
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: jwt})

	h := &Handler{}
	result := h.withUserContext(req)

	uc, ok := GetUserContext(result.Context())
	if !ok {
		t.Fatal("expected GetUserContext to return ok=true")
	}
	if uc.UserID != "user42" {
		t.Errorf("expected UserID user42, got %s", uc.UserID)
	}
}

func TestWithUserContext_NoCookie(t *testing.T) {
	req := httptest.NewRequest("GET", "/mcp", nil)

	h := &Handler{}
	result := h.withUserContext(req)

	_, ok := GetUserContext(result.Context())
	if ok {
		t.Error("expected GetUserContext to return ok=false when no cookie is set")
	}
}

func TestWithUserContext_InvalidJWT(t *testing.T) {
	req := httptest.NewRequest("GET", "/mcp", nil)
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: "not-a-jwt"})

	h := &Handler{}
	result := h.withUserContext(req)

	_, ok := GetUserContext(result.Context())
	if ok {
		t.Error("expected GetUserContext to return ok=false for invalid JWT")
	}
}

// --- Bearer token tests ---

func TestWithUserContext_BearerToken(t *testing.T) {
	jwt := buildTestJWT("bearer-user")

	req := httptest.NewRequest("POST", "/mcp", nil)
	req.Header.Set("Authorization", "Bearer "+jwt)

	h := &Handler{}
	result := h.withUserContext(req)

	uc, ok := GetUserContext(result.Context())
	if !ok {
		t.Fatal("expected GetUserContext to return ok=true for Bearer token")
	}
	if uc.UserID != "bearer-user" {
		t.Errorf("expected UserID bearer-user, got %s", uc.UserID)
	}
}

func TestWithUserContext_BearerTokenTakesPriority(t *testing.T) {
	bearerJWT := buildTestJWT("bearer-user")
	cookieJWT := buildTestJWT("cookie-user")

	req := httptest.NewRequest("POST", "/mcp", nil)
	req.Header.Set("Authorization", "Bearer "+bearerJWT)
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: cookieJWT})

	h := &Handler{}
	result := h.withUserContext(req)

	uc, ok := GetUserContext(result.Context())
	if !ok {
		t.Fatal("expected GetUserContext to return ok=true")
	}
	if uc.UserID != "bearer-user" {
		t.Errorf("expected Bearer to take priority, got UserID %s", uc.UserID)
	}
}

func TestWithUserContext_InvalidBearerFallsToCookie(t *testing.T) {
	cookieJWT := buildTestJWT("cookie-user")

	req := httptest.NewRequest("POST", "/mcp", nil)
	req.Header.Set("Authorization", "Bearer invalid-jwt")
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: cookieJWT})

	h := &Handler{}
	result := h.withUserContext(req)

	uc, ok := GetUserContext(result.Context())
	if !ok {
		t.Fatal("expected GetUserContext to return ok=true from cookie fallback")
	}
	if uc.UserID != "cookie-user" {
		t.Errorf("expected cookie fallback, got UserID %s", uc.UserID)
	}
}

func TestWithUserContext_EmptyBearerIgnored(t *testing.T) {
	req := httptest.NewRequest("POST", "/mcp", nil)
	req.Header.Set("Authorization", "Bearer ")

	h := &Handler{}
	result := h.withUserContext(req)

	_, ok := GetUserContext(result.Context())
	if ok {
		t.Error("expected no user context for empty Bearer token")
	}
}

func TestWithUserContext_NonBearerAuthIgnored(t *testing.T) {
	cookieJWT := buildTestJWT("cookie-user")

	req := httptest.NewRequest("POST", "/mcp", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	req.AddCookie(&http.Cookie{Name: "vire_session", Value: cookieJWT})

	h := &Handler{}
	result := h.withUserContext(req)

	uc, ok := GetUserContext(result.Context())
	if !ok {
		t.Fatal("expected GetUserContext to return ok=true from cookie")
	}
	if uc.UserID != "cookie-user" {
		t.Errorf("expected cookie fallback for Basic auth, got UserID %s", uc.UserID)
	}
}

// --- extractJWTSub Tests ---

func TestExtractJWTSub_ValidJWT(t *testing.T) {
	jwt := buildTestJWT("user99")

	sub := extractJWTSub(jwt)
	if sub != "user99" {
		t.Errorf("expected sub user99, got %s", sub)
	}
}

func TestExtractJWTSub_InvalidBase64Payload(t *testing.T) {
	// Two parts but second part is invalid base64
	sub := extractJWTSub("header.!!!invalid-base64!!!.sig")
	if sub != "" {
		t.Errorf("expected empty string for invalid base64, got %s", sub)
	}
}

func TestExtractJWTSub_InvalidJSON(t *testing.T) {
	payload := base64.RawURLEncoding.EncodeToString([]byte("not json at all"))
	sub := extractJWTSub("header." + payload + ".sig")
	if sub != "" {
		t.Errorf("expected empty string for invalid JSON, got %s", sub)
	}
}

func TestExtractJWTSub_MissingSub(t *testing.T) {
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"iss":"vire-server","exp":999999999}`))
	sub := extractJWTSub("header." + payload + ".sig")
	if sub != "" {
		t.Errorf("expected empty string for missing sub, got %s", sub)
	}
}

func TestExtractJWTSub_EmptyString(t *testing.T) {
	sub := extractJWTSub("")
	if sub != "" {
		t.Errorf("expected empty string for empty input, got %s", sub)
	}
}

func TestExtractJWTSub_NoDotsInToken(t *testing.T) {
	sub := extractJWTSub("nodots")
	if sub != "" {
		t.Errorf("expected empty string for token with no dots, got %s", sub)
	}
}

func TestExtractJWTSub_SingleDot(t *testing.T) {
	sub := extractJWTSub("one.dot")
	// Has two parts (index 0 and 1), so second part is decoded
	// "dot" is valid base64 but decodes to garbage JSON
	if sub != "" {
		t.Errorf("expected empty string for single-dot token, got %s", sub)
	}
}
