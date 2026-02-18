# UI Test Workflow

Validate portal UI changes using chromedp browser tests. Run this after any frontend changes (templates, CSS, JS, Alpine components).

## Usage

```
/ui-test          # Run smoke tests (default)
/ui-test [suite]  # Run tests for specific suite (e.g. smoke, dashboard, nav, auth)
/ui-test all      # Run all tests
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
- `/ui-test` or `/ui-test smoke` → run smoke tests
- `/ui-test [category]` → run tests matching `Test[Category]*` (case-insensitive)
- `/ui-test all` → run all tests

## Steps

1. Run the requested Go test suite (config loaded from `tests/ui/test_config.toml`):

**For specific suite:**
```bash
# Extract category from input (default: smoke)
CATEGORY="${1:-smoke}"

if [ "$CATEGORY" = "all" ]; then
    PATTERN="."
else
    # Capitalize first letter (e.g. smoke -> Smoke)
    TITLE_CASE="$(tr '[:lower:]' '[:upper:]' <<< ${CATEGORY:0:1})${CATEGORY:1}"
    PATTERN="^Test${TITLE_CASE}"
fi

go test -v ./tests/ui -run "$PATTERN" -timeout 120s | tee "$RESULT_DIR/${CATEGORY}.log"
```

2. Check results in `tests/results/{timestamp}/`:
```bash
LATEST=$(ls -td tests/results/*/ | head -1)
echo "Results: $LATEST"
ls -la "$LATEST"
```

3. Generate summary from test output:
```bash
LATEST=$(ls -td tests/results/*/ | head -1)
# Find the log file (e.g. smoke.log, auth.log)
LOG_FILE=$(ls "$LATEST"*.log 2>/dev/null | head -1)
SUMMARY_FILE="$LATEST/summary.md"

PASSED=$(grep -c "^--- PASS:" "$LOG_FILE" 2>/dev/null || echo 0)
FAILED=$(grep -c "^--- FAIL:" "$LOG_FILE" 2>/dev/null || echo 0)
SKIPPED=$(grep -c "^--- SKIP:" "$LOG_FILE" 2>/dev/null || echo 0)

if [ "$FAILED" -eq 0 ]; then STATUS="✅ PASS"; else STATUS="❌ FAIL"; fi

cat > "$SUMMARY_FILE" << EOF
# Test Summary

**Status:** $STATUS
**Log:** $LOG_FILE

| Metric | Count |
|--------|-------|
| Passed | $PASSED |
| Failed | $FAILED |
| Skipped | $SKIPPED |

## Screenshots
EOF

for img in "$LATEST"*.png; do
    if [ -f "$img" ]; then
        echo "- $(basename "$img")" >> "$SUMMARY_FILE"
    fi
done

echo "" >> "$SUMMARY_FILE"
echo "## Failure Details" >> "$SUMMARY_FILE"
grep -B1 -A5 "^--- FAIL:" "$LOG_FILE" 2>/dev/null | head -20 >> "$SUMMARY_FILE" || echo "No failures." >> "$SUMMARY_FILE"
```

4. Report results:
```bash
LATEST=$(ls -td tests/results/*/ | head -1)
cat "$LATEST/summary.md"
```

## Test Categories

The workflow dynamically matches tests based on `Test<Category>*` naming convention.

Available suites in `tests/ui/*.go`:
- **Smoke**: `TestSmoke*` (basic health checks)
- **Dashboard**: `TestDashboard*` (dashboard UI/logic)
- **Nav**: `TestNav*` (navigation bar)
- **Auth**: `TestAuth*` (login flows)

See `tests/ui/` for full test list.

## Common Library (`tests/common/`)

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
- Review screenshots in `$LATEST/` to diagnose issues

## Integration with Develop Workflow

This workflow is designed to be used by the `develop` workflow for iterative testing:

1. Run `/ui-test dashboard` to test dashboard changes
2. Review failures in results directory
3. Fix code
4. Re-run specific test: `/ui-test dashboard`
5. Repeat until all pass
