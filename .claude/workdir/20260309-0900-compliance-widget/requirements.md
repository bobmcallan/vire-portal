# Requirements: Dashboard Compliance Widget

**Feature:** Develop D from fb_bd98d8e5 — Compliance dashboard widget with 4 states
**Work dir:** .claude/workdir/20260309-0900-compliance-widget/

---

## Scope

Add a compliance status widget to the dashboard page between the breadth bar and holdings table. The widget shows the latest Gemini compliance report with four display states. Page load never blocks on compliance — data is fetched via SSR in parallel with existing endpoints. Users can trigger a fresh review via a "Run Review" button.

**NOT in scope:** Price monitor, ticker registry, event bus, dirty flag infrastructure (Develop A/B). The `dirty` field comes from the server — the portal just displays it.

---

## API Endpoints (already exist on vire-server)

### GET /api/compliance/latest?portfolio_name={name}
Returns cached compliance report. Zero external API calls.

**Response (report exists):**
```json
{
  "report": {
    "portfolio_name": "SMSF",
    "generated_at": "2026-03-09T08:45:58Z",
    "dirty": false,
    "triggered_by": "manual",
    "disclaimer": "Informational only. No buy/sell instructions.",
    "summary": { "breach_count": 1, "warning_count": 3, "info_count": 4 },
    "findings": [
      { "severity": "BREACH", "ticker": "SXE", "message": "Holding SXE contradicts..." },
      { "severity": "WARNING", "ticker": "PMGOLD", "message": "Position weight 20.97%..." }
    ]
  }
}
```

**Response (no report):**
```json
{ "message": "No compliance report yet.", "report": null }
```

### POST /api/compliance/run
Triggers fresh Gemini evaluation. Returns report in same structure.

**Request body:** `{"portfolio_name": "SMSF", "force_refresh": true}`
**Response:** Same as GET /api/compliance/latest (with fresh report).

---

## Four Widget States

| State | Condition | Display |
|-------|-----------|---------|
| Fresh, clean | `report != null && !dirty && breach+warning == 0` | "Reviewed 14:32 -- no issues" |
| Fresh, issues | `report != null && !dirty && breach+warning > 0` | "Reviewed 14:32 -- 1 breach, 2 warnings [View]" |
| Dirty | `report != null && dirty` | "Prices moved since last review -- [Run Review]" |
| Never run | `report == null` | "No review yet -- [Run Review]" |

---

## File Changes

### 1. `internal/handlers/dashboard.go`

**Add compliance SSR fetch as 5th parallel goroutine (inside the `wg.Add(4)` block).**

Change `wg.Add(4)` to `wg.Add(5)`. Add new goroutine:

```go
// Add new template.JS variable at line 96:
var complianceJSON template.JS
complianceJSON = "null"

// Inside the wg block, add 5th goroutine:
go func() {
    defer wg.Done()
    tc := time.Now()
    if cBody, err := h.proxyGetFn("/api/compliance/latest?portfolio_name="+escapedName, userID); err == nil {
        complianceJSON = SafeJS(cBody)
        if h.logger != nil {
            h.logger.Info().Int64("duration_ms", time.Since(tc).Milliseconds()).Str("portfolio", selected).Msg("dashboard SSR: compliance")
        }
    } else if h.logger != nil {
        h.logger.Warn().Int64("duration_ms", time.Since(tc).Milliseconds()).Str("portfolio", selected).Str("error", err.Error()).Msg("dashboard SSR: compliance failed")
    }
}()
```

**Add to template data map:**
```go
"ComplianceJSON": complianceJSON,
```

### 2. `pages/dashboard.html`

**Add compliance to SSR hydration block (after glossary, before selectedPortfolio):**
```html
<script>
window.__VIRE_DATA__ = {
    portfolios: {{.PortfoliosJSON}},
    portfolio: {{.PortfolioJSON}},
    timeline: {{.TimelineJSON}},
    watchlist: {{.WatchlistJSON}},
    glossary: {{.GlossaryJSON}},
    compliance: {{.ComplianceJSON}},
    selectedPortfolio: {{.SelectedJSON}}
};
</script>
```

**Add compliance widget HTML between breadth bar (after line 162) and holdings table (before line 164):**

```html
<!-- Compliance widget -->
<section class="compliance-widget" x-show="!loading && !portfolioLoading" x-cloak>
    <div class="compliance-header" :class="complianceHeaderClass">
        <span class="compliance-title">COMPLIANCE</span>
        <span class="compliance-status" x-text="complianceStatusText"></span>
        <span class="compliance-actions">
            <button class="btn-compliance-run" x-show="complianceShowRun" @click="runComplianceReview()" :disabled="complianceRunning">
                <span x-show="!complianceRunning">Run Review</span>
                <span x-show="complianceRunning">Reviewing...</span>
            </button>
            <button class="btn-compliance-toggle" x-show="complianceHasFindings" @click="complianceExpanded = !complianceExpanded" x-text="complianceExpanded ? 'Hide' : 'View'"></button>
        </span>
    </div>
    <div class="compliance-findings" x-show="complianceExpanded && complianceHasFindings" x-cloak>
        <template x-for="f in complianceFindings" :key="f.message">
            <div class="compliance-finding" :class="'compliance-' + f.severity.toLowerCase()">
                <span class="compliance-severity" x-text="f.severity"></span>
                <span class="compliance-ticker" x-show="f.ticker" x-text="f.ticker"></span>
                <span class="compliance-message" x-text="f.message"></span>
            </div>
        </template>
        <div class="compliance-disclaimer" x-text="complianceDisclaimer"></div>
    </div>
</section>
```

### 3. `pages/static/common.js` — portfolioDashboard() component

**Add data properties (after `watchlist: []` at ~line 258):**
```javascript
complianceReport: null,
complianceExpanded: false,
complianceRunning: false,
```

**Add computed getters (after existing getters):**
```javascript
get complianceState() {
    if (!this.complianceReport) return 'never';
    if (this.complianceReport.dirty) return 'dirty';
    const s = this.complianceReport.summary;
    if (s && (s.breach_count > 0 || s.warning_count > 0)) return 'issues';
    return 'clean';
},
get complianceStatusText() {
    const r = this.complianceReport;
    if (!r) return 'No review yet';
    const time = this._fmtComplianceTime(r.generated_at);
    if (r.dirty) return 'Prices moved since last review';
    const s = r.summary;
    if (!s || (s.breach_count === 0 && s.warning_count === 0)) return 'Reviewed ' + time + ' \u2014 no issues';
    const parts = [];
    if (s.breach_count > 0) parts.push(s.breach_count + ' breach' + (s.breach_count > 1 ? 'es' : ''));
    if (s.warning_count > 0) parts.push(s.warning_count + ' warning' + (s.warning_count > 1 ? 's' : ''));
    return 'Reviewed ' + time + ' \u2014 ' + parts.join(', ');
},
get complianceHeaderClass() {
    const state = this.complianceState;
    return 'compliance-state-' + state;
},
get complianceShowRun() {
    return this.complianceState === 'never' || this.complianceState === 'dirty';
},
get complianceHasFindings() {
    const r = this.complianceReport;
    return r && r.findings && r.findings.length > 0;
},
get complianceFindings() {
    if (!this.complianceReport || !this.complianceReport.findings) return [];
    return this.complianceReport.findings;
},
get complianceDisclaimer() {
    return this.complianceReport?.disclaimer || '';
},
```

**Add methods:**
```javascript
_fmtComplianceTime(isoStr) {
    if (!isoStr) return '';
    const d = new Date(isoStr);
    if (isNaN(d.getTime())) return '';
    return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
},

_applyComplianceData(data) {
    this.complianceReport = data?.report || null;
},

async fetchCompliance() {
    try {
        const res = await vireStore.fetch('/api/compliance/latest?portfolio_name=' + encodeURIComponent(this.selected));
        if (res.ok) {
            const data = await res.json();
            this._applyComplianceData(data);
        }
    } catch (e) {
        console.warn('[dashboard] compliance fetch failed:', e);
    }
},

async runComplianceReview() {
    this.complianceRunning = true;
    try {
        const res = await vireStore.fetch('/api/compliance/run', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ portfolio_name: this.selected, force_refresh: true })
        });
        if (res.ok) {
            const data = await res.json();
            this._applyComplianceData(data);
            this.complianceExpanded = true;
        }
    } catch (e) {
        console.warn('[dashboard] compliance run failed:', e);
    } finally {
        this.complianceRunning = false;
    }
},
```

**SSR hydration (in init(), after watchlist hydration ~line 408):**
```javascript
if (ssrData.compliance) {
    this._applyComplianceData(ssrData.compliance);
}
```

**Client-side fallback (in the fallback init path, after fetchWatchlist):**
Add `this.fetchCompliance()` call.

**Portfolio switching (in loadPortfolio() method, after watchlist fetch):**
Add `this.fetchCompliance()` call.

### 4. `pages/static/css/portal.css`

**Add after the watchlist verdict styles (~line 1026):**

```css
/* Compliance widget */
.compliance-widget {
    margin-bottom: 1.5rem;
    border: 2px solid #000;
}

.compliance-header {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    padding: 0.5rem 1rem;
    font-size: 0.75rem;
    font-weight: 700;
    letter-spacing: 0.1em;
    text-transform: uppercase;
}

.compliance-state-clean {
    background: #2d8a4e;
    color: #fff;
}

.compliance-state-issues {
    background: #a33;
    color: #fff;
}

.compliance-state-dirty {
    background: #b58a1b;
    color: #fff;
}

.compliance-state-never {
    background: #888;
    color: #fff;
}

.compliance-title {
    /* Fixed label */
}

.compliance-status {
    flex: 1;
    font-weight: 400;
    text-transform: none;
    letter-spacing: 0;
}

.compliance-actions {
    display: flex;
    gap: 0.5rem;
}

.btn-compliance-run,
.btn-compliance-toggle {
    background: rgba(255,255,255,0.2);
    color: #fff;
    border: 1px solid rgba(255,255,255,0.4);
    padding: 0.2rem 0.6rem;
    font-size: 0.6875rem;
    font-weight: 600;
    letter-spacing: 0.05em;
    cursor: pointer;
    text-transform: uppercase;
}

.btn-compliance-run:hover,
.btn-compliance-toggle:hover {
    background: rgba(255,255,255,0.3);
}

.btn-compliance-run:disabled {
    opacity: 0.6;
    cursor: not-allowed;
}

.compliance-findings {
    padding: 0.75rem 1rem;
    border-top: 1px solid #ddd;
}

.compliance-finding {
    padding: 0.4rem 0;
    font-size: 0.8125rem;
    border-bottom: 1px solid #eee;
    display: flex;
    gap: 0.5rem;
    align-items: baseline;
}

.compliance-finding:last-of-type {
    border-bottom: none;
}

.compliance-severity {
    font-size: 0.6875rem;
    font-weight: 700;
    letter-spacing: 0.05em;
    padding: 0.1rem 0.4rem;
    border-radius: 2px;
    flex-shrink: 0;
}

.compliance-BREACH .compliance-severity,
.compliance-breach .compliance-severity {
    background: #a33;
    color: #fff;
}

.compliance-WARNING .compliance-severity,
.compliance-warning .compliance-severity {
    background: #b58a1b;
    color: #fff;
}

.compliance-INFO .compliance-severity,
.compliance-info .compliance-severity {
    background: #888;
    color: #fff;
}

.compliance-ticker {
    font-weight: 700;
    font-size: 0.8125rem;
    flex-shrink: 0;
}

.compliance-message {
    color: #333;
}

.compliance-disclaimer {
    margin-top: 0.5rem;
    padding-top: 0.5rem;
    border-top: 1px solid #eee;
    font-size: 0.6875rem;
    color: #888;
    font-style: italic;
}
```

### 5. `internal/handlers/dashboard_stress_test.go`

**Add 5 stress tests following existing patterns:**

```go
func TestDashboardHandler_StressComplianceSSR(t *testing.T)
```
- Set proxyGetFn to return compliance JSON for `/api/compliance/latest*`
- Verify response body contains `"compliance":` in __VIRE_DATA__ block
- Verify `ComplianceJSON` is not `null`

```go
func TestDashboardHandler_StressComplianceNull(t *testing.T)
```
- Set proxyGetFn to return error for compliance endpoint
- Verify page still renders (compliance is optional)
- Verify `"compliance":null` in __VIRE_DATA__

```go
func TestDashboardHandler_StressComplianceXSS(t *testing.T)
```
- Set proxyGetFn to return compliance JSON with `<script>alert('xss')</script>` in finding message
- Verify raw `<script>` does NOT appear in response body (SafeJS escapes it)

```go
func TestDashboardHandler_StressComplianceEmptyReport(t *testing.T)
```
- Return `{"report":null}` from proxyGetFn
- Verify page renders correctly with null report

```go
func TestDashboardHandler_StressComplianceParallelFetch(t *testing.T)
```
- Verify compliance fetch runs in parallel with other SSR fetches (timing test)
- All 6 fetches (portfolios sequential, then 5 parallel) complete within expected time

### 6. `tests/ui/dashboard_test.go`

**Add 3 subtests to existing TestDashboard parent (after watchlist tests, before interactive tests):**

```go
t.Run("ComplianceWidgetVisible", func(t *testing.T) {
    takeScreenshot(t, ctx, "dashboard", "compliance-widget.png")
    exists, err := commontest.Exists(ctx, `.compliance-widget`)
    if err != nil {
        t.Fatalf("error checking compliance widget: %v", err)
    }
    if !exists {
        t.Skip("compliance widget not visible (compliance may not be configured)")
    }
})

t.Run("ComplianceWidgetState", func(t *testing.T) {
    takeScreenshot(t, ctx, "dashboard", "compliance-state.png")
    exists, err := commontest.Exists(ctx, `.compliance-header`)
    if err != nil {
        t.Fatalf("error checking compliance header: %v", err)
    }
    if !exists {
        t.Skip("compliance header not found")
    }
    // Verify one of the 4 state classes is present
    hasState, err := commontest.EvalBool(ctx, `
        (() => {
            const el = document.querySelector('.compliance-header');
            if (!el) return false;
            return el.classList.contains('compliance-state-clean') ||
                   el.classList.contains('compliance-state-issues') ||
                   el.classList.contains('compliance-state-dirty') ||
                   el.classList.contains('compliance-state-never');
        })()
    `)
    if err != nil {
        t.Fatalf("error checking compliance state: %v", err)
    }
    if !hasState {
        t.Error("compliance header missing state class")
    }
})

t.Run("ComplianceNoTemplateMarkers", func(t *testing.T) {
    takeScreenshot(t, ctx, "dashboard", "compliance-no-markers.png")
    // Check compliance widget area for raw template markers
    hasMarker, err := commontest.EvalBool(ctx, `
        (() => {
            const el = document.querySelector('.compliance-widget');
            if (!el) return false;
            const text = el.textContent;
            return text.includes('{{') || text.includes('<no value>');
        })()
    `)
    if err != nil {
        t.Fatalf("error checking template markers: %v", err)
    }
    if hasMarker {
        t.Error("raw template markers found in compliance widget")
    }
})
```

---

## Edge Cases

1. **No compliance report exists** — widget shows "No review yet" with grey header, Run Review button
2. **Compliance API unavailable** — SSR returns null, widget renders "never" state gracefully
3. **XSS in finding messages** — SafeJS on server side, Alpine x-text on client side (both escape)
4. **Portfolio switch** — fetchCompliance() called, widget updates to new portfolio's report
5. **Run Review while already running** — button disabled via `:disabled="complianceRunning"`
6. **Empty findings array** — treat as "clean" state (no issues)
7. **Large number of findings** — findings list scrollable, no truncation

---

## Dependencies

- No new Go packages
- No new npm packages
- No new API routes on portal (uses existing proxy via vireStore.fetch)
- Requires vire-server compliance endpoints (already deployed)
