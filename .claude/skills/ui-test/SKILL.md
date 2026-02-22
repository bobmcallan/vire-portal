# /ui-test - UI Test Review and Execution

Validate portal UI changes using chromedp browser tests. This skill has two mandatory phases: **review** (compliance check) and **execute** (run with result capture).

**Both phases are mandatory. Tests MUST be reviewed before execution. Execution MUST capture results.**

## Usage

```
/ui-test                    # Review + execute smoke tests (default)
/ui-test [suite]            # Review + execute specific suite
/ui-test all                # Review + execute all suites
/ui-test review [suite]     # Review only (no execution)
/ui-test execute [suite]    # Execute only (skip review)
```

**Suites:** `smoke`, `dashboard`, `nav`, `auth`, `devauth`, `mcp`, `all`

## Mandatory Rules

All UI tests MUST comply with these rules. These are non-negotiable.

### Rule 1: Tests Are Independent of Claude

Tests MUST be executable via standard `go test`. No test may depend on Claude, MCP, or any AI tooling to run. Every test must pass with:

```bash
go test -v ./tests/ui -run "^TestSuite" -timeout 120s
```

### Rule 2: Common Browser Setup

All UI tests MUST use the shared browser setup from `tests/common/`:
- `NewBrowserContext(cfg)` for headless Chrome
- `LoginAndNavigate(ctx, url, timeout)` for authenticated pages
- `JSErrorCollector` for capturing JS errors

### Rule 3: Test Results Output

All test execution MUST produce results in:

```
tests/results/{YYYY-MM-DD-HH-MM-SS}/
├── {suite}.log       # Full test output (REQUIRED)
├── summary.md        # Pass/fail summary (REQUIRED)
└── *.png             # Screenshots from failures (if any)
```

This is achieved by running tests via the wrapper script `./scripts/ui-test.sh` which captures output via `tee` and generates `summary.md`.

### Rule 4: Execute Is Read-Only

The execute phase MUST NEVER modify test files. If compliance issues are found during review, they must be fixed before execution.

## Phase 1: Review (Mandatory Before Execution)

Check each test file in the target suite against the compliance rules.

### Step 1: Read Test Files

Read the test files for the target suite:

| Suite | File |
|-------|------|
| smoke | `tests/ui/smoke_test.go` |
| dashboard | `tests/ui/dashboard_test.go` |
| nav | `tests/ui/nav_test.go` |
| auth | `tests/ui/auth_test.go` |
| devauth | `tests/ui/dev_auth_test.go` |
| mcp | `tests/ui/mcp_test.go` |

### Step 2: Check Compliance

| # | Rule | What to Check |
|---|------|---------------|
| 1 | Independent of Claude | No Claude/AI imports or runtime dependencies |
| 2 | Common browser setup | Uses `newBrowser(t)`, `loginAndNavigate()`, helpers from `ui_helpers_test.go` |
| 3 | Correct selectors | CSS selectors match current HTML templates in `pages/` |
| 4 | Standard Go patterns | Uses `t.Fatal()`, `t.Error()`, `t.Skip()`, `t.Logf()` correctly |
| 5 | No stale references | No selectors for removed/renamed elements |

### Step 3: Report Compliance

```
# UI Test Review: {suite}

## Compliance
- Files checked: N
- Compliant: N
- Non-compliant: N

## Issues (if any)
- `file_test.go:45` — stale selector `.old-class` (element removed)
- `file_test.go:72` — missing test for new `.new-element`

## Recommendation
Fix issues before execution.
```

**If non-compliant:** Fix the issues, then proceed to execution.

## Phase 2: Execute (Mandatory Result Capture)

### Step 1: Pre-Flight Check

```bash
# Check server is running (use test config URL)
curl -sf http://localhost:8883/api/health || echo "Server not running — start with ./scripts/run.sh restart"
```

If server is not running, STOP and report. Do not attempt to fix.

### Step 2: Execute via Wrapper Script

**CRITICAL: Always use the wrapper script for test execution.** Never run `go test` directly — the wrapper captures output, generates summary, and collects artifacts.

```bash
# Run specific suite
./scripts/ui-test.sh dashboard

# Run all suites
./scripts/ui-test.sh all

# Available suites: smoke, dashboard, nav, auth, devauth, mcp, all
```

The wrapper script:
1. Creates timestamped results directory
2. Checks server health
3. Runs `go test -v` with output captured via `tee` to `{suite}.log`
4. Parses output for pass/fail/skip counts
5. Generates `summary.md` with results and artifact list
6. Exits with the test exit code

### Step 3: Read and Report Results

**MANDATORY: After execution, read the results and report them.**

```bash
# Find latest results
LATEST=$(ls -td tests/results/*/ | head -1)

# Read summary
cat "$LATEST/summary.md"

# List all artifacts
ls -la "$LATEST"
```

The summary and log contents MUST be included in the completion report. Do not just say "tests passed" — show the actual results.

### Step 4: Handle Failures

If tests fail:
1. Read `{suite}.log` for failure details
2. Check `*_FAIL.png` screenshots for visual context
3. Fix the code (if you have write access) or report the failures
4. Re-run via wrapper script
5. Repeat until all pass

## Test Writing Pattern

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

## Common Library (`tests/common/`)

| Function | Purpose |
|----------|---------|
| `NewBrowserContext` | Create headless Chrome context |
| `NewJSErrorCollector` | Collect JS console errors |
| `NavigateAndWait` | Navigate and wait for page load |
| `LoginAndNavigate` | Dev login + navigate to URL |
| `IsVisible` / `IsHidden` | Element visibility checks |
| `ElementCount` | Count DOM elements |
| `TextContains` | Check element text content |
| `EvalBool` | Evaluate JavaScript expression |
| `Screenshot` | Capture screenshot to file |

## Integration with /develop

When called from the `/develop` workflow (Phase 2b):

1. The implementer MUST use `./scripts/ui-test.sh` for execution
2. The implementer MUST read `summary.md` after execution
3. The implementer MUST send the summary contents to the team lead
4. The team lead MUST verify results exist in `tests/results/`
5. If tests fail, the implementer MUST fix and re-run before marking Phase 2b complete
