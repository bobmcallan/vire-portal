#!/bin/bash
set -euo pipefail

# Docker image builder for vire-portal
# Builds a Go binary in a multi-stage Docker image

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Parse arguments
VERBOSE=false
CLEAN=false

while [[ $# -gt 0 ]]; do
    case $1 in
        -v|--verbose) VERBOSE=true; shift ;;
        -c|--clean) CLEAN=true; shift ;;
        -h|--help)
            echo "Usage: $0 [options]"
            echo "Options:"
            echo "  -v, --verbose  Show verbose build output"
            echo "  -c, --clean    Remove existing images before building"
            echo "  -h, --help     Show this help message"
            exit 0
            ;;
        *) echo "Unknown option: $1"; exit 1 ;;
    esac
done

cd "$PROJECT_ROOT"

# Clean if requested
if [[ "$CLEAN" == "true" ]]; then
    echo "Removing existing vire-portal images..."
    # Extract current version for targeted removal (glob not supported in docker image rm)
    CLEAN_VERSION="unknown"
    if [[ -f "$PROJECT_ROOT/.version" ]]; then
        CLEAN_VERSION=$(grep "^version:" "$PROJECT_ROOT/.version" | sed 's/version:\s*//' | tr -d ' ')
    fi
    docker image rm vire-portal:latest "vire-portal:$CLEAN_VERSION" 2>/dev/null || true
fi

# Extract version info
VERSION="dev"
BUILD_TS="unknown"
VERSION_FILE="$PROJECT_ROOT/.version"
if [[ -f "$VERSION_FILE" ]]; then
    VERSION=$(grep "^version:" "$VERSION_FILE" | sed 's/version:\s*//' | tr -d ' ')
    # Note: local builds use MM-DD-HH-MM-SS format; CI uses YYYYMMDDHHmmss (intentional difference)
    BUILD_TS=$(date +"%m-%d-%H-%M-%S")
    # Update build timestamp in .version file
    sed -i "s/^build:.*/build: $BUILD_TS/" "$VERSION_FILE"
fi
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Validate version is not empty (would produce invalid docker tags)
if [ -z "$VERSION" ]; then
    echo "Error: version is empty after extraction from .version"
    exit 1
fi

echo "Building vire-portal v$VERSION (commit: $GIT_COMMIT)..."

# Build Docker image (args passed individually to avoid word-splitting issues)
if [[ "$VERBOSE" == "true" ]]; then
    docker build \
        --build-arg "VERSION=$VERSION" \
        --build-arg "BUILD=$BUILD_TS" \
        --build-arg "GIT_COMMIT=$GIT_COMMIT" \
        --progress=plain \
        -t "vire-portal:latest" \
        -t "vire-portal:$VERSION" .
else
    docker build \
        --build-arg "VERSION=$VERSION" \
        --build-arg "BUILD=$BUILD_TS" \
        --build-arg "GIT_COMMIT=$GIT_COMMIT" \
        -t "vire-portal:latest" \
        -t "vire-portal:$VERSION" .
fi

# Show result
IMAGE_SIZE=$(docker image inspect "vire-portal:latest" --format='{{.Size}}' 2>/dev/null | awk '{printf "%.1fMB", $1/1024/1024}')
echo ""
echo "Built Docker image:"
echo "  vire-portal:latest ($IMAGE_SIZE)"
echo "  vire-portal:$VERSION"
echo ""
echo "Run: docker run -p 8080:8080 vire-portal:latest"
