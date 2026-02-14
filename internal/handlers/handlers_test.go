package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthHandler_ReturnsOK(t *testing.T) {
	handler := NewHealthHandler(nil)

	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if body["status"] != "ok" {
		t.Errorf("expected status ok, got %s", body["status"])
	}
}

func TestHealthHandler_RejectsNonGET(t *testing.T) {
	handler := NewHealthHandler(nil)

	req := httptest.NewRequest("POST", "/api/health", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}

func TestVersionHandler_ReturnsJSON(t *testing.T) {
	handler := NewVersionHandler(nil)

	req := httptest.NewRequest("GET", "/api/version", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", contentType)
	}

	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if _, ok := body["version"]; !ok {
		t.Error("expected version field in response")
	}
	if _, ok := body["build"]; !ok {
		t.Error("expected build field in response")
	}
	if _, ok := body["git_commit"]; !ok {
		t.Error("expected git_commit field in response")
	}
}

func TestVersionHandler_RejectsNonGET(t *testing.T) {
	handler := NewVersionHandler(nil)

	req := httptest.NewRequest("DELETE", "/api/version", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}

func TestRequireMethod_Matches(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	ok := RequireMethod(w, req, "GET")
	if !ok {
		t.Error("expected RequireMethod to return true for matching method")
	}
}

func TestRequireMethod_Mismatch(t *testing.T) {
	req := httptest.NewRequest("POST", "/test", nil)
	w := httptest.NewRecorder()

	ok := RequireMethod(w, req, "GET")
	if ok {
		t.Error("expected RequireMethod to return false for mismatching method")
	}
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()

	data := map[string]string{"key": "value"}
	WriteJSON(w, http.StatusCreated, data)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}

	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", w.Header().Get("Content-Type"))
	}

	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if body["key"] != "value" {
		t.Errorf("expected key=value, got key=%s", body["key"])
	}
}

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()

	WriteError(w, http.StatusBadRequest, "something went wrong")

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if body["error"] != "something went wrong" {
		t.Errorf("expected error message 'something went wrong', got %s", body["error"])
	}
	if body["status"] != "error" {
		t.Errorf("expected status 'error', got %s", body["status"])
	}
}
