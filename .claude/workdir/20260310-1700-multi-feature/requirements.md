# Requirements: Multi-Feature Implementation

## Feature 1: Stock Page Restructure (fb_7f033ebb)

### 1a: Remove Filings Timeline Section
**File: `pages/stock.html`** — Delete lines 114-160 (entire Filings Timeline section)

**File: `pages/static/common.js`** — Remove `expandedFilings: {}` (line 1334) and `toggleFiling()` method (lines 1366-1368)

### 1b: Reverse Key Events Order
**File: `pages/stock.html`** — Line 173, change:
```html
x-for="evt in detail.company_timeline.key_events"
```
to:
```html
x-for="evt in [...detail.company_timeline.key_events].reverse()"
```

### 1c: Redesign Walk Chart with Breakeven Overlay
**File: `pages/static/common.js`** — Rewrite `renderWalkChart()` (lines 1401-1496)

The server now provides `breakeven_price` and `close_price` per day in `positionTimeline`, plus `trade_events` array.

New chart design:
- **Dataset 0**: Close price line (blue/dark, `#334155`, 2px width)
- **Dataset 1**: Breakeven price line (dashed, `#888`, 1.5px width, `borderDash: [4, 3]`)
- **Fill between datasets**: `fill: { target: 1, above: 'rgba(45, 138, 79, 0.15)', below: 'rgba(192, 96, 96, 0.15)' }` on dataset 0
- **Dataset 2**: Buy scatter markers (green triangles, from `trade_events` where type=Buy)
- **Dataset 3**: Sell scatter markers (red inverted triangles, from `trade_events` where type=Sell)
- Y-axis: price axis (no zero line emphasis)
- Tooltip: show price for close/breakeven, show type + realised P&L for trade markers
- Trade events come from `positionTimeline` entries that have `trade_events` array
- Labels from `positionTimeline[].date`

Build trade markers from positionTimeline:
```js
const buyPoints = [];
const sellPoints = [];
for (let i = 0; i < timeline.length; i++) {
    const p = timeline[i];
    if (p.trade_events) {
        for (const te of p.trade_events) {
            const point = { x: labels[i], y: p.close_price };
            if (te.type === 'Buy') buyPoints.push(point);
            else sellPoints.push(point);
        }
    }
}
```

Guard: if `timeline[0]` has no `close_price` field, fall back to old P&L chart logic.

## Feature 2: BUG Severity Compliance (fb_291d46a0)

### 2a: Add userRole to dashboard SSR data
**File: `pages/dashboard.html`** — Add to `__VIRE_DATA__`:
```js
userRole: "{{.UserRole}}"
```

### 2b: Filter BUG findings from main compliance
**File: `pages/static/common.js`** — Update `complianceFindings` getter:
```js
get complianceFindings() {
    if (!this.complianceReport || !this.complianceReport.findings) return [];
    return this.complianceReport.findings.filter(f => f.severity !== 'BUG');
},
```

Update `complianceHasFindings`:
```js
get complianceHasFindings() {
    return this.complianceFindings.length > 0;
},
```

### 2c: Add BUG findings getter (admin-only)
**File: `pages/static/common.js`** — Add new getters:
```js
get complianceBugFindings() {
    if (!this.complianceReport || !this.complianceReport.findings) return [];
    return this.complianceReport.findings.filter(f => f.severity === 'BUG');
},
get complianceHasBugs() {
    return this.complianceBugFindings.length > 0;
},
get isAdmin() {
    return this.userRole === 'admin';
},
```

Add `userRole: ''` to state init, hydrate from `__VIRE_DATA__.userRole`.

### 2d: Add BUG section to dashboard HTML
**File: `pages/dashboard.html`** — After the compliance-findings div (after line 190), add:
```html
<div class="compliance-bug-indicator" x-show="isAdmin && complianceHasBugs" x-cloak>
    <span class="text-muted" x-text="complianceBugFindings.length + ' data quality issue' + (complianceBugFindings.length !== 1 ? 's' : '')"></span>
    <button class="btn-compliance-toggle" @click="bugFindingsExpanded = !bugFindingsExpanded" x-text="bugFindingsExpanded ? 'Hide' : 'View'"></button>
</div>
<div class="compliance-bug-section" x-show="isAdmin && bugFindingsExpanded && complianceHasBugs" x-cloak>
    <div class="compliance-bug-title">DATA QUALITY ISSUES (ADMIN)</div>
    <template x-for="f in complianceBugFindings" :key="f.message">
        <div class="compliance-finding compliance-bug">
            <span class="compliance-severity">BUG</span>
            <span class="compliance-ticker" x-show="f.ticker" x-text="f.ticker"></span>
            <span class="compliance-message" x-text="f.message"></span>
        </div>
    </template>
</div>
```

Add `bugFindingsExpanded: false` to Alpine state.

### 2e: BUG CSS styling
**File: `pages/static/css/portal.css`** — Add after existing compliance severity styles:
```css
.compliance-bug { border-left-color: #d97706; }
.compliance-bug .compliance-severity { color: #d97706; }
.compliance-bug-indicator { margin-top: 0.5rem; display: flex; align-items: center; gap: 0.5rem; }
.compliance-bug-title { font-weight: 700; font-size: 0.75rem; letter-spacing: 0.05em; margin-bottom: 0.5rem; color: #d97706; }
.compliance-bug-section { margin-top: 0.5rem; }
```

## Feature 3: User Timezone (fb_d41457bd)

### 3a: Add Timezone to UserProfile
**File: `internal/client/vire_client.go`** — Add field:
```go
type UserProfile struct {
    Username         string `json:"username"`
    Email            string `json:"email"`
    Role             string `json:"role"`
    Timezone         string `json:"timezone"`
    NavexaKeySet     bool   `json:"navexa_key_set"`
    NavexaKeyPreview string `json:"navexa_key_preview"`
}
```

### 3b: Profile page timezone section
**File: `pages/profile.html`** — After the Navexa API Key section (after line 53, before `</form>`), add timezone field inside the same form:
```html
<div class="form-group" style="margin-top: 1rem">
    <label for="timezone" class="form-label">TIMEZONE</label>
    <input type="text" id="timezone" name="timezone" class="form-input"
           value="{{.UserTimezone}}" list="tz-list"
           placeholder="e.g. Australia/Sydney">
    <datalist id="tz-list">
        <option value="Australia/Sydney">
        <option value="Australia/Melbourne">
        <option value="Australia/Brisbane">
        <option value="Australia/Perth">
        <option value="Australia/Adelaide">
        <option value="America/New_York">
        <option value="America/Chicago">
        <option value="America/Denver">
        <option value="America/Los_Angeles">
        <option value="Europe/London">
        <option value="Europe/Berlin">
        <option value="Europe/Paris">
        <option value="Asia/Tokyo">
        <option value="Asia/Singapore">
        <option value="Asia/Hong_Kong">
        <option value="Pacific/Auckland">
    </datalist>
</div>
```

### 3c: Pass UserTimezone to profile template
**File: `internal/handlers/profile.go`** — In HandleProfile, after line 111, add:
```go
data["UserTimezone"] = user.Timezone
```

### 3d: Save timezone in HandleSaveProfile
**File: `internal/handlers/profile.go`** — In HandleSaveProfile, change the save call to include timezone:
```go
navexaKey := strings.TrimSpace(r.FormValue("navexa_key"))
timezone := strings.TrimSpace(r.FormValue("timezone"))

fields := map[string]string{}
if navexaKey != "" {
    fields["navexa_key"] = navexaKey
}
if timezone != "" {
    fields["timezone"] = timezone
}

if len(fields) == 0 {
    http.Redirect(w, r, "/profile?saved=1", http.StatusFound)
    return
}

if err := h.userSaveFn(claims.Sub, fields); err != nil {
```

### 3e: Auto-detect browser timezone on dashboard
**File: `pages/static/common.js`** — In dashboard init (after SSR hydration), add timezone auto-detect:
```js
// Auto-detect and store browser timezone
try {
    const tz = Intl.DateTimeFormat().resolvedOptions().timeZone;
    if (tz && document.cookie.includes('vire_session')) {
        const storedTz = sessionStorage.getItem('vire_tz_sent');
        if (storedTz !== tz) {
            fetch('/profile', {
                method: 'POST',
                headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
                body: '_csrf=' + encodeURIComponent(this._getCsrf()) + '&timezone=' + encodeURIComponent(tz)
            }).then(() => sessionStorage.setItem('vire_tz_sent', tz)).catch(() => {});
        }
    }
} catch(e) {}
```

Add `_getCsrf()` helper:
```js
_getCsrf() {
    const c = document.cookie.split('; ').find(c => c.startsWith('_csrf='));
    return c ? c.split('=')[1] : '';
},
```

## Feature 4: Larger Breadth Arrows

**File: `pages/static/css/portal.css`** — Line 1358, change:
```css
.holding-breadth-arrow { font-size: 0.6rem; margin-right: 0.25rem; }
```
to:
```css
.holding-breadth-arrow { font-size: 0.75rem; margin-right: 0.25rem; }
```

## Test Cases

### Unit Tests
- Test complianceFindings filters out BUG severity
- Test complianceBugFindings returns only BUG severity
- Test BUG section HTML present with admin-only x-show
- Test filings section removed from stock.html
- Test walk chart references close_price/breakeven_price
- Test timezone field in UserProfile struct
- Test HandleSaveProfile saves timezone conditionally

### Stress Tests
- Stock page: verify no filing-related selectors
- Dashboard: verify BUG section has admin guard
- Dashboard: verify userRole in __VIRE_DATA__
- Walk chart: verify breakeven datasets

### UI Tests
- Stock page: verify no filings section, key events reversed
- Dashboard: verify BUG section visibility rules
- Profile: timezone field present and editable
