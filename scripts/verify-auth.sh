#!/usr/bin/env bash
set -euo pipefail

# verify-auth.sh — Manual validation of auth endpoints on a running vire-portal.
# Usage: ./scripts/verify-auth.sh
# Exit 0 if all checks pass, 1 if any fail.

PORT="${PORTAL_PORT:-8881}"
BASE="http://localhost:${PORT}"
PASS=0
FAIL=0

pass() {
    echo "  PASS: $1"
    PASS=$((PASS + 1))
}

fail() {
    echo "  FAIL: $1"
    FAIL=$((FAIL + 1))
}

echo "=== vire-portal auth verification ==="
echo "Target: ${BASE}"
echo ""

# 1. Portal health
echo "[1] Portal health"
if curl -sf "${BASE}/api/health" > /dev/null 2>&1; then
    pass "GET /api/health returned 200"
else
    fail "GET /api/health did not return 200"
fi

# 2. Vire-server health (proxied)
echo "[2] Vire-server health (proxied)"
if curl -sf "${BASE}/api/server-health" > /dev/null 2>&1; then
    pass "GET /api/server-health returned 200"
else
    fail "GET /api/server-health did not return 200 (vire-server may not be running)"
fi

# 3. Login flow — POST /api/auth/login
echo "[3] Login flow"
LOGIN_RESP=$(curl -s -o /dev/null -w "%{http_code}:%{redirect_url}" \
    -X POST \
    -d "username=dev_user&password=dev123" \
    -H "Content-Type: application/x-www-form-urlencoded" \
    "${BASE}/api/auth/login" 2>&1 || true)
LOGIN_CODE="${LOGIN_RESP%%:*}"
LOGIN_REDIRECT="${LOGIN_RESP#*:}"
if [ "$LOGIN_CODE" = "302" ]; then
    pass "POST /api/auth/login returned 302"
else
    fail "POST /api/auth/login returned ${LOGIN_CODE} (expected 302)"
fi

# Try to capture the session cookie
SESSION_COOKIE=$(curl -s -c - \
    -X POST \
    -d "username=dev_user&password=dev123" \
    -H "Content-Type: application/x-www-form-urlencoded" \
    "${BASE}/api/auth/login" 2>/dev/null | grep vire_session | awk '{print $NF}' || true)
if [ -n "$SESSION_COOKIE" ] && [ "$SESSION_COOKIE" != "" ]; then
    pass "Login set vire_session cookie"
else
    fail "Login did not set vire_session cookie (vire-server may not be running)"
fi

# 4. OAuth redirect — GET /api/auth/login/google
echo "[4] OAuth redirect (Google)"
GOOGLE_RESP=$(curl -s -o /dev/null -w "%{http_code}:%{redirect_url}" \
    "${BASE}/api/auth/login/google" 2>&1 || true)
GOOGLE_CODE="${GOOGLE_RESP%%:*}"
GOOGLE_REDIRECT="${GOOGLE_RESP#*:}"
if [ "$GOOGLE_CODE" = "302" ]; then
    pass "GET /api/auth/login/google returned 302"
else
    fail "GET /api/auth/login/google returned ${GOOGLE_CODE} (expected 302)"
fi
if echo "$GOOGLE_REDIRECT" | grep -q "/api/auth/login/google"; then
    pass "Google redirect points to vire-server OAuth endpoint"
else
    fail "Google redirect URL unexpected: ${GOOGLE_REDIRECT}"
fi

# 5. OAuth callback — GET /auth/callback?token=<test-jwt>
echo "[5] OAuth callback"
# Build a minimal test JWT (alg:none, valid structure)
HEADER=$(echo -n '{"alg":"none","typ":"JWT"}' | base64 -w0 | tr '+/' '-_' | tr -d '=')
CLAIMS=$(echo -n "{\"sub\":\"test_user\",\"exp\":$(($(date +%s) + 86400))}" | base64 -w0 | tr '+/' '-_' | tr -d '=')
TEST_JWT="${HEADER}.${CLAIMS}."

CALLBACK_RESP=$(curl -s -o /dev/null -w "%{http_code}" \
    "${BASE}/auth/callback?token=${TEST_JWT}" 2>&1 || true)
if [ "$CALLBACK_RESP" = "302" ]; then
    pass "GET /auth/callback?token=... returned 302"
else
    fail "GET /auth/callback?token=... returned ${CALLBACK_RESP} (expected 302)"
fi

CALLBACK_COOKIE=$(curl -s -c - \
    "${BASE}/auth/callback?token=${TEST_JWT}" 2>/dev/null | grep vire_session | awk '{print $NF}' || true)
if [ -n "$CALLBACK_COOKIE" ] && [ "$CALLBACK_COOKIE" != "" ]; then
    pass "Callback set vire_session cookie"
else
    fail "Callback did not set vire_session cookie"
fi

# 6. MCP endpoint — POST /mcp with JSON-RPC initialize
echo "[6] MCP endpoint"
MCP_RESP=$(curl -s -w "\n%{http_code}" \
    -X POST \
    -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"verify-auth","version":"1.0.0"}}}' \
    "${BASE}/mcp" 2>&1 || true)
MCP_CODE=$(echo "$MCP_RESP" | tail -1)
MCP_BODY=$(echo "$MCP_RESP" | head -n -1)
if [ "$MCP_CODE" = "200" ]; then
    pass "POST /mcp returned 200"
else
    fail "POST /mcp returned ${MCP_CODE} (expected 200)"
fi
if echo "$MCP_BODY" | grep -q '"jsonrpc"'; then
    pass "MCP response contains JSON-RPC"
else
    fail "MCP response missing JSON-RPC structure"
fi

# Summary
echo ""
echo "=== Summary ==="
echo "  Passed: ${PASS}"
echo "  Failed: ${FAIL}"
echo "  Total:  $((PASS + FAIL))"

if [ "$FAIL" -gt 0 ]; then
    echo ""
    echo "Some checks failed. Ensure vire-portal and vire-server are running."
    exit 1
fi

echo ""
echo "All checks passed."
exit 0
