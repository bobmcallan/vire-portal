#!/bin/bash
# UI Test Runner - Executes UI tests with full artifact collection
#
# Usage:
#   ./scripts/ui-test.sh [suite]     # Run specific suite (smoke, dashboard, nav, auth)
#   ./scripts/ui-test.sh all         # Run all tests
#
# Artifacts are saved to tests/results/{timestamp}/:
#   - {suite}.log      # Full test output
#   - summary.md       # Pass/fail summary with screenshots list
#   - *.png            # Screenshots from failures

set -e

# Configuration
RESULTS_DIR="tests/results"
TIMEOUT=120
CONFIG_FILE="tests/ui/test_config.toml"

# Read server URL from test config, fall back to env var, then default
if [ -n "$VIRE_TEST_URL" ]; then
    SERVER_URL="$VIRE_TEST_URL"
elif [ -f "$CONFIG_FILE" ]; then
    SERVER_URL=$(grep -E '^url\s*=' "$CONFIG_FILE" | sed 's/.*=\s*"\(.*\)"/\1/' | head -1)
fi
SERVER_URL="${SERVER_URL:-http://localhost:8883}"

# Determine test pattern
CATEGORY="${1:-smoke}"

if [ "$CATEGORY" = "all" ]; then
    PATTERN="."
    SUITE_NAME="all"
else
    # Map common aliases to actual test prefixes
    case "$CATEGORY" in
        devauth|dev-auth|dev_auth)
            PATTERN="^TestDevAuth"
            SUITE_NAME="devauth"
            ;;
        *)
            # Capitalize first letter (smoke -> Smoke)
            TITLE_CASE="$(tr '[:lower:]' '[:upper:]' <<< ${CATEGORY:0:1})${CATEGORY:1}"
            PATTERN="^Test${TITLE_CASE}"
            SUITE_NAME="$CATEGORY"
            ;;
    esac
fi

# Create timestamped results directory
TIMESTAMP=$(date +"%Y-%m-%d-%H-%M-%S")
RESULT_DIR="${RESULTS_DIR}/${TIMESTAMP}"
mkdir -p "$RESULT_DIR"

# Export ABSOLUTE path for tests to use
export VIRE_TEST_RESULTS_DIR="$(cd "$RESULT_DIR" && pwd)"

LOG_FILE="${RESULT_DIR}/${SUITE_NAME}.log"

echo "========================================"
echo "UI Test Runner"
echo "========================================"
echo "Suite:     $SUITE_NAME"
echo "Pattern:   $PATTERN"
echo "Results:   $RESULT_DIR"
echo "Log:       $LOG_FILE"
echo "========================================"
echo ""

# Check server health
echo "Checking server health..."
if ! curl -sf "${SERVER_URL}/api/health" > /dev/null 2>&1; then
    echo "ERROR: Server not responding at ${SERVER_URL}"
    echo "Start the server with: ./scripts/run.sh restart"
    exit 1
fi
echo "Server: OK"
echo ""

# Run tests, capturing output to log file
echo "Running tests..."
echo ""

# Use tee to capture output while still showing it
set +e
go test -v ./tests/ui -run "$PATTERN" -timeout "${TIMEOUT}s" -count=1 2>&1 | tee "$LOG_FILE"
TEST_EXIT_CODE=${PIPESTATUS[0]}
set -e

echo ""
echo "========================================"

# Count results by parsing the log file
PASSED=0
FAILED=0
SKIPPED=0

while IFS= read -r line; do
    case "$line" in
        "--- PASS:"*) PASSED=$((PASSED + 1)) ;;
        "--- FAIL:"*) FAILED=$((FAILED + 1)) ;;
        "--- SKIP:"*) SKIPPED=$((SKIPPED + 1)) ;;
    esac
done < "$LOG_FILE"

# Determine status
if [ "$FAILED" -eq 0 ]; then
    STATUS="✅ PASS"
else
    STATUS="❌ FAIL"
fi

# Count screenshots (including subdirectories)
SCREENSHOT_COUNT=$(find "$RESULT_DIR" -name "*.png" -type f 2>/dev/null | wc -l)

# Write summary.md
SUMMARY_FILE="${RESULT_DIR}/summary.md"
cat > "$SUMMARY_FILE" << EOF
# Test Summary: ${SUITE_NAME}

**Status:** ${STATUS}
**Timestamp:** $(date +"%Y-%m-%d %H:%M:%S")
**Server:** ${SERVER_URL}

## Results

| Metric | Count |
|--------|-------|
| Passed | ${PASSED} |
| Failed | ${FAILED} |
| Skipped | ${SKIPPED} |

## Artifacts

- Log: \`${SUITE_NAME}.log\`
EOF

# Add screenshots if any (including subdirectories)
if [ "$SCREENSHOT_COUNT" -gt 0 ]; then
    echo "" >> "$SUMMARY_FILE"
    echo "### Screenshots (${SCREENSHOT_COUNT})" >> "$SUMMARY_FILE"
    echo "" >> "$SUMMARY_FILE"
    find "$RESULT_DIR" -name "*.png" -type f 2>/dev/null | sort | while read img; do
        # Get relative path from RESULT_DIR
        rel_path="${img#$RESULT_DIR/}"
        echo "- \`$rel_path\`" >> "$SUMMARY_FILE"
    done
fi

# Add failure details if any
if [ "$FAILED" -gt 0 ]; then
    echo "" >> "$SUMMARY_FILE"
    echo "## Failures" >> "$SUMMARY_FILE"
    echo "" >> "$SUMMARY_FILE"
    grep -B1 -A5 "^--- FAIL:" "$LOG_FILE" 2>/dev/null | head -50 >> "$SUMMARY_FILE" || echo "See log for details." >> "$SUMMARY_FILE"
fi

# Print summary
echo "Summary:"
echo "  Status:     $STATUS"
echo "  Passed:     $PASSED"
echo "  Failed:     $FAILED"
echo "  Skipped:    $SKIPPED"
echo "  Log:        $LOG_FILE"
echo "  Summary:    $SUMMARY_FILE"

if [ "$SCREENSHOT_COUNT" -gt 0 ]; then
    echo "  Screenshots: $SCREENSHOT_COUNT files"
fi

echo ""

# Exit with test result
exit $TEST_EXIT_CODE
