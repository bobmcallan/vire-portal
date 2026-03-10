---
name: vire-portal-test-execute
description: Run vire-portal UI tests and report results. Use when asked to execute, run, or check tests. Read-only -- never modifies test files.
---

# /vire-portal-test-execute - Test Execution

Run portal UI tests and report results.

**Mandatory rules are defined in `.claude/skills/vire-portal-test-common/SKILL.md`. Read them first.**

**CRITICAL: This skill MUST NEVER modify or update test files. It is read-only.**

## Usage
```
/vire-portal-test-execute [scope]
```

**Examples:**
- `/vire-portal-test-execute` - Run smoke tests (default)
- `/vire-portal-test-execute smoke` - Run smoke tests
- `/vire-portal-test-execute dashboard` - Run dashboard tests
- `/vire-portal-test-execute stock` - Run stock page tests
- `/vire-portal-test-execute nav` - Run navigation tests
- `/vire-portal-test-execute auth` - Run auth tests
- `/vire-portal-test-execute devauth` - Run dev auth tests
- `/vire-portal-test-execute mcp` - Run MCP tests
- `/vire-portal-test-execute profile` - Run profile tests
- `/vire-portal-test-execute all` - Run all UI tests
- `/vire-portal-test-execute TestSmokeLanding` - Run a specific test by name

## Workflow

### Step 1: Validate Test Structure (Mandatory)

Before executing any tests, validate structural compliance. Check each test file in scope against the mandatory rules from `.claude/skills/vire-portal-test-common/SKILL.md`:

| # | Rule | What to Check |
|---|------|---------------|
| 1 | Independent of Claude | No Claude/AI imports or runtime dependencies |
| 2 | Common browser setup | Uses `newBrowser(t)`, `loginAndNavigate()`, helpers from `ui_helpers_test.go` |
| 3 | Correct selectors | CSS selectors match current HTML templates in `pages/` |
| 4 | Standard Go patterns | Uses `t.Fatal()`, `t.Error()`, `t.Skip()`, `t.Logf()` correctly |
| 5 | **JS error checking** | For pages with Alpine data fetching: at least one test uses `newJSErrorCollector` + assert no errors (Rule 8 from test-common) |

**If non-compliant files are found:**
1. Document each violation in the output report
2. Advise the user to run `/vire-portal-test-create review` to fix
3. Still execute the tests (non-compliance does not block execution)
4. **DO NOT modify the test files**

**After execution -- check test output for JS console warnings:**
Even if all tests pass, scan the test log for lines containing `Alpine Warning` or `Uncaught`. These indicate runtime component bugs that tests didn't catch because the JS error collector wasn't set up. Report them as non-compliance (Rule 8) alongside the pass/fail results.

### Step 2: Determine Test Scope

Parse the argument to determine what to run:

| Argument | Script Command | Test Pattern |
|----------|---------------|--------------|
| *(none)* or `smoke` | `./scripts/ui-test.sh smoke` | `^TestSmoke` |
| `dashboard` | `./scripts/ui-test.sh dashboard` | `^TestDashboard` |
| `stock` | `./scripts/ui-test.sh stock` | `^TestStock` |
| `nav` | `./scripts/ui-test.sh nav` | `^TestNav` |
| `auth` | `./scripts/ui-test.sh auth` | `^TestAuth` |
| `devauth` | `./scripts/ui-test.sh devauth` | `^TestDevAuth` |
| `mcp` | `./scripts/ui-test.sh mcp` | `^TestMcp` |
| `profile` | `./scripts/ui-test.sh profile` | `^TestProfile` |
| `all` | `./scripts/ui-test.sh all` | `.` (all tests) |
| `TestName` | *(see below)* | Specific test |

**Running a specific test by name:** When the argument starts with `Test`, run it directly:

```bash
go test -v ./tests/ui/... -run TestName -timeout 120s
```

### Step 3: Execute Tests

**CRITICAL: Always use the wrapper script for suite execution.** Never run `go test` directly for suites -- the wrapper captures output, generates summary, and collects artifacts.

```bash
# Run specific suite
./scripts/ui-test.sh dashboard

# Run all suites
./scripts/ui-test.sh all

# Available suites: smoke, dashboard, stock, nav, auth, devauth, mcp, profile, all
```

### Step 4: Read and Report Results

**MANDATORY: After execution, read the results and report them.**

```bash
# Find latest results
LATEST=$(ls -td tests/logs/*/ | head -1)

# Read summary
cat "$LATEST/summary.md"

# List all artifacts
ls -la "$LATEST"
```

The summary and log contents MUST be included in the completion report. Do not just say "tests passed" -- show the actual results.

### Step 5: Handle Failures

If tests fail:
1. Read `{suite}.log` for failure details
2. Check `*.png` screenshots for visual context
3. Report the failures with details
4. **DO NOT modify test files** -- advise using `/vire-portal-test-create` if tests need updating

## Notes

- Tests always start Docker containers via `TestMain` (no manual/local mode)
- Latest `vire-server:latest` image is always pulled from GHCR
- First run builds `vire-portal:test` Docker image (may be slow)
- Container logs saved to `tests/logs/{timestamp}/container.log`
- Results always saved to `tests/logs/{timestamp}/`
