#!/bin/bash
set -euo pipefail

# Docker image builder for vire-portal and vire-mcp
# Builds Go binaries in multi-stage Docker images

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Parse arguments
VERBOSE=false
CLEAN=false
BUILD_TARGET="all"  # all, portal, mcp

while [[ $# -gt 0 ]]; do
    case $1 in
        -v|--verbose) VERBOSE=true; shift ;;
        -c|--clean) CLEAN=true; shift ;;
        --portal) BUILD_TARGET="portal"; shift ;;
        --mcp) BUILD_TARGET="mcp"; shift ;;
        -h|--help)
            echo "Usage: $0 [options]"
            echo "Options:"
            echo "  -v, --verbose  Show verbose build output"
            echo "  -c, --clean    Remove existing images before building"
            echo "  --portal       Build only vire-portal image"
            echo "  --mcp          Build only vire-mcp image"
            echo "  -h, --help     Show this help message"
            exit 0
            ;;
        *) echo "Unknown option: $1"; exit 1 ;;
    esac
done

cd "$PROJECT_ROOT"

# Clean if requested
if [[ "$CLEAN" == "true" ]]; then
    echo "Removing existing images..."
    CLEAN_VERSION="unknown"
    if [[ -f "$PROJECT_ROOT/.version" ]]; then
        CLEAN_VERSION=$(grep "^version:" "$PROJECT_ROOT/.version" | sed 's/version:\s*//' | tr -d ' ')
    fi
    docker image rm vire-portal:latest "vire-portal:$CLEAN_VERSION" vire-mcp:latest "vire-mcp:$CLEAN_VERSION" 2>/dev/null || true
fi

# Extract version info
VERSION="dev"
BUILD_TS="unknown"
VERSION_FILE="$PROJECT_ROOT/.version"
if [[ -f "$VERSION_FILE" ]]; then
    VERSION=$(grep "^version:" "$VERSION_FILE" | sed 's/version:\s*//' | tr -d ' ')
    # Note: local builds use MM-DD-HH-MM-SS format; CI uses YYYYMMDDHHmmss (intentional difference)
    BUILD_TS=$(date +"%m-%d-%H-%M-%S")
fi
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
CONTRIBUTOR=$(git config user.email 2>/dev/null || echo "unknown")

# Validate version is not empty (would produce invalid docker tags)
if [ -z "$VERSION" ]; then
    echo "Error: version is empty after extraction from .version"
    exit 1
fi

# build_image builds a Docker image with version injection.
# Usage: build_image <dockerfile> <image-name>
build_image() {
    local dockerfile="$1"
    local image_name="$2"

    echo "Building $image_name v$VERSION (commit: $GIT_COMMIT)..."

    if [[ "$VERBOSE" == "true" ]]; then
        docker build \
            -f "$dockerfile" \
            --build-arg "VERSION=$VERSION" \
            --build-arg "BUILD=$BUILD_TS" \
            --build-arg "GIT_COMMIT=$GIT_COMMIT" \
            --build-arg "CONTRIBUTOR=$CONTRIBUTOR" \
            --progress=plain \
            -t "$image_name:latest" \
            -t "$image_name:$VERSION" .
    else
        docker build \
            -f "$dockerfile" \
            --build-arg "VERSION=$VERSION" \
            --build-arg "BUILD=$BUILD_TS" \
            --build-arg "GIT_COMMIT=$GIT_COMMIT" \
            --build-arg "CONTRIBUTOR=$CONTRIBUTOR" \
            -t "$image_name:latest" \
            -t "$image_name:$VERSION" .
    fi

    local image_size
    image_size=$(docker image inspect "$image_name:latest" --format='{{.Size}}' 2>/dev/null | awk '{printf "%.1fMB", $1/1024/1024}')
    echo "  Built: $image_name:latest ($image_size)"
    echo "  Built: $image_name:$VERSION"
}

# Build images based on target
if [[ "$BUILD_TARGET" == "all" || "$BUILD_TARGET" == "portal" ]]; then
    build_image "docker/Dockerfile" "vire-portal"
fi

if [[ "$BUILD_TARGET" == "all" || "$BUILD_TARGET" == "mcp" ]]; then
    build_image "docker/Dockerfile.mcp" "vire-mcp"
fi

echo ""
echo "Done."
