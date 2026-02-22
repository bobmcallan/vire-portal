# Requirements: Fix Google OAuth redirect exposing internal Docker address

**Date:** 2026-02-23
**Requested:** Google "Sign in" on http://localhost:8880/ redirects browser to `http://server:8080/...` which is an internal Docker address not accessible from the browser. Users should never see internal addresses.

## Problem

Current flow in `HandleGoogleLogin` (auth.go:218-221):
```go
redirectURL := h.apiURL + "/api/auth/login/google?callback=" + h.callbackURL
http.Redirect(w, r, redirectURL, http.StatusFound)
```

When `VIRE_API_URL=http://server:8080` (Docker internal), the browser gets redirected to `http://server:8080/api/auth/login/google?callback=http://localhost:8880/auth/callback` which the browser cannot reach.

## Scope
- In scope: Fix `HandleGoogleLogin` and `HandleGitHubLogin` to proxy the OAuth initiation server-side instead of redirecting the browser to an internal URL
- In scope: Unit tests for the new proxy behavior
- In scope: UI test for Google login flow (browser must never see internal addresses)
- Out of scope: vire-server changes, Docker config changes

## Approach

### Change 1: Proxy OAuth initiation server-side

**File: `internal/handlers/auth.go`** — `HandleGoogleLogin` and `HandleGitHubLogin`

Instead of redirecting the browser to `{VIRE_API_URL}/api/auth/login/google`, make a **server-side HTTP request** to vire-server and forward the resulting redirect Location header to the browser.

```go
func (h *AuthHandler) HandleGoogleLogin(w http.ResponseWriter, r *http.Request) {
    serverURL := h.apiURL + "/api/auth/login/google?callback=" + url.QueryEscape(h.callbackURL)

    client := &http.Client{
        CheckRedirect: func(req *http.Request, via []*http.Request) error {
            return http.ErrUseLastResponse // Don't follow redirects
        },
        Timeout: 10 * time.Second,
    }
    resp, err := client.Get(serverURL)
    if err != nil {
        h.logger.Error("Google login: failed to reach vire-server", "error", err)
        http.Redirect(w, r, "/error?reason=auth_unavailable", http.StatusFound)
        return
    }
    defer resp.Body.Close()

    location := resp.Header.Get("Location")
    if location == "" {
        h.logger.Error("Google login: vire-server did not return redirect", "status", resp.StatusCode)
        http.Redirect(w, r, "/error?reason=auth_failed", http.StatusFound)
        return
    }

    http.Redirect(w, r, location, http.StatusFound)
}
```

Same pattern for `HandleGitHubLogin`.

### Change 2: Unit tests for proxy behavior

**File: `internal/handlers/auth_test.go`** — Add tests:
- `TestHandleGoogleLogin_ProxiesRedirect` — mock vire-server returns 302 to Google, verify browser gets Google URL
- `TestHandleGoogleLogin_ServerUnreachable` — vire-server down, verify redirect to error page
- `TestHandleGoogleLogin_ServerNoRedirect` — vire-server returns 200 (no redirect), verify error handling
- Same for GitHub

### Change 3: Update existing unit tests

**File: `internal/handlers/auth_test.go`** — `TestHandleGoogleLogin_RedirectsToVireServer` needs updating since behavior changed from direct redirect to proxy
**File: `internal/handlers/auth_stress_test.go`** — `TestGoogleLogin_StressOpenRedirectProtection` needs updating

### Change 4: UI test for Google login redirect

**File: `tests/ui/auth_test.go`** — Update `TestAuthGoogleLoginRedirect`:
- Verify clicking "Sign in with Google" does NOT redirect to an internal address
- Verify the redirect goes to Google (accounts.google.com) or shows error page, never `server:*` URLs

## Files Expected to Change
- `internal/handlers/auth.go`
- `internal/handlers/auth_test.go`
- `internal/handlers/auth_stress_test.go`
- `tests/ui/auth_test.go`
