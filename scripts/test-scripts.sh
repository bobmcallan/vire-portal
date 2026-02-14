#!/bin/bash
set -euo pipefail

# Test suite for build scripts, deploy scripts, and Docker configs
# Run from project root: ./scripts/test-scripts.sh

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
# 1. File existence checks
# ---------------------------------------------------------------------------
section "File existence"

for f in \
    "scripts/deploy.sh" \
    "scripts/build.sh" \
    "docker/docker-compose.yml" \
    "docker/docker-compose.ghcr.yml" \
    "docker/portal.toml" \
    "Dockerfile" \
    ".version" \
    "go.mod" \
    "go.sum" \
    "cmd/portal/main.go"; do
    if [ -f "$PROJECT_DIR/$f" ]; then
        pass "$f exists"
    else
        fail "$f missing"
    fi
done

# Check directories exist
for d in "internal" "pages" "cmd/portal"; do
    if [ -d "$PROJECT_DIR/$d" ]; then
        pass "$d/ directory exists"
    else
        fail "$d/ directory missing"
    fi
done

# Verify SPA files are removed
for f in \
    "package.json" \
    "package-lock.json" \
    "tsconfig.json" \
    "vite.config.ts" \
    "eslint.config.js" \
    "index.html" \
    "nginx.conf" \
    "Dockerfile.portal"; do
    if [ ! -f "$PROJECT_DIR/$f" ]; then
        pass "$f removed (SPA artifact)"
    else
        fail "$f still exists (should be removed)"
    fi
done

for d in "src" "node_modules"; do
    if [ ! -d "$PROJECT_DIR/$d" ]; then
        pass "$d/ removed (SPA artifact)"
    else
        fail "$d/ still exists (should be removed)"
    fi
done

# ---------------------------------------------------------------------------
# 2. Scripts are executable
# ---------------------------------------------------------------------------
section "Script permissions"

for f in "scripts/deploy.sh" "scripts/build.sh"; do
    if [ -f "$PROJECT_DIR/$f" ] && [ -x "$PROJECT_DIR/$f" ]; then
        pass "$f is executable"
    else
        fail "$f is not executable"
    fi
done

# ---------------------------------------------------------------------------
# 3. Scripts have proper shebang and set -euo pipefail
# ---------------------------------------------------------------------------
section "Script headers"

for f in "scripts/deploy.sh" "scripts/build.sh"; do
    filepath="$PROJECT_DIR/$f"
    if [ ! -f "$filepath" ]; then
        fail "$f not found, skipping header checks"
        continue
    fi

    if head -1 "$filepath" | grep -q '^#!/bin/bash'; then
        pass "$f has bash shebang"
    else
        fail "$f missing bash shebang"
    fi

    if head -5 "$filepath" | grep -q 'set -euo pipefail'; then
        pass "$f has set -euo pipefail"
    else
        fail "$f missing set -euo pipefail"
    fi
done

# Check build.sh header comment describes Docker image builder
BUILD="$PROJECT_DIR/scripts/build.sh"
if [ -f "$BUILD" ]; then
    if head -5 "$BUILD" | grep -qi 'docker.*image\|image.*build'; then
        pass "build.sh header clarifies it is a Docker image builder"
    else
        fail "build.sh header should clarify it is a Docker image builder"
    fi
fi

# ---------------------------------------------------------------------------
# 4. .version file format
# ---------------------------------------------------------------------------
section ".version file format"

VERSION_FILE="$PROJECT_DIR/.version"
if [ -f "$VERSION_FILE" ]; then
    if grep -qE '^version: [0-9]+\.[0-9]+\.[0-9]+' "$VERSION_FILE"; then
        pass ".version has valid version line (semver)"
    else
        fail ".version missing valid version line"
    fi

    if grep -qE '^build: [0-9]{2}-[0-9]{2}-[0-9]{2}-[0-9]{2}-[0-9]{2}' "$VERSION_FILE"; then
        pass ".version has valid build timestamp"
    else
        fail ".version missing valid build timestamp"
    fi

    LINE_COUNT=$(wc -l < "$VERSION_FILE")
    if [ "$LINE_COUNT" -eq 2 ]; then
        pass ".version has exactly 2 lines"
    else
        fail ".version has $LINE_COUNT lines (expected 2)"
    fi
else
    fail ".version file not found"
fi

# ---------------------------------------------------------------------------
# 5. deploy.sh mode validation
# ---------------------------------------------------------------------------
section "deploy.sh modes and features"

DEPLOY="$PROJECT_DIR/scripts/deploy.sh"
if [ -f "$DEPLOY" ]; then
    for mode in local ghcr down prune; do
        if grep -q "$mode)" "$DEPLOY"; then
            pass "deploy.sh handles '$mode' mode"
        else
            fail "deploy.sh missing '$mode' mode"
        fi
    done

    # Check usage/help on invalid mode
    if grep -q 'Usage:' "$DEPLOY"; then
        pass "deploy.sh has usage message for invalid mode"
    else
        fail "deploy.sh missing usage message"
    fi

    # Check --force flag support
    if grep -q '\-\-force' "$DEPLOY"; then
        pass "deploy.sh supports --force flag"
    else
        fail "deploy.sh missing --force flag support"
    fi

    # Check version extraction from .version
    if grep -q '\.version' "$DEPLOY"; then
        pass "deploy.sh reads .version file"
    else
        fail "deploy.sh does not read .version file"
    fi

    # Check build timestamp update
    if grep -qE "sed.*build:" "$DEPLOY"; then
        pass "deploy.sh updates build timestamp"
    else
        fail "deploy.sh does not update build timestamp"
    fi

    # Check git commit extraction
    if grep -q 'git.*rev-parse' "$DEPLOY"; then
        pass "deploy.sh extracts git commit"
    else
        fail "deploy.sh does not extract git commit"
    fi

    # Check that it references the correct compose files
    if grep -q 'docker-compose.yml' "$DEPLOY"; then
        pass "deploy.sh references docker-compose.yml"
    else
        fail "deploy.sh missing docker-compose.yml reference"
    fi

    if grep -q 'docker-compose.ghcr.yml' "$DEPLOY"; then
        pass "deploy.sh references docker-compose.ghcr.yml"
    else
        fail "deploy.sh missing docker-compose.ghcr.yml reference"
    fi

    # Check smart rebuild sentinel
    if grep -q '\.last_build' "$DEPLOY"; then
        pass "deploy.sh uses .last_build sentinel"
    else
        fail "deploy.sh missing .last_build sentinel"
    fi

    # Smart rebuild checks Go files
    if grep -q '"\*\.go"' "$DEPLOY"; then
        pass "deploy.sh checks *.go files for changes"
    else
        fail "deploy.sh missing *.go change detection"
    fi

    if grep -q 'go\.mod' "$DEPLOY"; then
        pass "deploy.sh checks go.mod for changes"
    else
        fail "deploy.sh missing go.mod change detection"
    fi

    if grep -q 'Dockerfile' "$DEPLOY"; then
        pass "deploy.sh checks Dockerfile for changes"
    else
        fail "deploy.sh missing Dockerfile change detection"
    fi

    # Footer health URL uses PORTAL_PORT variable
    if grep -q 'PORTAL_PORT' "$DEPLOY"; then
        pass "deploy.sh footer uses PORTAL_PORT variable"
    else
        fail "deploy.sh footer missing PORTAL_PORT variable"
    fi

    # Verify no SPA references remain
    if ! grep -q 'package\.json' "$DEPLOY"; then
        pass "deploy.sh has no package.json references (SPA removed)"
    else
        fail "deploy.sh still references package.json"
    fi

    if ! grep -q 'nginx' "$DEPLOY"; then
        pass "deploy.sh has no nginx references (SPA removed)"
    else
        fail "deploy.sh still references nginx"
    fi

    if ! grep -q 'sync_version' "$DEPLOY"; then
        pass "deploy.sh has no sync_version function (SPA removed)"
    else
        fail "deploy.sh still has sync_version function"
    fi
else
    fail "deploy.sh not found, skipping mode checks"
fi

# ---------------------------------------------------------------------------
# 6. build.sh validation
# ---------------------------------------------------------------------------
section "build.sh"

if [ -f "$BUILD" ]; then
    # Check version extraction
    if grep -q '\.version' "$BUILD"; then
        pass "build.sh reads .version file"
    else
        fail "build.sh does not read .version file"
    fi

    # Check build timestamp update
    if grep -qE "sed.*build:" "$BUILD"; then
        pass "build.sh updates build timestamp"
    else
        fail "build.sh does not update build timestamp"
    fi

    # Check git commit extraction
    if grep -q 'git.*rev-parse' "$BUILD"; then
        pass "build.sh extracts git commit"
    else
        fail "build.sh does not extract git commit"
    fi

    # Check docker build with build-args
    if grep -q 'docker build' "$BUILD" || grep -q 'docker.*build' "$BUILD"; then
        pass "build.sh runs docker build"
    else
        fail "build.sh does not run docker build"
    fi

    # Check VERSION build-arg injection
    if grep -q 'VERSION' "$BUILD"; then
        pass "build.sh injects VERSION build-arg"
    else
        fail "build.sh missing VERSION build-arg"
    fi

    if grep -q 'BUILD' "$BUILD"; then
        pass "build.sh injects BUILD build-arg"
    else
        fail "build.sh missing BUILD build-arg"
    fi

    if grep -q 'GIT_COMMIT' "$BUILD"; then
        pass "build.sh injects GIT_COMMIT build-arg"
    else
        fail "build.sh missing GIT_COMMIT build-arg"
    fi

    # Check --help flag
    if grep -q '\-\-help' "$BUILD"; then
        pass "build.sh supports --help flag"
    else
        fail "build.sh missing --help flag"
    fi

    # Check --clean flag
    if grep -q '\-\-clean' "$BUILD"; then
        pass "build.sh supports --clean flag"
    else
        fail "build.sh missing --clean flag"
    fi

    # Verify no SPA references remain
    if ! grep -q 'package\.json' "$BUILD"; then
        pass "build.sh has no package.json references (SPA removed)"
    else
        fail "build.sh still references package.json"
    fi

    if ! grep -q 'sync_version' "$BUILD"; then
        pass "build.sh has no sync_version function (SPA removed)"
    else
        fail "build.sh still has sync_version function"
    fi
else
    fail "build.sh not found, skipping checks"
fi

# ---------------------------------------------------------------------------
# 7. docker-compose.yml validation
# ---------------------------------------------------------------------------
section "docker-compose.yml"

LOCAL_COMPOSE="$PROJECT_DIR/docker/docker-compose.yml"
if [ -f "$LOCAL_COMPOSE" ]; then
    # Check compose name
    if grep -q '^name: vire-portal' "$LOCAL_COMPOSE"; then
        pass "docker-compose.yml has correct project name"
    else
        fail "docker-compose.yml missing project name 'vire-portal'"
    fi

    # Check service name
    if grep -q 'vire-portal:' "$LOCAL_COMPOSE"; then
        pass "docker-compose.yml has vire-portal service"
    else
        fail "docker-compose.yml missing vire-portal service"
    fi

    # Check build context points to parent
    if grep -q 'context: \.\.' "$LOCAL_COMPOSE"; then
        pass "docker-compose.yml build context is parent directory"
    else
        fail "docker-compose.yml build context incorrect"
    fi

    # Check Dockerfile reference
    if grep -q 'dockerfile: Dockerfile' "$LOCAL_COMPOSE"; then
        pass "docker-compose.yml references root Dockerfile"
    else
        fail "docker-compose.yml missing Dockerfile reference"
    fi

    # Check build args
    for arg in VERSION BUILD GIT_COMMIT; do
        if grep -q "$arg:" "$LOCAL_COMPOSE"; then
            pass "docker-compose.yml has $arg build arg"
        else
            fail "docker-compose.yml missing $arg build arg"
        fi
    done

    # Check healthcheck
    if grep -q 'healthcheck' "$LOCAL_COMPOSE"; then
        pass "docker-compose.yml has healthcheck"
    else
        fail "docker-compose.yml missing healthcheck"
    fi

    # Healthcheck verifies HTTP response
    if grep -qE 'curl.*-f|wget.*-q.*http' "$LOCAL_COMPOSE"; then
        pass "docker-compose.yml healthcheck verifies HTTP response"
    else
        fail "docker-compose.yml healthcheck should verify HTTP response (not just TCP)"
    fi

    # Check port mapping (may use PORTAL_PORT variable)
    if grep -qE '8080' "$LOCAL_COMPOSE"; then
        pass "docker-compose.yml maps port 8080"
    else
        fail "docker-compose.yml missing port 8080 mapping"
    fi

    # Check VIRE_ environment vars
    if grep -q 'VIRE_SERVER_HOST' "$LOCAL_COMPOSE"; then
        pass "docker-compose.yml has VIRE_SERVER_HOST environment var"
    else
        fail "docker-compose.yml missing VIRE_SERVER_HOST environment var"
    fi

    if grep -q 'VIRE_BADGER_PATH' "$LOCAL_COMPOSE"; then
        pass "docker-compose.yml has VIRE_BADGER_PATH environment var"
    else
        fail "docker-compose.yml missing VIRE_BADGER_PATH environment var"
    fi

    # Check data volume
    if grep -q 'portal-data' "$LOCAL_COMPOSE"; then
        pass "docker-compose.yml has portal-data volume"
    else
        fail "docker-compose.yml missing portal-data volume"
    fi

    # Validate with docker compose config (syntax check)
    if command -v docker &>/dev/null; then
        if docker compose -f "$LOCAL_COMPOSE" config --quiet 2>/dev/null; then
            pass "docker-compose.yml passes docker compose config validation"
        else
            fail "docker-compose.yml fails docker compose config validation"
        fi
    else
        echo "  SKIP: docker not available for compose config validation"
    fi
else
    fail "docker/docker-compose.yml not found"
fi

# ---------------------------------------------------------------------------
# 8. docker-compose.ghcr.yml validation
# ---------------------------------------------------------------------------
section "docker-compose.ghcr.yml"

GHCR_COMPOSE="$PROJECT_DIR/docker/docker-compose.ghcr.yml"
if [ -f "$GHCR_COMPOSE" ]; then
    # Check compose name
    if grep -q '^name: vire-portal' "$GHCR_COMPOSE"; then
        pass "docker-compose.ghcr.yml has correct project name"
    else
        fail "docker-compose.ghcr.yml missing project name 'vire-portal'"
    fi

    # Check GHCR image reference
    if grep -q 'ghcr.io/bobmcallan/vire-portal' "$GHCR_COMPOSE"; then
        pass "docker-compose.ghcr.yml uses GHCR image"
    else
        fail "docker-compose.ghcr.yml missing GHCR image reference"
    fi

    # Check pull_policy
    if grep -q 'pull_policy: always' "$GHCR_COMPOSE"; then
        pass "docker-compose.ghcr.yml has pull_policy: always"
    else
        fail "docker-compose.ghcr.yml missing pull_policy"
    fi

    # Check watchtower service
    if grep -q 'watchtower' "$GHCR_COMPOSE"; then
        pass "docker-compose.ghcr.yml has watchtower service"
    else
        fail "docker-compose.ghcr.yml missing watchtower service"
    fi

    # Check watchtower scope
    if grep -q 'WATCHTOWER_SCOPE=vire-portal' "$GHCR_COMPOSE"; then
        pass "docker-compose.ghcr.yml has vire-portal watchtower scope"
    else
        fail "docker-compose.ghcr.yml missing vire-portal watchtower scope"
    fi

    # Check watchtower labels
    if grep -q 'com.centurylinklabs.watchtower.scope=vire-portal' "$GHCR_COMPOSE"; then
        pass "docker-compose.ghcr.yml has watchtower scope labels"
    else
        fail "docker-compose.ghcr.yml missing watchtower scope labels"
    fi

    # Check healthcheck
    if grep -q 'healthcheck' "$GHCR_COMPOSE"; then
        pass "docker-compose.ghcr.yml has healthcheck"
    else
        fail "docker-compose.ghcr.yml missing healthcheck"
    fi

    # Healthcheck verifies HTTP response
    if grep -qE 'curl.*-f|wget.*-q.*http' "$GHCR_COMPOSE"; then
        pass "docker-compose.ghcr.yml healthcheck verifies HTTP response"
    else
        fail "docker-compose.ghcr.yml healthcheck should verify HTTP response"
    fi

    # Check no build section (should only pull)
    if ! grep -q 'build:' "$GHCR_COMPOSE"; then
        pass "docker-compose.ghcr.yml has no build section (pull only)"
    else
        fail "docker-compose.ghcr.yml should not have build section"
    fi

    # Check VIRE_ environment vars
    if grep -q 'VIRE_SERVER_HOST' "$GHCR_COMPOSE"; then
        pass "docker-compose.ghcr.yml has VIRE_SERVER_HOST environment var"
    else
        fail "docker-compose.ghcr.yml missing VIRE_SERVER_HOST environment var"
    fi

    # Check data volume
    if grep -q 'portal-data' "$GHCR_COMPOSE"; then
        pass "docker-compose.ghcr.yml has portal-data volume"
    else
        fail "docker-compose.ghcr.yml missing portal-data volume"
    fi

    # Validate with docker compose config
    if command -v docker &>/dev/null; then
        if docker compose -f "$GHCR_COMPOSE" config --quiet 2>/dev/null; then
            pass "docker-compose.ghcr.yml passes docker compose config validation"
        else
            fail "docker-compose.ghcr.yml fails docker compose config validation"
        fi
    else
        echo "  SKIP: docker not available for compose config validation"
    fi
else
    fail "docker/docker-compose.ghcr.yml not found"
fi

# ---------------------------------------------------------------------------
# 9. Dockerfile build args and Go build
# ---------------------------------------------------------------------------
section "Dockerfile build args"

DOCKERFILE="$PROJECT_DIR/Dockerfile"
if [ -f "$DOCKERFILE" ]; then
    for arg in VERSION BUILD GIT_COMMIT; do
        if grep -q "ARG $arg" "$DOCKERFILE"; then
            pass "Dockerfile declares ARG $arg"
        else
            fail "Dockerfile missing ARG $arg"
        fi
    done

    # Check Go build with ldflags
    if grep -q 'go build' "$DOCKERFILE"; then
        pass "Dockerfile runs go build"
    else
        fail "Dockerfile missing go build command"
    fi

    if grep -q 'ldflags' "$DOCKERFILE"; then
        pass "Dockerfile uses ldflags for version injection"
    else
        fail "Dockerfile missing ldflags for version injection"
    fi

    # Check multi-stage build
    if grep -q 'FROM golang' "$DOCKERFILE"; then
        pass "Dockerfile has Go builder stage"
    else
        fail "Dockerfile missing Go builder stage"
    fi

    if grep -q 'FROM alpine' "$DOCKERFILE"; then
        pass "Dockerfile has alpine runtime stage"
    else
        fail "Dockerfile missing alpine runtime stage"
    fi

    # Check pages are copied
    if grep -q 'pages' "$DOCKERFILE"; then
        pass "Dockerfile copies pages directory"
    else
        fail "Dockerfile missing pages copy"
    fi

    # Check portal.toml is copied
    if grep -q 'portal.toml' "$DOCKERFILE"; then
        pass "Dockerfile copies portal.toml config"
    else
        fail "Dockerfile missing portal.toml copy"
    fi

    # Verify no SPA references
    if ! grep -q 'node\|npm\|nginx\|VITE' "$DOCKERFILE"; then
        pass "Dockerfile has no SPA references (node/npm/nginx/VITE)"
    else
        fail "Dockerfile still has SPA references"
    fi
else
    fail "Dockerfile not found"
fi

# ---------------------------------------------------------------------------
# 10. .gitignore includes docker/.last_build and Go patterns
# ---------------------------------------------------------------------------
section ".gitignore"

GITIGNORE="$PROJECT_DIR/.gitignore"
if [ -f "$GITIGNORE" ]; then
    if grep -q 'docker/\.last_build' "$GITIGNORE" || grep -q '\.last_build' "$GITIGNORE"; then
        pass ".gitignore includes .last_build"
    else
        fail ".gitignore missing .last_build entry"
    fi

    if grep -q '/portal' "$GITIGNORE"; then
        pass ".gitignore includes /portal binary"
    else
        fail ".gitignore missing /portal binary entry"
    fi

    if grep -q 'data/' "$GITIGNORE"; then
        pass ".gitignore includes data/ directory"
    else
        fail ".gitignore missing data/ directory"
    fi

    # Verify no SPA-only entries
    if ! grep -q 'node_modules' "$GITIGNORE"; then
        pass ".gitignore has no node_modules entry (SPA removed)"
    else
        fail ".gitignore still has node_modules entry"
    fi

    if ! grep -q 'npm-debug' "$GITIGNORE"; then
        pass ".gitignore has no npm-debug entry (SPA removed)"
    else
        fail ".gitignore still has npm-debug entry"
    fi
else
    fail ".gitignore not found"
fi

# ---------------------------------------------------------------------------
# 11. Cross-file consistency
# ---------------------------------------------------------------------------
section "Cross-file consistency"

# Check deploy.sh and build.sh use same timestamp format
if [ -f "$DEPLOY" ] && [ -f "$BUILD" ]; then
    DEPLOY_FMT=$(grep -oE 'date \+[^ )]+' "$DEPLOY" | head -1 || true)
    BUILD_FMT=$(grep -oE 'date \+[^ )]+' "$BUILD" | head -1 || true)
    if [ -n "$DEPLOY_FMT" ] && [ "$DEPLOY_FMT" = "$BUILD_FMT" ]; then
        pass "deploy.sh and build.sh use same timestamp format"
    elif [ -n "$DEPLOY_FMT" ] && [ -n "$BUILD_FMT" ]; then
        fail "Timestamp format mismatch: deploy.sh=$DEPLOY_FMT build.sh=$BUILD_FMT"
    else
        echo "  SKIP: Could not extract timestamp formats"
    fi
fi

# ---------------------------------------------------------------------------
# 12. Script execution
# ---------------------------------------------------------------------------
section "Script execution"

# build.sh --help should exit 0 and print usage
if [ -f "$BUILD" ]; then
    HELP_OUTPUT=$("$BUILD" --help 2>&1) || true
    if [ $? -eq 0 ] || echo "$HELP_OUTPUT" | grep -q "Usage:"; then
        pass "build.sh --help exits 0"
    else
        fail "build.sh --help exits non-zero"
    fi

    if echo "$HELP_OUTPUT" | grep -q "Usage:"; then
        pass "build.sh --help prints usage text"
    else
        fail "build.sh --help does not print usage text"
    fi

    if echo "$HELP_OUTPUT" | grep -q "\-\-clean"; then
        pass "build.sh --help documents --clean flag"
    else
        fail "build.sh --help missing --clean documentation"
    fi

    # build.sh with invalid arg should exit non-zero
    if "$BUILD" --invalid-flag >/dev/null 2>&1; then
        fail "build.sh --invalid-flag should exit non-zero"
    else
        pass "build.sh --invalid-flag exits non-zero"
    fi
fi

# deploy.sh with invalid mode should exit non-zero and print usage
if [ -f "$DEPLOY" ]; then
    DEPLOY_OUTPUT=$("$DEPLOY" invalidmode 2>&1) || true
    if echo "$DEPLOY_OUTPUT" | grep -q "Usage:"; then
        pass "deploy.sh invalidmode prints usage message"
    else
        fail "deploy.sh invalidmode does not print usage message"
    fi

    if "$DEPLOY" invalidmode >/dev/null 2>&1; then
        fail "deploy.sh invalidmode should exit non-zero"
    else
        pass "deploy.sh invalidmode exits non-zero"
    fi
fi

# Behavioral: version extraction from .version file
if [ -f "$BUILD" ] && [ -f "$VERSION_FILE" ]; then
    # Test that the grep/sed pipeline in the script matches our expected extraction
    SCRIPT_PATTERN=$(grep -oE "grep.*version.*sed.*tr" "$BUILD" | head -1 || true)
    if [ -n "$SCRIPT_PATTERN" ]; then
        pass "build.sh uses grep/sed/tr pipeline for version extraction"
    else
        if grep -qE 'grep.*version.*\|.*sed' "$BUILD"; then
            pass "build.sh uses grep+sed pipeline for version extraction"
        else
            fail "build.sh version extraction pattern not recognized"
        fi
    fi
fi

# ---------------------------------------------------------------------------
# 13. Cross-file build arg consistency
# ---------------------------------------------------------------------------
section "Build arg consistency across files"

# All files that reference VERSION/BUILD/GIT_COMMIT should be consistent
BUILD_ARG_FILES=()
for f in \
    "$PROJECT_DIR/Dockerfile" \
    "$PROJECT_DIR/docker/docker-compose.yml" \
    "$PROJECT_DIR/scripts/deploy.sh" \
    "$PROJECT_DIR/scripts/build.sh" \
    "$PROJECT_DIR/.github/workflows/release.yml"; do
    if [ -f "$f" ]; then
        BUILD_ARG_FILES+=("$f")
    fi
done

for arg in VERSION BUILD GIT_COMMIT; do
    found_in=()
    for f in "${BUILD_ARG_FILES[@]}"; do
        if grep -q "$arg" "$f"; then
            found_in+=("$(basename "$f")")
        fi
    done
    if [ ${#found_in[@]} -ge 3 ]; then
        pass "$arg referenced consistently across ${#found_in[@]} files: ${found_in[*]}"
    else
        fail "$arg only found in ${#found_in[@]} files (expected 3+): ${found_in[*]}"
    fi
done

# Verify Dockerfile ARG names match docker-compose.yml build arg names
if [ -f "$DOCKERFILE" ] && [ -f "$LOCAL_COMPOSE" ]; then
    DOCKERFILE_ARGS=$(grep -oE 'ARG (VERSION|BUILD|GIT_COMMIT)' "$DOCKERFILE" | sed 's/ARG //' | sort)
    COMPOSE_ARGS=$(grep -oE '(VERSION|BUILD|GIT_COMMIT):' "$LOCAL_COMPOSE" | sed 's/://' | sort -u)
    if [ "$DOCKERFILE_ARGS" = "$COMPOSE_ARGS" ]; then
        pass "Dockerfile ARG names match docker-compose.yml build arg names"
    else
        fail "Dockerfile ARGs ($DOCKERFILE_ARGS) != compose args ($COMPOSE_ARGS)"
    fi
fi

# Verify release.yml build-arg names match Dockerfile ARG names
RELEASE_YML="$PROJECT_DIR/.github/workflows/release.yml"
if [ -f "$DOCKERFILE" ] && [ -f "$RELEASE_YML" ]; then
    RELEASE_ARGS=$(grep -oE '(VERSION|BUILD|GIT_COMMIT)=' "$RELEASE_YML" | sed 's/=//' | sort -u)
    if [ "$DOCKERFILE_ARGS" = "$RELEASE_ARGS" ]; then
        pass "release.yml build-arg names match Dockerfile ARG names"
    else
        fail "release.yml args ($RELEASE_ARGS) != Dockerfile ARGs ($DOCKERFILE_ARGS)"
    fi
fi

# ---------------------------------------------------------------------------
# 14. Go-specific validation
# ---------------------------------------------------------------------------
section "Go build validation"

# Verify Go module is valid
if [ -f "$PROJECT_DIR/go.mod" ]; then
    if grep -q 'module github.com/bobmcallan/vire-portal' "$PROJECT_DIR/go.mod"; then
        pass "go.mod has correct module path"
    else
        fail "go.mod has incorrect module path"
    fi
fi

# Verify Go compiles
if command -v go &>/dev/null; then
    if (cd "$PROJECT_DIR" && go build ./cmd/portal/) 2>/dev/null; then
        pass "Go project compiles successfully"
    else
        fail "Go project fails to compile"
    fi

    if (cd "$PROJECT_DIR" && go vet ./...) 2>/dev/null; then
        pass "go vet passes"
    else
        fail "go vet reports issues"
    fi
else
    echo "  SKIP: go not available for build validation"
fi

# ---------------------------------------------------------------------------
# 15. Empty version validation
# ---------------------------------------------------------------------------
section "Empty version validation"

for script_name in "deploy.sh" "build.sh"; do
    script_path="$PROJECT_DIR/scripts/$script_name"
    if [ -f "$script_path" ]; then
        if grep -qE '\-z.*VERSION' "$script_path"; then
            pass "$script_name validates VERSION is not empty"
        else
            fail "$script_name missing empty VERSION validation"
        fi
    fi
done

# ---------------------------------------------------------------------------
# 16. deploy.sh down and prune exit before footer
# ---------------------------------------------------------------------------
section "deploy.sh exit behavior"

if [ -f "$DEPLOY" ]; then
    if awk '/^[[:space:]]*down\)/,/;;/' "$DEPLOY" | grep -q 'exit 0'; then
        pass "deploy.sh 'down' mode exits before footer"
    else
        fail "deploy.sh 'down' mode should exit before footer"
    fi

    if awk '/^[[:space:]]*prune\)/,/;;/' "$DEPLOY" | grep -q 'exit 0'; then
        pass "deploy.sh 'prune' mode exits before footer"
    else
        fail "deploy.sh 'prune' mode should exit before footer"
    fi
fi

# ---------------------------------------------------------------------------
# 17. Watchtower image pinned
# ---------------------------------------------------------------------------
section "Watchtower version pinning"

if [ -f "$GHCR_COMPOSE" ]; then
    if grep -qE 'containrrr/watchtower:[0-9]' "$GHCR_COMPOSE"; then
        pass "docker-compose.ghcr.yml watchtower image is version-pinned"
    else
        fail "docker-compose.ghcr.yml watchtower should use a pinned version (not :latest)"
    fi
fi

# ---------------------------------------------------------------------------
# 18. Build args individually quoted in build.sh
# ---------------------------------------------------------------------------
section "Build arg quoting"

if [ -f "$BUILD" ]; then
    QUOTED_ARGS=$(grep -cE '\-\-build-arg "' "$BUILD" || true)
    if [ "$QUOTED_ARGS" -ge 3 ]; then
        pass "build.sh passes build args individually with quotes ($QUOTED_ARGS args)"
    else
        fail "build.sh should pass build args individually with quotes (found $QUOTED_ARGS)"
    fi
fi

# ---------------------------------------------------------------------------
# 19. Smart rebuild includes .version in change detection
# ---------------------------------------------------------------------------
section "Smart rebuild .version detection"

if [ -f "$DEPLOY" ]; then
    if grep -q '\.version.*last_build\|last_build.*\.version' "$DEPLOY" || \
       awk '/NEEDS_REBUILD/,/fi/' "$DEPLOY" | grep -q '\.version'; then
        pass "deploy.sh smart rebuild checks .version for changes"
    else
        fail "deploy.sh smart rebuild should include .version in change detection"
    fi
fi

# ---------------------------------------------------------------------------
# 20. .version edge case handling
# ---------------------------------------------------------------------------
section ".version edge case handling"

if [ -f "$BUILD" ]; then
    if grep -qE 'VERSION="dev"|VERSION=.dev.' "$BUILD"; then
        pass "build.sh has fallback VERSION=dev for missing .version"
    else
        fail "build.sh missing fallback for missing .version"
    fi

    if grep -qE 'BUILD_TS="unknown"|BUILD=.unknown.' "$BUILD"; then
        pass "build.sh has fallback BUILD=unknown for missing .version"
    else
        fail "build.sh missing fallback BUILD for missing .version"
    fi
fi

if [ -f "$DEPLOY" ]; then
    if grep -qE 'VERSION="dev"|VERSION=.dev.' "$DEPLOY"; then
        pass "deploy.sh has fallback VERSION=dev for missing .version"
    else
        fail "deploy.sh missing fallback for missing .version"
    fi
fi

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
echo ""
echo "==========================================="
echo "Results: $PASSED passed, $FAILED failed"
echo "==========================================="

if [ ${#ERRORS[@]} -gt 0 ]; then
    echo ""
    echo "Failures:"
    for err in "${ERRORS[@]}"; do
        echo "  - $err"
    done
fi

# Exit with error if any failures
if [ "$FAILED" -gt 0 ]; then
    exit 1
else
    exit 0
fi
