package handlers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// =============================================================================
// Docs Page Stress Tests — Security & Edge Cases
// =============================================================================

// --- Docs Page: Accessible Without Auth (Public Page) ---

func TestDocsPage_StressAccessibleWithoutAuth(t *testing.T) {
	// The docs page uses ServePage with pageName="docs", which does NOT trigger
	// auto-logout (only pageName="home" does). Unauthenticated users should see
	// the page rendered successfully (200 OK, not a redirect).
	handler := NewPageHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/docs", nil)
	w := httptest.NewRecorder()

	handler.ServePage("docs.html", "docs")(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("docs page should be accessible without auth, got status %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "WHAT IS VIRE") {
		t.Error("expected docs page to contain 'WHAT IS VIRE' section")
	}
}

// --- Docs Page: Does NOT Trigger Auto-Logout ---

func TestDocsPage_StressDoesNotClearSession(t *testing.T) {
	// CRITICAL: ServePage with pageName="home" clears the vire_session cookie.
	// The docs page uses pageName="docs" — it must NOT clear the cookie.
	// If it did, visiting /docs while logged in would destroy the session.
	handler := NewPageHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/docs", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServePage("docs.html", "docs")(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for authenticated docs request, got %d", w.Code)
	}

	// Check that no Set-Cookie header was sent to clear the session
	cookies := w.Result().Cookies()
	for _, c := range cookies {
		if c.Name == "vire_session" && c.MaxAge < 0 {
			t.Error("SECURITY: docs page cleared the vire_session cookie — pageName must not be 'home'")
		}
	}
}

// --- Docs Page: LoggedIn Shows Nav ---

func TestDocsPage_StressNavRenderedWhenLoggedIn(t *testing.T) {
	// When logged in, the nav should render ({{if .LoggedIn}}{{template "nav.html" .}}{{end}}).
	handler := NewPageHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/docs", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServePage("docs.html", "docs")(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "nav-brand") {
		t.Error("expected nav to render on docs page when logged in")
	}
}

func TestDocsPage_StressNavHiddenWhenLoggedOut(t *testing.T) {
	// When not logged in, the nav should NOT render.
	handler := NewPageHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/docs", nil)
	w := httptest.NewRecorder()

	handler.ServePage("docs.html", "docs")(w, req)

	body := w.Body.String()
	if strings.Contains(body, "nav-brand") {
		t.Error("nav should NOT render on docs page when logged out")
	}
}

// --- Docs Page: Nav Active State ---

func TestDocsPage_StressNavActiveState(t *testing.T) {
	handler := NewPageHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/docs", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServePage("docs.html", "docs")(w, req)

	body := w.Body.String()

	// Docs is accessible but no longer in the desktop nav — no active link expected
	// Dashboard link must NOT be active
	if strings.Contains(body, `href="/dashboard" class="active"`) {
		t.Error("dashboard nav link should not be active on docs page")
	}
}

// --- Docs Page: No XSS Vectors ---

func TestDocsPage_StressNoXHTMLDirectives(t *testing.T) {
	// Docs page is static content. Verify no x-html directives that could
	// render raw HTML. Should only use standard HTML elements.
	handler := NewPageHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/docs", nil)
	w := httptest.NewRecorder()

	handler.ServePage("docs.html", "docs")(w, req)

	body := w.Body.String()
	if strings.Contains(body, "x-html") {
		t.Error("SECURITY: docs page uses x-html which renders raw HTML")
	}
}

func TestDocsPage_StressNoInlineEventHandlers(t *testing.T) {
	handler := NewPageHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/docs", nil)
	w := httptest.NewRecorder()

	handler.ServePage("docs.html", "docs")(w, req)

	body := w.Body.String()

	dangerousAttrs := []string{
		` onclick=`, ` onerror=`, ` onload=`, ` onmouseover=`,
		` onfocus=`, ` onsubmit=`, ` onchange=`,
	}
	for _, attr := range dangerousAttrs {
		if strings.Contains(strings.ToLower(body), attr) {
			t.Errorf("SECURITY: found dangerous inline handler %q in docs template", attr)
		}
	}
}

func TestDocsPage_StressNoJavaScriptProtocol(t *testing.T) {
	handler := NewPageHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/docs", nil)
	w := httptest.NewRecorder()

	handler.ServePage("docs.html", "docs")(w, req)

	body := strings.ToLower(w.Body.String())
	if strings.Contains(body, "javascript:") {
		t.Error("SECURITY: docs page contains javascript: protocol URL")
	}
}

// --- Docs Page: External Links Safety ---

func TestDocsPage_StressExternalLinksPresent(t *testing.T) {
	// Verify Navexa links are present and use https
	handler := NewPageHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/docs", nil)
	w := httptest.NewRecorder()

	handler.ServePage("docs.html", "docs")(w, req)

	body := w.Body.String()
	if !strings.Contains(body, `href="https://www.navexa.com"`) {
		t.Error("expected Navexa link with https in docs page")
	}
	// Should not use http:// for external links
	if strings.Contains(body, `href="http://www.navexa.com"`) {
		t.Error("SECURITY: Navexa link uses http instead of https")
	}
}

// --- Docs Page: Internal Links Safety ---

func TestDocsPage_StressInternalLinksCorrect(t *testing.T) {
	handler := NewPageHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/docs", nil)
	w := httptest.NewRecorder()

	handler.ServePage("docs.html", "docs")(w, req)

	body := w.Body.String()

	// Profile link should go to /profile
	if !strings.Contains(body, `href="/profile"`) {
		t.Error("expected /profile link in docs page")
	}
	// Dashboard link should go to /dashboard
	if !strings.Contains(body, `href="/dashboard"`) {
		t.Error("expected /dashboard link in docs page")
	}
}

// --- Docs Page: Content Integrity ---

func TestDocsPage_StressAllSectionsPresent(t *testing.T) {
	handler := NewPageHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/docs", nil)
	w := httptest.NewRecorder()

	handler.ServePage("docs.html", "docs")(w, req)

	body := w.Body.String()

	expectedSections := []string{
		"WHAT IS VIRE",
		"NAVEXA",
		"GETTING YOUR API KEY",
		"CONFIGURING VIRE",
	}
	for _, section := range expectedSections {
		if !strings.Contains(body, section) {
			t.Errorf("expected section %q in docs page", section)
		}
	}
}

// --- Docs Page: Template Does Not Conflict With Other Handlers ---

func TestDocsPage_StressTemplateParsesWithAllPages(t *testing.T) {
	// Adding docs.html to pages/ means ParseGlob includes it in all handlers.
	// Verify the dashboard handler still works after docs.html is added.
	dashHandler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	dashHandler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("dashboard handler broken after docs.html added to pages/, got status %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "portfolioDashboard") {
		t.Error("dashboard template content not rendered correctly after docs.html added")
	}
}

func TestDocsPage_StressStrategyHandlerStillWorks(t *testing.T) {
	// Verify strategy handler also works after docs.html added
	stratHandler := NewStrategyHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/strategy", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	stratHandler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("strategy handler broken after docs.html added, got status %d", w.Code)
	}
}

// --- Docs Page: Concurrent Access ---

func TestDocsPage_StressConcurrentAccess(t *testing.T) {
	handler := NewPageHandler(nil, true, []byte(testJWTSecret), nil)
	serveDocs := handler.ServePage("docs.html", "docs")

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			req := httptest.NewRequest("GET", "/docs", nil)
			if n%2 == 0 {
				addAuthCookie(req, fmt.Sprintf("user-%d", n))
			}
			w := httptest.NewRecorder()

			serveDocs(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("concurrent docs request %d got status %d", n, w.Code)
			}
		}(i)
	}
	wg.Wait()
}

func TestDocsPage_StressConcurrentMixedPages(t *testing.T) {
	// Verify serving docs concurrently with dashboard doesn't cause data leaks
	pageHandler := NewPageHandler(nil, true, []byte(testJWTSecret), nil)
	dashHandler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)

	serveDocs := pageHandler.ServePage("docs.html", "docs")

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			if n%2 == 0 {
				// Docs page (public)
				req := httptest.NewRequest("GET", "/docs", nil)
				w := httptest.NewRecorder()
				serveDocs(w, req)
				if w.Code != http.StatusOK {
					t.Errorf("concurrent docs request %d got %d", n, w.Code)
				}
			} else {
				// Dashboard (auth required)
				req := httptest.NewRequest("GET", "/dashboard", nil)
				addAuthCookie(req, fmt.Sprintf("user-%d", n))
				w := httptest.NewRecorder()
				dashHandler.ServeHTTP(w, req)
				if w.Code != http.StatusOK {
					t.Errorf("concurrent dashboard request %d got %d", n, w.Code)
				}
			}
		}(i)
	}
	wg.Wait()
}

// =============================================================================
// Nav Brand Link Stress Tests — /dashboard redirect safety
// =============================================================================

func TestNavBrandLink_StressDashboardRedirectsUnauthenticated(t *testing.T) {
	// The nav brand now links to /dashboard instead of /.
	// When an unauthenticated user visits /dashboard directly, the DashboardHandler
	// must redirect to / (landing page). This confirms the safety net is intact.
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("SECURITY: unauthenticated /dashboard access returned %d, expected 302", w.Code)
	}
	location := w.Header().Get("Location")
	if location != "/" {
		t.Errorf("expected redirect to /, got %q", location)
	}
}

func TestNavBrandLink_StressLandingStillClearsSession(t *testing.T) {
	// The "/" route still uses pageName="home" which triggers auto-logout.
	// Even though the nav brand no longer links to /, direct navigation to /
	// must still clear the session (e.g., bookmarks, typed URL).
	handler := NewPageHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServePage("landing.html", "home")(w, req)

	cookies := w.Result().Cookies()
	sessionCleared := false
	for _, c := range cookies {
		if c.Name == "vire_session" && c.MaxAge < 0 {
			sessionCleared = true
			break
		}
	}
	if !sessionCleared {
		t.Error("SECURITY: landing page (/) did not clear vire_session cookie")
	}
}

func TestNavBrandLink_StressBrandHrefInTemplate(t *testing.T) {
	// Verify the nav brand link actually points to /dashboard
	handler := NewPageHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/docs", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServePage("docs.html", "docs")(w, req)

	body := w.Body.String()
	if !strings.Contains(body, `href="/dashboard" class="nav-brand"`) {
		t.Error("expected nav brand to link to /dashboard")
	}
	if strings.Contains(body, `href="/" class="nav-brand"`) {
		t.Error("REGRESSION: nav brand still links to / instead of /dashboard")
	}
}

// =============================================================================
// Nav Docs Item Stress Tests — all three locations
// =============================================================================

func TestNavDocsItem_StressDesktopNavPresent(t *testing.T) {
	handler := NewPageHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/docs", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServePage("docs.html", "docs")(w, req)

	body := w.Body.String()
	// Desktop nav should have Docs link in nav-links
	if !strings.Contains(body, `href="/docs"`) {
		t.Error("expected /docs link in nav")
	}
}

func TestNavDocsItem_StressMobileNavPresent(t *testing.T) {
	handler := NewPageHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/docs", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServePage("docs.html", "docs")(w, req)

	body := w.Body.String()
	// Count docs links — should appear in desktop nav, mobile menu, and hamburger dropdown
	count := strings.Count(body, `href="/docs"`)
	if count < 3 {
		t.Errorf("expected at least 3 /docs links (desktop, mobile, dropdown), found %d", count)
	}
}

func TestNavDocsItem_StressDropdownOrderCorrect(t *testing.T) {
	// In the hamburger dropdown, Docs should appear after Profile and before Logout.
	handler := NewPageHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/docs", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServePage("docs.html", "docs")(w, req)

	body := w.Body.String()

	// Find the nav-dropdown section and verify order: Profile, Docs, Logout
	dropdownIdx := strings.Index(body, "nav-dropdown")
	if dropdownIdx < 0 {
		t.Fatal("nav-dropdown not found in rendered page")
	}
	dropdownSection := body[dropdownIdx:]

	profileIdx := strings.Index(dropdownSection, `href="/profile"`)
	docsIdx := strings.Index(dropdownSection, `href="/docs"`)
	logoutIdx := strings.Index(dropdownSection, "nav-dropdown-logout")

	if profileIdx < 0 || docsIdx < 0 || logoutIdx < 0 {
		t.Fatal("missing items in dropdown: Profile, Docs, or Logout")
	}

	if !(profileIdx < docsIdx && docsIdx < logoutIdx) {
		t.Error("dropdown order should be: Profile, Docs, Logout")
	}
}

// =============================================================================
// Refresh Button Positioning — Template Integrity
// =============================================================================

func TestRefreshButton_StressMarginLeftAutoPresent(t *testing.T) {
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "margin-left:auto") {
		t.Error("expected refresh button to have margin-left:auto for right-alignment")
	}
}

func TestRefreshButton_StressDisabledStatePresent(t *testing.T) {
	handler := NewDashboardHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/dashboard", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	// Verify :disabled binding exists to prevent double-click
	if !strings.Contains(body, `:disabled="refreshing"`) {
		t.Error("refresh button must have :disabled binding to prevent race condition on double-click")
	}
}

// =============================================================================
// CSS Reference Verification
// =============================================================================

func TestDocsPage_StressCSSReference(t *testing.T) {
	handler := NewPageHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/docs", nil)
	w := httptest.NewRecorder()

	handler.ServePage("docs.html", "docs")(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "portal.css") {
		t.Error("expected portal.css to be referenced in docs template")
	}
}

// =============================================================================
// Template Data Isolation — Docs vs Other Pages
// =============================================================================

func TestDocsPage_StressPageNameIsolation(t *testing.T) {
	// Verify that the docs page has .Page="docs" and not some other value.
	// Docs is accessible but no longer in the desktop nav, so no active link expected.
	handler := NewPageHandler(nil, true, []byte(testJWTSecret), nil)

	req := httptest.NewRequest("GET", "/docs", nil)
	addAuthCookie(req, "test-user")
	w := httptest.NewRecorder()

	handler.ServePage("docs.html", "docs")(w, req)

	body := w.Body.String()

	// No nav link should be active since docs is not in the desktop nav
	activeLinks := strings.Count(body, `class="active"`)
	if activeLinks != 0 {
		t.Errorf("expected 0 active nav links on docs page, found %d", activeLinks)
	}
}
