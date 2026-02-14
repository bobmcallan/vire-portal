#!/bin/bash
set -euo pipefail

# Migration validation script for rename + vire-mcp migration.
# Verifies file structure, builds, tests, and Docker after migration.
# Run from project root: ./tests/validate-migration.sh

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

PASSED=0
FAILED=0
ERRORS=()

pass() {
    PASSED=$((PASSED + 1))
    echo "  PASS: $1"
}

fail() {
    FAILED=$((FAILED + 1))
    ERRORS+=("$1")
    echo "  FAIL: $1"
}

section() {
    echo ""
    echo "=== $1 ==="
}

# ---------------------------------------------------------------------------
# 1. cmd/vire-portal (renamed from cmd/portal)
# ---------------------------------------------------------------------------
section "cmd/vire-portal rename"

if [ -d "$PROJECT_DIR/cmd/vire-portal" ]; then
    pass "cmd/vire-portal/ directory exists"
else
    fail "cmd/vire-portal/ directory missing"
fi

if [ -f "$PROJECT_DIR/cmd/vire-portal/main.go" ]; then
    pass "cmd/vire-portal/main.go exists"
else
    fail "cmd/vire-portal/main.go missing"
fi

if [ ! -d "$PROJECT_DIR/cmd/portal" ]; then
    pass "cmd/portal/ removed (old name)"
else
    fail "cmd/portal/ still exists (should be renamed to cmd/vire-portal)"
fi

# ---------------------------------------------------------------------------
# 2. cmd/vire-mcp (migrated from vire repo)
# ---------------------------------------------------------------------------
section "cmd/vire-mcp migration"

for f in main.go proxy.go handlers.go formatters.go tools.go \
         proxy_test.go handlers_test.go formatters_test.go; do
    if [ -f "$PROJECT_DIR/cmd/vire-mcp/$f" ]; then
        pass "cmd/vire-mcp/$f exists"
    else
        fail "cmd/vire-mcp/$f missing"
    fi
done

# Verify import paths are rewritten (no references to old vire module)
if grep -rq 'github.com/bobmcallan/vire/' "$PROJECT_DIR/cmd/vire-mcp/" 2>/dev/null; then
    fail "cmd/vire-mcp/ still has old import paths (github.com/bobmcallan/vire/)"
else
    pass "cmd/vire-mcp/ import paths rewritten correctly"
fi

# Verify new import paths are present
if grep -rq 'github.com/bobmcallan/vire-portal/internal/vire/common' "$PROJECT_DIR/cmd/vire-mcp/" 2>/dev/null; then
    pass "cmd/vire-mcp/ uses new common import path"
else
    fail "cmd/vire-mcp/ missing new common import path"
fi

if grep -rq 'github.com/bobmcallan/vire-portal/internal/vire/models' "$PROJECT_DIR/cmd/vire-mcp/" 2>/dev/null; then
    pass "cmd/vire-mcp/ uses new models import path"
else
    fail "cmd/vire-mcp/ missing new models import path"
fi

# ---------------------------------------------------------------------------
# 3. internal/vire/ packages
# ---------------------------------------------------------------------------
section "internal/vire/ packages"

for pkg in common models interfaces; do
    if [ -d "$PROJECT_DIR/internal/vire/$pkg" ]; then
        pass "internal/vire/$pkg/ directory exists"
    else
        fail "internal/vire/$pkg/ directory missing"
    fi

    # Check at least one .go file exists
    if ls "$PROJECT_DIR/internal/vire/$pkg/"*.go >/dev/null 2>&1; then
        pass "internal/vire/$pkg/ has .go files"
    else
        fail "internal/vire/$pkg/ has no .go files"
    fi
done

# Verify import paths are rewritten in internal/vire/
for pkg_dir in common models interfaces; do
    if grep -rq 'github.com/bobmcallan/vire/' "$PROJECT_DIR/internal/vire/$pkg_dir/" 2>/dev/null; then
        fail "internal/vire/$pkg_dir/ still has old import paths"
    else
        pass "internal/vire/$pkg_dir/ import paths rewritten correctly"
    fi
done

# Verify source provenance comments
for pkg_dir in common models interfaces; do
    first_go=$(ls "$PROJECT_DIR/internal/vire/$pkg_dir/"*.go 2>/dev/null | grep -v _test.go | head -1)
    if [ -n "$first_go" ] && grep -q 'Copied from github.com/bobmcallan/vire' "$first_go"; then
        pass "internal/vire/$pkg_dir/ has source provenance comment"
    else
        fail "internal/vire/$pkg_dir/ missing source provenance comment"
    fi
done

# ---------------------------------------------------------------------------
# 4. vire-mcp config does NOT use viper (eliminated per DA challenge)
# ---------------------------------------------------------------------------
section "viper elimination"

if grep -rq 'github.com/spf13/viper' "$PROJECT_DIR/cmd/vire-mcp/" 2>/dev/null; then
    fail "cmd/vire-mcp/ still imports spf13/viper (should be eliminated)"
else
    pass "cmd/vire-mcp/ does not import spf13/viper"
fi

if grep -rq 'mapstructure:' "$PROJECT_DIR/cmd/vire-mcp/" 2>/dev/null; then
    fail "cmd/vire-mcp/ still has mapstructure tags (should be toml tags)"
else
    pass "cmd/vire-mcp/ uses toml tags (not mapstructure)"
fi

# ---------------------------------------------------------------------------
# 5. Go builds
# ---------------------------------------------------------------------------
section "Go compilation"

if command -v go &>/dev/null; then
    if (cd "$PROJECT_DIR" && go build ./cmd/vire-portal/) 2>/dev/null; then
        pass "go build ./cmd/vire-portal/ succeeds"
    else
        fail "go build ./cmd/vire-portal/ fails"
    fi

    if (cd "$PROJECT_DIR" && go build ./cmd/vire-mcp/) 2>/dev/null; then
        pass "go build ./cmd/vire-mcp/ succeeds"
    else
        fail "go build ./cmd/vire-mcp/ fails"
    fi

    if (cd "$PROJECT_DIR" && go vet ./...) 2>/dev/null; then
        pass "go vet ./... passes"
    else
        fail "go vet ./... reports issues"
    fi

    if (cd "$PROJECT_DIR" && go test ./... -count=1) 2>&1 | tail -5; then
        pass "go test ./... passes"
    else
        fail "go test ./... fails"
    fi
else
    echo "  SKIP: go not available"
fi

# ---------------------------------------------------------------------------
# 6. Docker files
# ---------------------------------------------------------------------------
section "Docker files"

if [ -f "$PROJECT_DIR/docker/Dockerfile.mcp" ]; then
    pass "docker/Dockerfile.mcp exists"
else
    fail "docker/Dockerfile.mcp missing"
fi

if [ -f "$PROJECT_DIR/docker/vire-mcp.toml" ]; then
    pass "docker/vire-mcp.toml exists"
else
    fail "docker/vire-mcp.toml missing"
fi

if [ -f "$PROJECT_DIR/docker/vire-mcp.toml.docker" ]; then
    pass "docker/vire-mcp.toml.docker exists"
else
    fail "docker/vire-mcp.toml.docker missing"
fi

# Verify Dockerfile.mcp builds cmd/vire-mcp
if [ -f "$PROJECT_DIR/docker/Dockerfile.mcp" ]; then
    if grep -q 'cmd/vire-mcp' "$PROJECT_DIR/docker/Dockerfile.mcp"; then
        pass "Dockerfile.mcp builds cmd/vire-mcp"
    else
        fail "Dockerfile.mcp does not reference cmd/vire-mcp"
    fi

    if grep -q 'internal/vire/common' "$PROJECT_DIR/docker/Dockerfile.mcp"; then
        pass "Dockerfile.mcp ldflags use internal/vire/common path"
    else
        fail "Dockerfile.mcp ldflags missing internal/vire/common path"
    fi

    if grep -q 'alpine:3.21' "$PROJECT_DIR/docker/Dockerfile.mcp"; then
        pass "Dockerfile.mcp uses alpine:3.21 (matches portal)"
    else
        fail "Dockerfile.mcp not using alpine:3.21"
    fi

    for arg in VERSION BUILD GIT_COMMIT; do
        if grep -q "ARG $arg" "$PROJECT_DIR/docker/Dockerfile.mcp"; then
            pass "Dockerfile.mcp declares ARG $arg"
        else
            fail "Dockerfile.mcp missing ARG $arg"
        fi
    done

    # Verify .docker config is used in Dockerfile (not the one with API keys)
    if grep -q 'vire-mcp.toml.docker' "$PROJECT_DIR/docker/Dockerfile.mcp"; then
        pass "Dockerfile.mcp copies vire-mcp.toml.docker (not sensitive config)"
    else
        fail "Dockerfile.mcp should copy vire-mcp.toml.docker"
    fi
fi

# Verify portal Dockerfile updated for rename
if [ -f "$PROJECT_DIR/docker/Dockerfile" ]; then
    if grep -q 'cmd/vire-portal' "$PROJECT_DIR/docker/Dockerfile"; then
        pass "Dockerfile builds cmd/vire-portal (renamed)"
    else
        fail "Dockerfile still references cmd/portal (not renamed)"
    fi
fi

# ---------------------------------------------------------------------------
# 7. docker-compose.yml has vire-mcp service
# ---------------------------------------------------------------------------
section "docker-compose.yml vire-mcp service"

COMPOSE="$PROJECT_DIR/docker/docker-compose.yml"
if [ -f "$COMPOSE" ]; then
    if grep -q 'vire-mcp:' "$COMPOSE"; then
        pass "docker-compose.yml has vire-mcp service"
    else
        fail "docker-compose.yml missing vire-mcp service"
    fi

    if grep -q 'Dockerfile.mcp' "$COMPOSE"; then
        pass "docker-compose.yml references Dockerfile.mcp"
    else
        fail "docker-compose.yml missing Dockerfile.mcp reference"
    fi

    if grep -q '4243' "$COMPOSE"; then
        pass "docker-compose.yml maps port 4243"
    else
        fail "docker-compose.yml missing port 4243"
    fi
fi

# ---------------------------------------------------------------------------
# 8. docker-compose.ghcr.yml has vire-mcp service
# ---------------------------------------------------------------------------
section "docker-compose.ghcr.yml vire-mcp service"

GHCR_COMPOSE="$PROJECT_DIR/docker/docker-compose.ghcr.yml"
if [ -f "$GHCR_COMPOSE" ]; then
    if grep -q 'vire-mcp:' "$GHCR_COMPOSE"; then
        pass "docker-compose.ghcr.yml has vire-mcp service"
    else
        fail "docker-compose.ghcr.yml missing vire-mcp service"
    fi

    if grep -q 'ghcr.io/bobmcallan/vire-mcp' "$GHCR_COMPOSE"; then
        pass "docker-compose.ghcr.yml uses GHCR image for vire-mcp"
    else
        fail "docker-compose.ghcr.yml missing GHCR image for vire-mcp"
    fi

    if grep -q 'com.centurylinklabs.watchtower.scope=vire-portal' "$GHCR_COMPOSE"; then
        pass "docker-compose.ghcr.yml has watchtower scope for vire-mcp"
    else
        fail "docker-compose.ghcr.yml missing watchtower scope for vire-mcp"
    fi
fi

# ---------------------------------------------------------------------------
# 9. .gitignore updated
# ---------------------------------------------------------------------------
section ".gitignore"

GITIGNORE="$PROJECT_DIR/.gitignore"
if [ -f "$GITIGNORE" ]; then
    if grep -q '/vire-portal' "$GITIGNORE"; then
        pass ".gitignore has /vire-portal binary entry"
    else
        fail ".gitignore missing /vire-portal binary entry"
    fi

    if grep -q '/vire-mcp' "$GITIGNORE"; then
        pass ".gitignore has /vire-mcp binary entry"
    else
        fail ".gitignore missing /vire-mcp binary entry"
    fi

    if grep -q '/portal' "$GITIGNORE" && ! grep -q '/vire-portal' "$GITIGNORE"; then
        fail ".gitignore still has old /portal entry"
    else
        pass ".gitignore /portal entry updated or removed"
    fi
fi

# ---------------------------------------------------------------------------
# 10. Scripts updated
# ---------------------------------------------------------------------------
section "Scripts"

if [ -f "$PROJECT_DIR/scripts/test-scripts.sh" ]; then
    if grep -q 'cmd/vire-portal' "$PROJECT_DIR/scripts/test-scripts.sh"; then
        pass "test-scripts.sh references cmd/vire-portal"
    else
        fail "test-scripts.sh missing cmd/vire-portal reference"
    fi

    if grep -q 'cmd/vire-mcp' "$PROJECT_DIR/scripts/test-scripts.sh"; then
        pass "test-scripts.sh references cmd/vire-mcp"
    else
        fail "test-scripts.sh missing cmd/vire-mcp reference"
    fi

    if grep -q 'Dockerfile.mcp' "$PROJECT_DIR/scripts/test-scripts.sh"; then
        pass "test-scripts.sh checks Dockerfile.mcp"
    else
        fail "test-scripts.sh missing Dockerfile.mcp check"
    fi
fi

if [ -f "$PROJECT_DIR/scripts/build.sh" ]; then
    if grep -q 'vire-mcp' "$PROJECT_DIR/scripts/build.sh"; then
        pass "build.sh supports vire-mcp"
    else
        fail "build.sh missing vire-mcp support"
    fi
fi

# ---------------------------------------------------------------------------
# 11. Docker builds (if docker available)
# ---------------------------------------------------------------------------
section "Docker builds"

if command -v docker &>/dev/null; then
    if docker build -f "$PROJECT_DIR/docker/Dockerfile" -t vire-portal:test "$PROJECT_DIR" >/dev/null 2>&1; then
        pass "docker build vire-portal succeeds"
        docker image rm vire-portal:test >/dev/null 2>&1 || true
    else
        fail "docker build vire-portal fails"
    fi

    if docker build -f "$PROJECT_DIR/docker/Dockerfile.mcp" -t vire-mcp:test "$PROJECT_DIR" >/dev/null 2>&1; then
        pass "docker build vire-mcp succeeds"
        docker image rm vire-mcp:test >/dev/null 2>&1 || true
    else
        fail "docker build vire-mcp fails"
    fi
else
    echo "  SKIP: docker not available"
fi

# ---------------------------------------------------------------------------
# 12. Go version
# ---------------------------------------------------------------------------
section "Go version"

if [ -f "$PROJECT_DIR/go.mod" ]; then
    if grep -q 'go 1.25' "$PROJECT_DIR/go.mod"; then
        pass "go.mod specifies Go 1.25"
    else
        fail "go.mod Go version unexpected"
    fi
fi

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
echo ""
echo "==========================================="
echo "Migration Validation: $PASSED passed, $FAILED failed"
echo "==========================================="

if [ ${#ERRORS[@]} -gt 0 ]; then
    echo ""
    echo "Failures:"
    for err in "${ERRORS[@]}"; do
        echo "  - $err"
    done
fi

if [ "$FAILED" -gt 0 ]; then
    exit 1
else
    exit 0
fi
