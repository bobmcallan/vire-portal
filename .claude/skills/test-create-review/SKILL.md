# /test-create-review - Create or Review UI Tests

Create new UI tests or review existing tests for compliance. This skill consolidates test creation and compliance review into a single workflow.

**Mandatory rules are defined in `/test-common`. Read them first.**

## Usage
```
/test-create-review <action> [suite]
```

**Actions:**
- `create` -- Scaffold a new test file
- `review` -- Review existing tests for compliance and fix issues
- `audit` -- Review all tests without making changes (report only)

**Examples:**
- `/test-create-review create portfolio` -- Create portfolio UI tests
- `/test-create-review review smoke` -- Review and fix smoke tests
- `/test-create-review review` -- Review and fix all UI tests
- `/test-create-review audit` -- Audit all tests for compliance (no changes)

## Prerequisites

Read `/test-common` for:
- **Mandatory Rules** (compliance requirements for all tests)
- Test environment setup patterns
- Key components and helpers

## Workflow

### Step 1: Determine Test Location

All UI tests live in `tests/ui/`:

| Suite | File |
|-------|------|
| smoke | `tests/ui/smoke_test.go` |
| dashboard | `tests/ui/dashboard_test.go` |
| nav | `tests/ui/nav_test.go` |
| auth | `tests/ui/auth_test.go` |
| devauth | `tests/ui/dev_auth_test.go` |
| mcp | `tests/ui/mcp_test.go` |
| *(new)* | `tests/ui/{name}_test.go` |

### Step 2: For `create` -- Scaffold Using Template

Create the test file using the template below. Ensure compliance with all mandatory rules from `/test-common`.

### Step 3: For `review` -- Check and Fix Compliance

Read the target test files and check each mandatory rule:

#### Compliance Checklist

| # | Rule | Check | Fix |
|---|------|-------|-----|
| 1 | Independent of Claude | No Claude/AI imports or dependencies | Remove any Claude-specific code |
| 2 | Common browser setup | Uses `newBrowser(t)`, helpers from `ui_helpers_test.go` | Replace custom setup with common helpers |
| 3 | Correct selectors | CSS selectors match current HTML in `pages/` | Update selectors to match current templates |
| 4 | No stale references | No selectors for removed/renamed elements | Remove or update stale selectors |
| 5 | Standard Go patterns | Uses `t.Fatal()`, `t.Error()`, `t.Skip()` correctly | Fix assertion patterns |

For each non-compliant item: fix the test file directly, then document what was changed.

### Step 4: For `audit` -- Report Only

Same checks as `review`, but do NOT modify any files. Output a compliance report:

```
# Test Compliance Audit

## Summary
- Files checked: N
- Compliant: N
- Non-compliant: N

## Non-Compliant Files

### tests/ui/file_test.go
- [ ] Rule 3: Stale selector `.old-class` -- element removed from template
- [ ] Rule 5: Using `t.Log` instead of `t.Fatal` for setup failures

## Recommendation
Run `/test-create-review review [suite]` to fix.
```

## UI Test Template

```go
func TestFeature(t *testing.T) {
    ctx, cancel := newBrowser(t)
    defer cancel()

    err := loginAndNavigate(ctx, serverURL()+"/target-page")
    if err != nil {
        t.Fatalf("login failed: %v", err)
    }

    // Check element visibility
    visible, err := isVisible(ctx, ".target-element")
    if err != nil {
        t.Fatalf("error checking visibility: %v", err)
    }
    if !visible {
        t.Fatal("target element not visible")
    }

    // Check text content
    if err := assertTextContains(ctx, ".element", "expected text", "element description"); err != nil {
        t.Error(err)
    }

    // Check element count
    if err := assertElementCount(ctx, ".items li", 3, "list items"); err != nil {
        t.Error(err)
    }

    // Take screenshot on failure
    if t.Failed() {
        takeScreenshot(t, ctx, "feature", "failure.png")
    }
}

func TestFeatureSubtests(t *testing.T) {
    ctx, cancel := newBrowser(t)
    defer cancel()

    err := loginAndNavigate(ctx, serverURL()+"/target-page")
    if err != nil {
        t.Fatalf("login failed: %v", err)
    }

    t.Run("section_visible", func(t *testing.T) {
        if err := assertVisible(ctx, ".section", "main section"); err != nil {
            t.Error(err)
        }
    })

    t.Run("no_js_errors", func(t *testing.T) {
        collector := newJSErrorCollector(ctx)
        // navigate or interact
        if err := assertNoJSErrors(collector); err != nil {
            t.Error(err)
        }
    })
}
```

## Key Patterns

### Browser Setup
Every test uses `newBrowser(t)` which creates a headless Chrome context via chromedp.

### Authentication
Use `loginAndNavigate(ctx, url)` for pages requiring dev auth. Use `navigateAndWait(ctx, url)` for public pages.

### Server URL
Always use `serverURL()` -- never hardcode URLs. This function checks `VIRE_TEST_URL` env var first, then falls back to config.

### Selectors
Selectors must match current HTML templates in `pages/`. Always verify selectors against the actual templates before writing tests.

### Screenshots
Use `takeScreenshot(t, ctx, "suite", "name.png")` to save screenshots to the results directory.

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

## Structural Checklist

Before completing any create or review action:

- [ ] Uses `newBrowser(t)` for browser context
- [ ] Uses `serverURL()` for server URL (never hardcoded)
- [ ] Uses `loginAndNavigate()` for authenticated pages
- [ ] Selectors match current HTML templates in `pages/`
- [ ] Both success and error paths tested
- [ ] Proper cleanup via `defer cancel()`
- [ ] Screenshots on failure where appropriate
- [ ] Module path is `github.com/bobmcallan/vire-portal`
- [ ] Executable via `go test` without Claude
- [ ] Uses helpers from `ui_helpers_test.go`
