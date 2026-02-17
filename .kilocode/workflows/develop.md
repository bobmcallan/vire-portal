# Develop Workflow

Iteratively develop, test, and fix UI issues. This workflow runs browser tests, analyzes failures, and implements fixes until all tests pass.

## Usage

```
/develop fix the dashboard
/develop check and fix the nav
/develop smoke test and fix issues
/develop all tests must pass
```

## Input Detection

Parse user input to determine test category:
- Contains "dashboard" → run dashboard tests
- Contains "nav" or "navigation" → run nav tests
- Contains "smoke" → run smoke tests
- Contains "all" → run all tests
- Default → run smoke tests

## Workflow

1. **Initial Assessment**
   - Parse the user request to identify the target area
   - Confirm understanding of what needs to be fixed
   - Set up the test output directory

2. **Run Browser Tests**
   ```
   Execute /browser-check workflow for the identified category
   ```

3. **Analyze Results**
   - Read the generated summary.md
   - Identify passed vs failed tests
   - If all tests pass → report success and exit

4. **Assess Failures**
   For each failed test:
   - Read the failure details from the log
   - Review relevant screenshots
   - Identify the root cause:
     - **Code bug**: Fix in templates, CSS, JS, or Go handlers
     - **Test misalignment**: Test expects behavior that doesn't match requirements → STOP and report
     - **Not technically achievable**: Constraint that cannot be met → STOP and report

5. **Implement Fix**
   Use the develop skill to code the fix:
   - Identify affected files (templates, CSS, JS, Go code)
   - Make minimal, targeted changes
   - Ensure changes align with existing code patterns

6. **Re-run Tests**
   - Execute /browser-check again for the same category
   - Compare results with previous run
   - If new failures introduced → assess and fix

7. **Iterate**
   Repeat steps 3-6 until:
   - All tests pass, OR
   - Max iterations reached (5), OR
   - Test misalignment detected (cannot proceed)

## Execution Steps

### Step 1: Setup
```bash
cd /home/bobmc/development/vire-portal
export VIRE_TEST_URL="${VIRE_TEST_URL:-http://localhost:8881}"
ITERATION=0
MAX_ITERATIONS=5
```

### Step 2: Run Tests
Based on input category, execute:
```bash
# For dashboard:
go test -v ./tests/ui -run "^TestDashboard" -timeout 60s 2>&1 | tee tests/results/latest/dashboard.log

# For nav:
go test -v ./tests/ui -run "^TestNav" -timeout 60s 2>&1 | tee tests/results/latest/nav.log

# For smoke:
go test -v ./tests/ui -run "^TestSmoke" -timeout 60s 2>&1 | tee tests/results/latest/smoke.log
```

### Step 3: Check Results
```bash
FAILED=$(grep -c "^--- FAIL:" tests/results/latest/*.log 2>/dev/null || echo 0)
if [ "$FAILED" -eq 0 ]; then
    echo "All tests passed!"
    exit 0
fi
```

### Step 4: Analyze & Fix
For each failure, identify the issue and implement a fix using standard development practices.

### Step 5: Iterate
```bash
ITERATION=$((ITERATION + 1))
if [ $ITERATION -ge $MAX_ITERATIONS ]; then
    echo "Max iterations reached. Manual review required."
    exit 1
fi
# Go back to Step 2
```

## Constraints

1. **Cannot modify test files** - Tests in `tests/ui/*_test.go` are the specification
2. **Can stop if**:
   - Test expectations don't match documented requirements
   - Fix is not technically achievable (e.g., browser limitation)
   - Max iterations exceeded
3. **Must report**:
   - What was fixed in each iteration
   - Why stopped (if stopped early)
   - Final test status

## Output Format

After each iteration, report:
```
## Iteration N

**Tests Run:** [category]
**Passed:** X  **Failed:** Y  **Skipped:** Z

### Failures Fixed
- [test name]: [what was wrong] → [what was changed]

### Remaining Failures
- [test name]: [reason] → [proposed fix]

### Next Steps
[description of next action]
```

## Stop Conditions

Report and stop if:

1. **Test Misalignment**
   ```
   STOP: Test [name] expects [behavior] but requirements specify [different behavior].
   The test should not be modified per workflow constraints.
   Recommendation: [review test or requirements]
   ```

2. **Not Achievable**
   ```
   STOP: [test name] failure cannot be fixed.
   Reason: [technical constraint or limitation]
   Recommendation: [alternative approach]
   ```

3. **Max Iterations**
   ```
   STOP: Maximum iterations (5) reached without all tests passing.
   Remaining failures: [list]
   Recommendation: Manual investigation required.
   ```

## Example Session

```
User: /develop fix the dashboard

Agent: Running dashboard tests...

## Iteration 1

**Tests Run:** Dashboard
**Passed:** 8  **Failed:** 1  **Skipped:** 3

### Failures
- TestDashboardDesignNoBorderRadius: border-radius found on span.status-dot (50%)

### Analysis
The status-dot element has border-radius: 50% for circular indicators.
This is intentional for status indicators.

### Fix Applied
Updated status-dot CSS to use border-radius: 0 (square) to match design rules.

Running tests again...

## Iteration 2

**All tests passed!**

### Summary
- Fixed: status-dot border-radius violation
- Files changed: static/css/styles.css
```
