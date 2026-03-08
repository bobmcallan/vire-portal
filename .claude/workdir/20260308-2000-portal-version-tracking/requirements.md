# Requirements: Portal Version Tracking

**Feature:** fb_3f4a5832 — Portal version tracking via headers and startup POST
**Status:** planned

## Scope

Two mechanisms for reporting portal version to vire-server:

1. **Passive header injection** — `X-Portal-Version` and `X-Portal-Build` headers on every VireClient HTTP request via a custom `http.RoundTripper`.
2. **Explicit startup POST** — `POST /api/portal/version` with `{"version":"x.y.z","build":"YYYYMMDD-HHMMSS"}` during app initialization, using service auth.

**NOT in scope:** UI changes, MCP proxy header changes (already has X-Vire-Portal-* headers), config changes.

## File Changes

### 1. `internal/client/vire_client.go` — Core changes

**Add `versionTransport` type** (unexported, implements `http.RoundTripper`):
```go
type versionTransport struct {
    base    http.RoundTripper
    version string
    build   string
}

func (t *versionTransport) RoundTrip(req *http.Request) (*http.Response, error) {
    if t.version != "" {
        req.Header.Set("X-Portal-Version", t.version)
    }
    if t.build != "" {
        req.Header.Set("X-Portal-Build", t.build)
    }
    return t.base.RoundTrip(req)
}
```

**Update `NewVireClient` signature** to accept version and build:
```go
func NewVireClient(baseURL, version, build string) *VireClient
```

Use `versionTransport` as the HTTP client's Transport.

**Add `ReportPortalVersion` method:**
```go
func (c *VireClient) ReportPortalVersion(serviceID, version, build string) error
```
- POST /api/portal/version with JSON body `{"version":"...","build":"..."}`
- Set `X-Vire-Service-ID` header for auth
- Set `Content-Type: application/json`
- Return error on non-200 status

### 2. `internal/app/app.go` — Wiring

**Update VireClient creation** (line 112):
```go
// Before:
vireClient := client.NewVireClient(a.Config.API.URL)
// After:
vireClient := client.NewVireClient(a.Config.API.URL, config.GetVersion(), config.GetBuild())
```

**Add version report to startup goroutine** (after SyncAdmins call, line 97):
```go
if err := seed.ReportPortalVersion(cfg.API.URL, serviceUserID, config.GetVersion(), config.GetBuild(), logger); err != nil {
    logger.Warn().Err(err).Msg("portal version report failed")
}
```

**Add new else-if branch** for service key without admin emails (after the existing `} else if len(cfg.AdminEmails()) > 0 {` block):
```go
} else if cfg.Service.Key != "" {
    go func() {
        portalID := cfg.Service.PortalID
        if portalID == "" {
            portalID, _ = os.Hostname()
        }
        serviceUserID, err := seed.RegisterService(cfg.API.URL, portalID, cfg.Service.Key, logger)
        if err != nil {
            logger.Warn().Err(err).Msg("service registration failed, skipping version report")
            return
        }
        if err := seed.ReportPortalVersion(cfg.API.URL, serviceUserID, config.GetVersion(), config.GetBuild(), logger); err != nil {
            logger.Warn().Err(err).Msg("portal version report failed")
        }
    }()
}
```

### 3. `internal/seed/version.go` — NEW FILE

Follow `service.go` pattern. Retry wrapper for version reporting:
```go
func ReportPortalVersion(apiURL, serviceUserID, version, build string, logger *common.Logger) error
```
- Create VireClient, call ReportPortalVersion with retries (use `seedRetryAttempts` and `seedRetryDelay` from seed package)
- Log success/failure at each attempt

### 4. `internal/seed/seed.go`, `internal/seed/service.go`, `internal/seed/admin.go` — Update call sites

Change all `client.NewVireClient(apiURL)` to `client.NewVireClient(apiURL, "", "")`.

### 5. Test files — Update signatures + new tests

**Update all existing `NewVireClient(url)` calls** in test files to `NewVireClient(url, "", "")`:
- `internal/client/vire_client_test.go`
- `internal/client/vire_client_stress_test.go`
- `internal/client/list_users_stress_test.go`
- `internal/client/service_auth_stress_test.go`
- `internal/client/proxy_get_stress_test.go`
- `internal/client/log_store_test.go`
- `internal/client/log_store_stress_test.go`

**New tests in `internal/client/vire_client_test.go`:**
- `TestVersionHeaders_SentOnAllRequests` — verify X-Portal-Version/Build on ProxyGet
- `TestVersionHeaders_EmptyWhenNotSet` — verify no headers when empty strings
- `TestVersionHeaders_OnGetUser` — verify headers on GetUser (convenience method path)
- `TestReportPortalVersion_Success` — happy path POST
- `TestReportPortalVersion_ServerError` — 403 returns error
- `TestReportPortalVersion_Unreachable` — connection refused returns error

**New file `internal/seed/version_test.go`:**
- `TestReportPortalVersion_Success` — happy path with retries
- `TestReportPortalVersion_RetryOnFailure` — fails twice then succeeds
- `TestReportPortalVersion_AllRetriesFail` — all retries exhausted

## Edge Cases
- Empty version/build strings: headers not set (graceful no-op)
- Service key not configured: startup POST skipped entirely
- vire-server unreachable: retries with existing seed retry constants, logs warning
- Service registration fails: version POST skipped (requires service auth)

## Implementation Order
1. Add versionTransport and update NewVireClient signature
2. Add ReportPortalVersion method
3. Update all call sites (app.go, seed/*.go)
4. Create seed/version.go
5. Add startup wiring in app.go
6. Update all test signatures
7. Add new tests
8. Run `go test ./...` and `go vet ./...`
