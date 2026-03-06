# /test-common - Shared Test Infrastructure

Documentation for vire-portal's test infrastructure patterns and mandatory rules.

## Mandatory Rules

All portal tests MUST comply with these rules. These are non-negotiable.

### Rule 1: Tests Are Independent of Claude

Tests MUST be executable via standard Go tooling. No test may depend on Claude, MCP, or any AI tooling to run. Every test must pass with:

```bash
go test ./tests/ui/... -run "^TestSuite" -timeout 120s
```

Tests may be *created* or *reviewed* by Claude skills, but their execution must never require Claude.

### Rule 2: Common Browser Setup, Docker Container

All UI tests MUST use the shared setup from `tests/common/`:
- `StartPortal(t)` or `StartPortalForTestMain()` for Docker container lifecycle
- `NewBrowserContext(cfg)` for headless Chrome
- `LoginAndNavigate(ctx, url, timeout)` for authenticated pages
- `JSErrorCollector` for capturing JS errors

The portal container is shared per test process via `sync.Once`. Tests always start their own Docker containers to ensure the latest vire-server image is used.

### Rule 3: Test Results Output

All test execution MUST produce results in:

```
tests/logs/{YYYYMMDD-HHMMSS}/
├── {suite}.log       # Full test output (REQUIRED)
├── summary.md        # Pass/fail summary (REQUIRED)
├── container.log     # Portal container logs (Docker mode)
└── {suite}/          # Validation screenshots (REQUIRED, see Rule 4)
    └── *.png         # One per test — visual proof of page state
```

This is achieved by running tests via `./scripts/ui-test.sh` which captures output via `tee` and generates `summary.md`. Container logs are collected automatically by `TestMain`.

### Rule 4: Validation Screenshots Are Mandatory

Every UI test MUST capture at least one validation screenshot using `takeScreenshot(t, ctx, "suite", "name.png")`. Screenshots are the primary evidence that the test saw the correct page state. They are NOT optional and NOT only for failures.

**Where to place the screenshot call:**
- After navigation/login completes and the page is in the state being validated
- Before assertions run (so the screenshot captures the state regardless of pass/fail)
- After interactions that change page state (clicks, scrolls, form submissions)

**Naming convention:** `takeScreenshot(t, ctx, "<suite>", "<test-slug>.png")`
- Suite matches the test file name (e.g., `settings`, `dashboard`, `nav`)
- Test slug is a short kebab-case description (e.g., `page-layout`, `form-elements`, `section-border`)

**Example:**
```go
func TestSettingsPageLayout(t *testing.T) {
    ctx, cancel := newBrowser(t)
    defer cancel()

    err := loginAndNavigate(ctx, serverURL()+"/profile")
    if err != nil {
        t.Fatal(err)
    }

    takeScreenshot(t, ctx, "settings", "page-layout.png")  // MANDATORY

    visible, err := isVisible(ctx, "main.page")
    // ... assertions ...
}
```

Screenshots are saved to `tests/logs/{timestamp}/{suite}/name.png` and serve as visual proof that the page rendered correctly. A test without a screenshot is incomplete.

### Rule 5: Docker Container Safety

Test containers use the `-tc` suffix (`vire-db-tc`, `vire-server-tc`, `vire-portal-tc`) and are managed exclusively by `containers.go` and `ui-test.sh`. These rules are non-negotiable:

1. **NEVER run `docker rm`, `docker stop`, `docker kill`, or any destructive Docker command** outside of the test infrastructure code. The test setup handles stale container cleanup automatically.
2. **NEVER touch containers without the `-tc` suffix.** The user's dev stack containers (`vire-server`, `vire-surrealdb`, etc.) must never be stopped, removed, or modified by tests or test tooling.
3. If a container name conflict occurs, the fix belongs in `containers.go` (code), not in a manual shell command.

### Rule 6: No Files in Project Root

Tests and test tooling MUST NEVER create files in the project root directory. All test output goes to `tests/logs/{timestamp}/`. Never redirect test output to ad-hoc files like `test_execution.log`, `output.log`, etc. in the project root or anywhere outside `tests/logs/`.

### Rule 7: test-execute Is Read-Only

`/test-execute` MUST NEVER modify or update test files. Its role is:
1. Validate test structure compliance (Rules 1-3) before running
2. Execute the tests
3. Report results and any structural non-compliance

If structural issues are found, `/test-execute` documents them and advises using `/test-create-review` to fix.

### Rule 8: Alpine Pages Must Assert No JS Console Errors

Any test that loads a page using Alpine.js data fetching (`x-data`, `x-init`, Alpine components with API calls) **MUST** collect and assert no JS console errors:

```go
func TestMyAlpinePage(t *testing.T) {
    ctx, cancel := newBrowser(t)
    defer cancel()

    errs := newJSErrorCollector(ctx)  // MUST be set up BEFORE navigation

    err := loginAndNavigate(ctx, serverURL()+"/my-page")
    if err != nil {
        t.Fatalf("login failed: %v", err)
    }

    // Wait for Alpine component to initialise and fetch data
    chromedp.Run(ctx, chromedp.Sleep(2*time.Second))

    takeScreenshot(t, ctx, "suite", "name.png")

    if jsErrs := errs.Errors(); len(jsErrs) > 0 {
        t.Errorf("JS errors on page:\n  %v", jsErrs)
    }
}
```

**Why this matters:** Alpine warnings such as `Duplicate key on x-for`, undefined reactive properties, and IntersectionObserver errors do NOT throw exceptions — they only appear in the browser console. Without explicit JS error collection, they are invisible to tests and slip through to production.

**What to catch:**
- `Alpine Warning: Duplicate key on x-for` — duplicate array IDs, usually from concurrent fetches
- `Alpine Warning: ...` — any Alpine runtime warning
- `Uncaught TypeError` / `Uncaught ReferenceError` — unhandled JS errors
- Network errors logged to console

**Scope:** All pages using `x-data` + `x-init` + any API `fetch()` call. Static Alpine bindings (e.g., hamburger toggle only) are exempt.

## Test Environment Setup

### Docker Mode (Default)

Tests auto-start a portal Docker container via testcontainers-go:

```go
func TestMain(m *testing.M) {
    pc, err := commontest.StartPortalForTestMain()
    if err != nil {
        fmt.Fprintf(os.Stderr, "Failed to start portal: %v\n", err)
        os.Exit(1)
    }
    commontest.SetTestURL(pc.URL())

    code := m.Run()

    pc.CollectLogs(commontest.GetResultsDir())
    pc.Cleanup()
    os.Exit(code)
}
```

Tests always run against Docker containers — there is no manual/local mode. This ensures the latest `vire-server:latest` image is always pulled and used.

## Key Components

### `tests/common/containers.go` - Docker Container Management

- `buildPortalImage()` - Builds `vire-portal:test` image once via `sync.Once`
- `StartPortal(t)` - Starts shared container via `sync.Once`, returns `*PortalContainer`
- `StartPortalForTestMain()` - Variant for `TestMain` (no `*testing.T`)
- `PortalContainer.URL()` - Returns mapped URL
- `PortalContainer.CollectLogs(dir)` - Saves container stdout/stderr to `container.log`
- `PortalContainer.Cleanup()` - Terminates container
- Always pulls latest `vire-server:latest` image via `WithAlwaysPull()`

### `tests/common/browser.go` - Browser Helpers

- `NewBrowserContext(cfg)` - Create headless Chrome context
- `NewJSErrorCollector(ctx)` - Collect JS console errors
- `NavigateAndWait(ctx, url, timeout)` - Navigate and wait for page load
- `LoginAndNavigate(ctx, url, timeout)` - Dev login + navigate to URL
- `IsVisible(ctx, sel)` / `IsHidden(ctx, sel)` - Element visibility checks
- `ElementCount(ctx, sel)` - Count DOM elements
- `TextContains(ctx, sel, text)` - Check element text content
- `EvalBool(ctx, expr)` - Evaluate JavaScript expression
- `Screenshot(ctx, path)` - Capture screenshot to file

### `tests/ui/ui_helpers_test.go` - Test Helpers

Wrappers used by all UI test files:
- `newBrowser(t)` - Create browser context with test config
- `serverURL()` - Get server URL (from Docker container)
- `loginAndNavigate(ctx, url)` - Login and navigate
- `takeScreenshot(t, ctx, ...)` - Capture screenshot to results dir
- `isVisible(ctx, sel)` / `isHidden(ctx, sel)` - Visibility helpers
- `assertVisible(ctx, sel, desc)` - Assert element visible
- `assertTextContains(ctx, sel, expected, desc)` - Assert text content

### Test Docker Infrastructure

- `tests/docker/Dockerfile.server` - Multi-stage build for portal test image
- `tests/docker/portal-test.toml` - Test container config (dev mode, port 8080)

## Running Tests

```bash
# Via wrapper script (recommended)
./scripts/ui-test.sh smoke
./scripts/ui-test.sh all

# Direct (auto-starts Docker containers, pulls latest vire-server)
go test -v ./tests/ui/... -run TestSmoke -timeout 120s

# Specific test
go test -v ./tests/ui/... -run TestDashboardSections -timeout 120s
```
