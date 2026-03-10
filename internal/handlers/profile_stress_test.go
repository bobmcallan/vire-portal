package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/bobmcallan/vire-portal/internal/client"
)

// =============================================================================
// Profile / Timezone Security Stress Tests
// =============================================================================

func TestProfileStress_TimezoneXSSInTemplate(t *testing.T) {
	// UserTimezone is rendered as value="{{.UserTimezone}}" in an input field.
	// Go's html/template auto-escapes HTML attribute values.
	xssPayloads := []struct {
		input string
		check string // dangerous substring that must NOT appear unescaped
	}{
		{`"><script>alert(1)</script>`, `"><script>alert(1)`},
		{`" onmouseover="alert(1)`, `" onmouseover="`},
		{`<img src=x onerror=alert(1)>`, `<img src=x onerror`},
	}

	for _, tc := range xssPayloads {
		userLookup := func(sub string) (*client.UserProfile, error) {
			return &client.UserProfile{
				Timezone:     tc.input,
				NavexaKeySet: true,
			}, nil
		}
		handler := NewProfileHandler(nil, true, []byte(testJWTSecret), userLookup, nil)

		req := httptest.NewRequest("GET", "/profile", nil)
		addAuthCookie(req, "test-user")
		w := httptest.NewRecorder()

		handler.HandleProfile(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("payload %q: expected 200, got %d", tc.input, w.Code)
			continue
		}

		body := w.Body.String()
		if strings.Contains(body, tc.check) {
			t.Errorf("SECURITY XSS: timezone check %q appears unescaped in profile page (input: %q)", tc.check, tc.input)
		}
	}
}

func TestProfileStress_TimezoneFieldPresent(t *testing.T) {
	userLookup := func(sub string) (*client.UserProfile, error) {
		return &client.UserProfile{
			Timezone: "Australia/Sydney",
		}, nil
	}
	handler := NewProfileHandler(nil, true, []byte(testJWTSecret), userLookup, nil)

	req := httptest.NewRequest("GET", "/profile", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.HandleProfile(w, req)

	body := w.Body.String()
	if !strings.Contains(body, `name="timezone"`) {
		t.Error("profile page missing timezone input field")
	}
	if !strings.Contains(body, `id="tz-list"`) {
		t.Error("profile page missing timezone datalist")
	}
	if !strings.Contains(body, "Australia/Sydney") {
		t.Error("profile page missing timezone value")
	}
}

func TestProfileStress_SaveTimezoneOnly(t *testing.T) {
	// Saving timezone alone (no navexa_key) should succeed
	var savedFields map[string]string
	saveFn := func(sub string, fields map[string]string) error {
		savedFields = fields
		return nil
	}
	userLookup := func(sub string) (*client.UserProfile, error) {
		return &client.UserProfile{}, nil
	}
	handler := NewProfileHandler(nil, true, []byte(testJWTSecret), userLookup, saveFn)

	form := url.Values{}
	form.Set("timezone", "America/New_York")
	form.Set("_csrf", "test-csrf")

	req := httptest.NewRequest("POST", "/profile", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addAuthCookie(req, "test-user")
	req.AddCookie(&http.Cookie{Name: "_csrf", Value: "test-csrf"})
	w := httptest.NewRecorder()

	handler.HandleSaveProfile(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected 302 redirect, got %d", w.Code)
	}
	if savedFields == nil {
		t.Fatal("saveFn was not called")
	}
	if savedFields["timezone"] != "America/New_York" {
		t.Errorf("expected timezone America/New_York, got %q", savedFields["timezone"])
	}
	if _, hasKey := savedFields["navexa_key"]; hasKey {
		t.Error("navexa_key should not be in saved fields when empty")
	}
}

func TestProfileStress_SaveEmptyFieldsRedirects(t *testing.T) {
	// Submitting with no timezone and no navexa_key should redirect without calling save
	saveCalled := false
	saveFn := func(sub string, fields map[string]string) error {
		saveCalled = true
		return nil
	}
	handler := NewProfileHandler(nil, true, []byte(testJWTSecret), nil, saveFn)

	form := url.Values{}
	form.Set("_csrf", "test-csrf")

	req := httptest.NewRequest("POST", "/profile", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addAuthCookie(req, "test-user")
	req.AddCookie(&http.Cookie{Name: "_csrf", Value: "test-csrf"})
	w := httptest.NewRecorder()

	handler.HandleSaveProfile(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected 302 redirect, got %d", w.Code)
	}
	if saveCalled {
		t.Error("saveFn should not be called when no fields are provided")
	}
}

func TestProfileStress_TimezoneArbitraryInput(t *testing.T) {
	// Timezone accepts arbitrary strings — verify they are passed through
	// (validation is server-side responsibility)
	var savedTz string
	saveFn := func(sub string, fields map[string]string) error {
		savedTz = fields["timezone"]
		return nil
	}
	handler := NewProfileHandler(nil, true, []byte(testJWTSecret), nil, saveFn)

	arbitraryInputs := []string{
		"Not/A/Real/Timezone",
		"   spaces   ",
		"<script>alert(1)</script>",
		strings.Repeat("A", 500),
	}

	for _, input := range arbitraryInputs {
		form := url.Values{}
		form.Set("timezone", input)
		form.Set("_csrf", "test-csrf")

		req := httptest.NewRequest("POST", "/profile", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		addAuthCookie(req, "test-user")
		req.AddCookie(&http.Cookie{Name: "_csrf", Value: "test-csrf"})
		w := httptest.NewRecorder()

		handler.HandleSaveProfile(w, req)

		if w.Code != http.StatusFound {
			t.Errorf("input %q: expected 302, got %d", truncStr(input, 30), w.Code)
		}
		// TrimSpace is applied
		expected := strings.TrimSpace(input)
		if expected != "" && savedTz != expected {
			t.Errorf("input %q: expected saved %q, got %q", truncStr(input, 30), truncStr(expected, 30), truncStr(savedTz, 30))
		}
	}
}

func TestProfileStress_UnauthenticatedSaveReturns401(t *testing.T) {
	handler := NewProfileHandler(nil, true, []byte(testJWTSecret), nil, nil)

	form := url.Values{}
	form.Set("timezone", "Australia/Sydney")

	req := httptest.NewRequest("POST", "/profile", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	handler.HandleSaveProfile(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for unauthenticated save, got %d", w.Code)
	}
}

func TestProfileStress_TimezoneAndKeyBothSaved(t *testing.T) {
	var savedFields map[string]string
	saveFn := func(sub string, fields map[string]string) error {
		savedFields = fields
		return nil
	}
	handler := NewProfileHandler(nil, true, []byte(testJWTSecret), nil, saveFn)

	form := url.Values{}
	form.Set("navexa_key", "my-key-123")
	form.Set("timezone", "Europe/London")
	form.Set("_csrf", "test-csrf")

	req := httptest.NewRequest("POST", "/profile", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	addAuthCookie(req, "test-user")
	req.AddCookie(&http.Cookie{Name: "_csrf", Value: "test-csrf"})
	w := httptest.NewRecorder()

	handler.HandleSaveProfile(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected 302, got %d", w.Code)
	}
	if savedFields["navexa_key"] != "my-key-123" {
		t.Error("navexa_key not saved")
	}
	if savedFields["timezone"] != "Europe/London" {
		t.Error("timezone not saved")
	}
}
