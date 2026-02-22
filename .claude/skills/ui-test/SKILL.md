# UI Test Skill

Validate portal UI changes using chromedp browser tests. Run this after any frontend changes (templates, CSS, JS, Alpine components).

## Usage

```
/ui-test          # Run smoke tests (default)
/ui-test [suite]  # Run tests for specific suite (e.g. smoke, dashboard, nav, auth)
/ui-test all      # Run all tests
```

## Required Output Structure

**Every test run MUST produce the following artifacts:**

```
tests/results/{timestamp}/
├── {suite}.log       # Full test output (REQUIRED)
├── summary.md        # Pass/fail summary (REQUIRED)
└── *.png             # Screenshots from failures (if any)
```

### summary.md Format

```markdown
# Test Summary: {suite}

**Status:** ✅ PASS | ❌ FAIL
**Timestamp:** YYYY-MM-DD HH:MM:SS
**Server:** http://localhost:8881

## Results

| Metric | Count |
|--------|-------|
| Passed | N |
| Failed | N |
| Skipped | N |

## Artifacts

- Log: `{suite}.log`

### Screenshots (N)
- `TestName_FAIL.png`

## Failures

- **TestName**: Error message
```

## Pre-Test Checks (REQUIRED)

Before running any tests, verify:

1. **Server health check:**
   ```bash
   curl -sf http://localhost:8881/api/health || echo "Server not running"
   ```

2. **If server not running, STOP and inform user.** Do not attempt to fix.

## Test Execution

Run tests using the wrapper script (ensures artifact collection):

```bash
./scripts/ui-test.sh dashboard
```

Or manually with artifact capture:

```bash
# Create results directory
TIMESTAMP=$(date +"%Y-%m-%d-%H-%M-%S")
RESULT_DIR="tests/results/${TIMESTAMP}"
mkdir -p "$RESULT_DIR"

# Run tests with log capture
go test -v ./tests/ui -run "^TestDashboard" -timeout 120s 2>&1 | tee "$RESULT_DIR/dashboard.log"

# Generate summary (MUST happen)
# ... see scripts/ui-test.sh for full implementation
```

## Test Categories

Available suites in `tests/ui/*.go`:
- **Smoke**: `TestSmoke*` (basic health checks)
- **Dashboard**: `TestDashboard*` (dashboard UI/logic)
- **Nav**: `TestNav*` (navigation bar)
- **Auth**: `TestAuth*` (login flows)

## Common Library (`tests/common/`)

- `NewBrowserContext` — Create headless Chrome context
- `NewJSErrorCollector` — Collect JS errors
- `NavigateAndWait` — Navigate and wait for load
- `LoginAndNavigate` — Dev login + navigate
- `IsVisible/IsHidden` — Element visibility checks
- `ElementCount` — DOM queries
- `TextContains` — Text content checks
- `EvalBool` — JavaScript evaluation
- `Screenshot` — Capture screenshot

## Test Writing Pattern

Screenshots are captured **on failure only** — the `TestRunner.RunTest` framework captures a failure screenshot automatically. Add manual screenshots only when verifying a visual state change (developer's choice).

```go
func TestSomething(t *testing.T) {
    ctx, cancel := newBrowser(t)
    defer cancel()

    err := loginAndNavigate(ctx, serverURL()+"/dashboard")
    if err != nil {
        t.Fatalf("login failed: %v", err)
    }

    visible, err := isVisible(ctx, ".some-element")
    if err != nil {
        t.Fatalf("error checking visibility: %v", err)
    }
    if !visible {
        t.Fatal("element not visible")
    }
}
```

### Screenshot Convention

- `{TestName}_FAIL.png` — captured automatically on test failure
- Manual screenshots only when needed to verify a visual state change

## Failure Handling

1. Check `summary.md` for overview
2. Check `{suite}.log` for detailed output
3. Review `*_FAIL.png` screenshots for visual context
4. Fix issues and re-run

## Process Rules

1. **NEVER skip artifact generation** - Tests without logs are useless
2. **TRUST USER INPUT** - If user says server is running, believe them
3. **FAIL FAST** - If preconditions fail, stop and report clearly
4. **CAPTURE EVERYTHING** - Every failure needs a screenshot

## Integration with Develop Workflow

1. Run `/ui-test dashboard` to test dashboard changes
2. Review artifacts in `tests/results/{timestamp}/`
3. Fix code
4. Re-run: `/ui-test dashboard`
5. Repeat until all pass
