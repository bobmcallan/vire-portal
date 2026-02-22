# /test-execute - Test Execution

Run portal UI tests and report results.

**Mandatory rules are defined in `/test-common`. Read them first.**

**CRITICAL: This skill MUST NEVER modify or update test files. It is read-only.**

## Usage
```
/test-execute [scope]
```

**Examples:**
- `/test-execute` - Run smoke tests (default)
- `/test-execute smoke` - Run smoke tests
- `/test-execute dashboard` - Run dashboard tests
- `/test-execute nav` - Run navigation tests
- `/test-execute auth` - Run auth tests
- `/test-execute devauth` - Run dev auth tests
- `/test-execute mcp` - Run MCP tests
- `/test-execute all` - Run all UI tests
- `/test-execute TestSmokeLanding` - Run a specific test by name

## Workflow

### Step 1: Validate Test Structure (Mandatory)

Before executing any tests, validate structural compliance. Check each test file in scope against the mandatory rules from `/test-common`:

| # | Rule | What to Check |
|---|------|---------------|
| 1 | Independent of Claude | No Claude/AI imports or runtime dependencies |
| 2 | Common browser setup | Uses `newBrowser(t)`, `loginAndNavigate()`, helpers from `ui_helpers_test.go` |
| 3 | Correct selectors | CSS selectors match current HTML templates in `pages/` |
| 4 | Standard Go patterns | Uses `t.Fatal()`, `t.Error()`, `t.Skip()`, `t.Logf()` correctly |

**If non-compliant files are found:**
1. Document each violation in the output report
2. Advise the user to run `/test-create-review review` to fix
3. Still execute the tests (non-compliance does not block execution)
4. **DO NOT modify the test files**

### Step 2: Determine Test Scope

Parse the argument to determine what to run:

| Argument | Script Command | Test Pattern |
|----------|---------------|--------------|
| *(none)* or `smoke` | `./scripts/ui-test.sh smoke` | `^TestSmoke` |
| `dashboard` | `./scripts/ui-test.sh dashboard` | `^TestDashboard` |
| `nav` | `./scripts/ui-test.sh nav` | `^TestNav` |
| `auth` | `./scripts/ui-test.sh auth` | `^TestAuth` |
| `devauth` | `./scripts/ui-test.sh devauth` | `^TestDevAuth` |
| `mcp` | `./scripts/ui-test.sh mcp` | `^TestMcp` |
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

# Available suites: smoke, dashboard, nav, auth, devauth, mcp, all
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
4. **DO NOT modify test files** -- advise using `/test-create-review` if tests need updating

## Notes

- Docker mode (default): container starts automatically via `TestMain`
- Manual mode: set `VIRE_TEST_URL` to skip container startup
- First run builds `vire-portal:test` Docker image (may be slow)
- Container logs saved to `tests/logs/{timestamp}/container.log`
- Results always saved to `tests/logs/{timestamp}/`
