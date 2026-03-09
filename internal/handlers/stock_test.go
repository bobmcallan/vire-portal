package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bobmcallan/vire-portal/internal/client"
)

func TestStockHandler_UnauthenticatedRedirect(t *testing.T) {
	handler := NewStockHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/stock/CBA.AX", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected redirect 302, got %d", w.Code)
	}
	loc := w.Header().Get("Location")
	if loc != "/" {
		t.Errorf("expected redirect to /, got %s", loc)
	}
}

func TestStockHandler_EmptyTickerRedirect(t *testing.T) {
	handler := NewStockHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/stock/", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected redirect 302, got %d", w.Code)
	}
	loc := w.Header().Get("Location")
	if loc != "/dashboard" {
		t.Errorf("expected redirect to /dashboard, got %s", loc)
	}
}

func TestStockHandler_RenderWithHolding(t *testing.T) {
	lookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Username: "test-user", NavexaKeySet: true, Role: "user"}, nil
	}

	proxyGetFn := func(path, userID string) ([]byte, error) {
		if path == "/api/portfolios" {
			return json.Marshal(map[string]interface{}{
				"portfolios": []map[string]string{{"name": "Main"}},
				"default":    "Main",
			})
		}
		if strings.Contains(path, "/api/portfolios/Main") {
			return json.Marshal(map[string]interface{}{
				"holdings": []map[string]interface{}{
					{
						"ticker":                 "CBA.AX",
						"name":                   "Commonwealth Bank",
						"holding_value_market":   15000.50,
						"holding_return_net":     2500.00,
						"holding_return_net_pct": 20.0,
					},
				},
			})
		}
		return []byte("null"), nil
	}

	handler := NewStockHandler(nil, true, []byte(testJWTSecret), lookupFn)
	handler.SetProxyGetFn(proxyGetFn)

	req := httptest.NewRequest("GET", "/stock/CBA.AX", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "CBA.AX") {
		t.Error("expected ticker CBA.AX in page body")
	}
	if !strings.Contains(body, "PORTFOLIO HOLDING") {
		t.Error("expected PORTFOLIO HOLDING header in page body")
	}
	if !strings.Contains(body, "Commonwealth Bank") {
		t.Error("expected holding name in hydrated JSON")
	}
}

func TestStockHandler_RenderWithoutHolding(t *testing.T) {
	lookupFn := func(userID string) (*client.UserProfile, error) {
		return &client.UserProfile{Username: "test-user", NavexaKeySet: true, Role: "user"}, nil
	}

	proxyGetFn := func(path, userID string) ([]byte, error) {
		if path == "/api/portfolios" {
			return json.Marshal(map[string]interface{}{
				"portfolios": []map[string]string{{"name": "Main"}},
				"default":    "Main",
			})
		}
		if strings.Contains(path, "/api/portfolios/Main") {
			return json.Marshal(map[string]interface{}{
				"holdings": []map[string]interface{}{
					{"ticker": "BHP.AX", "name": "BHP Group"},
				},
			})
		}
		return []byte("null"), nil
	}

	handler := NewStockHandler(nil, true, []byte(testJWTSecret), lookupFn)
	handler.SetProxyGetFn(proxyGetFn)

	req := httptest.NewRequest("GET", "/stock/XYZ.AX", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "XYZ.AX") {
		t.Error("expected ticker XYZ.AX in page body")
	}
	// holdingJSON should be null when ticker not found
	if !strings.Contains(body, "holding: null") {
		t.Error("expected holding: null when ticker not in portfolio")
	}
}
