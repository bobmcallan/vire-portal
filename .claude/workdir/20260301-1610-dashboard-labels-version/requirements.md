# Requirements: Dashboard Labels, NET RETURN % Rebase, Portal Version Headers

## Feedback Items
- **fb_aa6c3aef** — Rename ambiguous dashboard labels
- **fb_a9a562df** — Rebase NET RETURN % on total deposited
- **fb_aeaad863** — Send portal version headers to vire-server

## Scope

### Does
1. Rename "TOTAL INVESTED" → "COST BASIS" (row 1, item 2 in dashboard summary)
2. Rename "CAPITAL INVESTED" → "TOTAL DEPOSITED" (row 2, item 1 in dashboard summary)
3. When `hasCapitalData && capitalInvested !== 0`, compute `totalGainPct` as `portfolioGain / capitalInvested * 100` instead of server's `total_net_return_pct`
4. Add `X-Vire-Portal-Version`, `X-Vire-Portal-Build`, `X-Vire-Portal-Commit` static headers in `NewMCPProxy()`

### Does NOT
- Change server-side calculations or API response shapes
- Change Row 2 labels for items 2-4
- Change `totalGain` dollar value
- Add server-side version validation (separate task)
- Change `fmt()` or `pct()` formatting functions

## File Changes

### 1. `pages/dashboard.html`

**Line 58:** Change label text
```
"TOTAL INVESTED" → "COST BASIS"
```

**Line 74:** Change label text
```
"CAPITAL INVESTED" → "TOTAL DEPOSITED"
```

### 2. `pages/static/common.js`

**Lines 220-222:** Modify `totalGainPct` getter in `portfolioDashboard()`

Current:
```javascript
get totalGainPct() {
    return this.portfolioGainPct;
},
```

New:
```javascript
get totalGainPct() {
    if (this.hasCapitalData && this.capitalInvested !== 0) {
        return (this.portfolioGain / this.capitalInvested) * 100;
    }
    return this.portfolioGainPct;
},
```

### 3. `internal/mcp/proxy.go`

**After line 37** (after DisplayCurrency header block): Add three version headers.

```go
// Portal version headers for server compatibility checks
headers.Set("X-Vire-Portal-Version", common.GetVersion())
headers.Set("X-Vire-Portal-Build", common.GetBuild())
headers.Set("X-Vire-Portal-Commit", common.GetGitCommit())
```

No new imports needed — `common` is already imported on line 14.

### 4. `tests/ui/dashboard_test.go`

**Line 212 (comment):** Update to say "COST BASIS"
**Line 219:** Update expected array:
```javascript
const expected = ['TOTAL VALUE', 'COST BASIS', 'NET RETURN $', 'NET RETURN %'];
```
**Line 230:** Update error message to say "COST BASIS"

**Line 546:** Update expected array:
```javascript
const expected = ['TOTAL DEPOSITED', 'CAPITAL GAIN $', 'SIMPLE RETURN %', 'ANNUALIZED %'];
```
**Line 557:** Update error message to say "TOTAL DEPOSITED"

### 5. `internal/handlers/dashboard_stress_test.go`

**Line 535:** Update expected label:
```go
summaryLabels := []string{"TOTAL VALUE", "COST BASIS", "NET RETURN $", "NET RETURN %"}
```

### 6. `internal/mcp/mcp_test.go`

**After existing `TestNewMCPProxy_UserHeaders_EmptyConfig` (line ~2167):** Add new test:

```go
func TestNewMCPProxy_UserHeaders_PortalVersion(t *testing.T) {
    cfg := testConfig()
    p := NewMCPProxy("http://localhost:4242", testLogger(), cfg)

    version := p.UserHeaders().Get("X-Vire-Portal-Version")
    if version == "" {
        t.Error("expected X-Vire-Portal-Version header to be set")
    }
    build := p.UserHeaders().Get("X-Vire-Portal-Build")
    if build == "" {
        t.Error("expected X-Vire-Portal-Build header to be set")
    }
    commit := p.UserHeaders().Get("X-Vire-Portal-Commit")
    if commit == "" {
        t.Error("expected X-Vire-Portal-Commit header to be set")
    }
}
```

**Update `TestNewMCPProxy_UserHeaders_EmptyConfig`:** Add assertions that version headers are always present even with empty config:
```go
if p.UserHeaders().Get("X-Vire-Portal-Version") == "" {
    t.Error("expected X-Vire-Portal-Version header even with empty config")
}
if p.UserHeaders().Get("X-Vire-Portal-Build") == "" {
    t.Error("expected X-Vire-Portal-Build header even with empty config")
}
if p.UserHeaders().Get("X-Vire-Portal-Commit") == "" {
    t.Error("expected X-Vire-Portal-Commit header even with empty config")
}
```

## Edge Cases

- `capitalInvested === 0` with `hasCapitalData === true`: Falls back to `portfolioGainPct` (division guard)
- `hasCapitalData === false`: Falls back to `portfolioGainPct` (server value)
- Version headers in dev mode: Values are `"dev"`, `"unknown"`, `"unknown"` — acceptable, lets server distinguish dev builds
- Version header values are compile-time constants, not user input — no CRLF sanitization needed

## Implementation Order

1. `pages/dashboard.html` — label text changes
2. `pages/static/common.js` — totalGainPct getter logic
3. `internal/mcp/proxy.go` — version headers
4. `internal/handlers/dashboard_stress_test.go` — update expected label
5. `tests/ui/dashboard_test.go` — update expected label arrays
6. `internal/mcp/mcp_test.go` — add version header test
