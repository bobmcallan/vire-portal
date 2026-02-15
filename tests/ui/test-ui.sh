#!/usr/bin/env bash
# scripts/test-ui.sh
#
# Build portal image → start containers → run chromedp UI tests → tear down.
# Claude Code skill calls this after making portal/CSS/JS/template changes.
#
# Usage:
#   ./scripts/test-ui.sh              # full: build + up + test + down
#   ./scripts/test-ui.sh --no-build   # skip build, containers already running
#   ./scripts/test-ui.sh --no-down    # leave containers running after tests
#
# Requires: docker, docker compose, go, chrome/chromium on host

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
COMPOSE_FILE="$PROJECT_DIR/docker/docker-compose.yml"

PORTAL_PORT="${PORTAL_PORT:-4241}"
PORTAL_URL="http://localhost:$PORTAL_PORT"
MAX_WAIT=60

DO_BUILD=true
DO_DOWN=true

for arg in "$@"; do
    case "$arg" in
        --no-build) DO_BUILD=false ;;
        --no-down)  DO_DOWN=false ;;
    esac
done

cd "$PROJECT_DIR"

cleanup() {
    if [ "$DO_DOWN" = true ]; then
        echo ""
        echo "=== TEAR DOWN ==="
        docker compose -f "$COMPOSE_FILE" down --timeout 10 2>/dev/null || true
    fi
}
trap cleanup EXIT

# ── 1. BUILD ────────────────────────────────────────────────
if [ "$DO_BUILD" = true ]; then
    echo "=== DOCKER BUILD ==="
    if ! docker compose -f "$COMPOSE_FILE" build vire-portal 2>&1; then
        echo "FAIL: docker build failed"
        exit 1
    fi
    echo "OK: image built"
fi

# ── 2. START ────────────────────────────────────────────────
echo ""
echo "=== DOCKER UP ==="
docker compose -f "$COMPOSE_FILE" up -d 2>&1

echo "Waiting for portal on $PORTAL_URL ..."
for i in $(seq 1 $MAX_WAIT); do
    STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$PORTAL_URL/api/health" 2>/dev/null || echo "000")
    if [ "$STATUS" = "200" ]; then
        echo "OK: portal ready (${i}s)"
        break
    fi
    if [ "$i" -eq "$MAX_WAIT" ]; then
        echo "FAIL: portal not healthy after ${MAX_WAIT}s (last status: $STATUS)"
        echo "--- container logs ---"
        docker compose -f "$COMPOSE_FILE" logs vire-portal --tail 30
        exit 1
    fi
    sleep 1
done

# ── 3. RUN TESTS ────────────────────────────────────────────
echo ""
echo "=== UI TESTS ==="
TEST_EXIT=0
VIRE_TEST_URL="$PORTAL_URL" go test ./tests/ -run "^TestUI" -v -count=1 -timeout 120s 2>&1 | tee /tmp/vire-ui-results.txt || TEST_EXIT=$?

# ── 4. SUMMARY ──────────────────────────────────────────────
echo ""
echo "========================================="

PASS=$(grep -c "^--- PASS:" /tmp/vire-ui-results.txt 2>/dev/null || echo 0)
FAIL=$(grep -c "^--- FAIL:" /tmp/vire-ui-results.txt 2>/dev/null || echo 0)
SKIP=$(grep -c "^--- SKIP:" /tmp/vire-ui-results.txt 2>/dev/null || echo 0)

echo "PASS: $PASS  FAIL: $FAIL  SKIP: $SKIP"

if [ "$FAIL" -gt 0 ]; then
    echo ""
    echo "FAILURES:"
    grep -A2 "^--- FAIL:" /tmp/vire-ui-results.txt || true
fi

echo "========================================="
exit "$TEST_EXIT"
