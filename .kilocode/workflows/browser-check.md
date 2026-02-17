# Browser Check Workflow

Validate portal UI changes using chromedp browser tests. Run this after any frontend changes (templates, CSS, JS, Alpine components).

## Usage

```
/browser-check          # Run smoke tests (default)
/browser-check smoke    # Run smoke tests only
/browser-check dashboard # Run dashboard tests only
/browser-check nav      # Run navigation tests only
/browser-check all      # Run all tests
```

## Prerequisites

1. Ensure server is running:
```bash
./scripts/run.sh status
```

If not running, start it:
```bash
./scripts/run.sh restart
```

2. Wait for health:
```bash
until curl -sf http://localhost:8881/api/health > /dev/null; do sleep 1; done
```

## Input Detection

Parse the user input from the command to determine test category:
- `/browser-check` or `/browser-check smoke` → run smoke tests
- `/browser-check dashboard` → run dashboard tests
- `/browser-check nav` → run navigation tests
- `/browser-check all` → run all tests

## Steps

1. Set variables and create output directory:
```bash
TIMESTAMP=$(date +%Y-%m-%d-%H-%M-%S)
RESULT_DIR="tests/results/$TIMESTAMP"
mkdir -p "$RESULT_DIR"

export VIRE_TEST_URL="${VIRE_TEST_URL:-http://localhost:8881}"
```

2. Run the requested Go test suite:

**For smoke tests:**
```bash
cd /home/bobmc/development/vire-portal
go test -v ./tests/ui -run "^TestSmoke" -timeout 60s 2>&1 | tee "$RESULT_DIR/smoke.log"
```

**For dashboard tests:**
```bash
cd /home/bobmc/development/vire-portal
go test -v ./tests/ui -run "^TestDashboard" -timeout 60s 2>&1 | tee "$RESULT_DIR/dashboard.log"
```

**For nav tests:**
```bash
cd /home/bobmc/development/vire-portal
go test -v ./tests/ui -run "^TestNav" -timeout 60s 2>&1 | tee "$RESULT_DIR/nav.log"
```

**For all tests:**
```bash
cd /home/bobmc/development/vire-portal
go test -v ./tests/ui -run "^TestSmoke|^TestDashboard|^TestNav" -timeout 120s 2>&1 | tee "$RESULT_DIR/all.log"
```

3. Capture screenshots for visual review:
```bash
cd /home/bobmc/development/vire-portal
go run tests/common/screenshot.go -url "$VIRE_TEST_URL/" -out "$RESULT_DIR/landing.png" || true
go run tests/common/screenshot.go -url "$VIRE_TEST_URL/dashboard" -login -out "$RESULT_DIR/dashboard.png" || true
```

4. Generate summary.md from test output:

```bash
LOG_FILE="$RESULT_DIR/smoke.log"
[ -f "$RESULT_DIR/dashboard.log" ] && LOG_FILE="$RESULT_DIR/dashboard.log"
[ -f "$RESULT_DIR/nav.log" ] && LOG_FILE="$RESULT_DIR/nav.log"
[ -f "$RESULT_DIR/all.log" ] && LOG_FILE="$RESULT_DIR/all.log"

SUMMARY_FILE="$RESULT_DIR/summary.md"

PASSED=$(grep -c "^--- PASS:" "$LOG_FILE" 2>/dev/null || echo 0)
FAILED=$(grep -c "^--- FAIL:" "$LOG_FILE" 2>/dev/null || echo 0)
SKIPPED=$(grep -c "^--- SKIP:" "$LOG_FILE" 2>/dev/null || echo 0)

if [ "$FAILED" -eq 0 ]; then STATUS="✅ PASS"; else STATUS="❌ FAIL"; fi

cat > "$SUMMARY_FILE" << EOF
# Browser Test Summary

**Status:** $STATUS
**Timestamp:** $TIMESTAMP
**Base URL:** $VIRE_TEST_URL

## Results

| Metric | Count |
|--------|-------|
| Passed | $PASSED |
| Failed | $FAILED |
| Skipped | $SKIPPED |

## Test Suites Run

EOF

if [ -f "$RESULT_DIR/smoke.log" ]; then
    if grep -q "^--- FAIL:" "$RESULT_DIR/smoke.log" 2>/dev/null; then
        echo "- Smoke tests: ❌" >> "$SUMMARY_FILE"
    else
        echo "- Smoke tests: ✅" >> "$SUMMARY_FILE"
    fi
fi

if [ -f "$RESULT_DIR/dashboard.log" ]; then
    if grep -q "^--- FAIL:" "$RESULT_DIR/dashboard.log" 2>/dev/null; then
        echo "- Dashboard tests: ❌" >> "$SUMMARY_FILE"
    else
        echo "- Dashboard tests: ✅" >> "$SUMMARY_FILE"
    fi
fi

if [ -f "$RESULT_DIR/nav.log" ]; then
    if grep -q "^--- FAIL:" "$RESULT_DIR/nav.log" 2>/dev/null; then
        echo "- Navigation tests: ❌" >> "$SUMMARY_FILE"
    else
        echo "- Navigation tests: ✅" >> "$SUMMARY_FILE"
    fi
fi

cat >> "$SUMMARY_FILE" << EOF

## Screenshots

EOF

for img in "$RESULT_DIR"/*.png; do
    if [ -f "$img" ]; then
        NAME=$(basename "$img")
        echo "- $NAME" >> "$SUMMARY_FILE"
    fi
done

cat >> "$SUMMARY_FILE" << EOF

## Failure Details

\`\`\`
$(grep -B1 -A5 "^--- FAIL:" "$LOG_FILE" 2>/dev/null | head -50 || echo "No failures")
\`\`\`

## Output Locations

- Results: \`$RESULT_DIR/\`
- Full log: \`$LOG_FILE\`
EOF
```

5. Report results:
```bash
cat "$SUMMARY_FILE"
```

## Test Categories

### Smoke Tests (`smoke_test.go`)
- `TestSmokeLandingNoJSErrors` — No JS errors on landing page
- `TestSmokeLandingLoginButtons` — Google/GitHub login buttons visible
- `TestSmokeLandingBranding` — VIRE branding present
- `TestSmokeDashboardLoads` — Dashboard page renders
- `TestSmokeDashboardNoJSErrors` — No JS errors on dashboard
- `TestSmokeDevLoginFlow` — Dev login flow works (dev mode)
- `TestSmokeCSSLoaded` — CSS fonts loaded
- `TestSmokeAlpineInitialized` — Alpine.js initialized

### Dashboard Tests (`dashboard_test.go`)
- `TestDashboardAuthLoad` — Dashboard loads after login
- `TestDashboardNavPresent` — Nav visible after login
- `TestDashboardSections` — Dashboard sections rendered
- `TestDashboardNoJSErrors` — No JS errors
- `TestDashboardAlpineInitialized` — Alpine.js works
- `TestDashboardCSSLoaded` — CSS loaded
- `TestDashboardPanelsClosedOnLoad` — Panels collapsed
- `TestDashboardCollapseToggles` — Panels expand/collapse
- `TestDashboardTabsSwitch` — Tab switching works
- `TestDashboardNoTemplateMarkers` — No raw template markers
- `TestDashboardDesignNoBorderRadius` — Design rules
- `TestDashboardDesignNoBoxShadow` — Design rules

### Navigation Tests (`nav_test.go`)
- `TestNavBrandText` — Nav brand shows VIRE
- `TestNavHamburgerVisible` — Hamburger visible
- `TestNavDropdownHiddenByDefault` — Dropdown hidden
- `TestNavDropdownOpensOnClick` — Dropdown opens
- `TestNavLinksPresent` — Nav links exist
- `TestNavSettingsInDropdown` — Settings in dropdown
- `TestNavLogoutInDropdown` — Logout in dropdown
- `TestNavMobileNavLinksHidden` — Mobile nav hidden
- `TestNavMobileHamburgerVisible` — Mobile hamburger
- `TestNavMobileMenuClosedOnLoad` — Mobile menu closed
- `TestNavMobileMenuOpensCloses` — Mobile menu toggles
- `TestNavDesktopLinksVisible` — Desktop links visible

## Common Library (`tests/common/browser.go`)

Shared utilities for browser testing:
- `NewBrowserContext` — Create headless Chrome context
- `NewJSErrorCollector` — Collect JS errors
- `NavigateAndWait` — Navigate and wait for load
- `LoginAndNavigate` — Dev login + navigate
- `SetViewport` — Set viewport dimensions
- `IsVisible/IsHidden` — Element visibility checks
- `Exists/ElementCount` — DOM queries
- `TextContains` — Text content checks
- `EvalBool` — JavaScript evaluation
- `Click/ClickNav` — Click actions
- `Screenshot` — Capture screenshot
- `RunCheck` — Run selector|state assertion
- `RunChecks` — Run multiple checks from CheckRequest

## Failure Handling

- Exit code 0 = pass, 1 = fail
- JS errors are always checked automatically
- If tests fail, fix issues before committing
- Review screenshots in `$RESULT_DIR/` to diagnose issues
- Check `$RESULT_DIR/summary.md` for pass/fail breakdown

## Integration with Develop Workflow

This workflow is designed to be used by the `develop` workflow for iterative testing:

1. Run `/browser-check dashboard` to test dashboard changes
2. Review failures in summary.md
3. Fix code
4. Re-run specific test: `/browser-check dashboard`
5. Repeat until all pass
