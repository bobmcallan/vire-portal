#!/bin/bash
set -euo pipefail

# Build script for vire-portal and vire-mcp
# Builds Go binaries to bin/ (default) or Docker images (--docker)

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Parse arguments
VERBOSE=false
CLEAN=false
DOCKER=false
BUILD_TARGET="all"  # all, portal, mcp

while [[ $# -gt 0 ]]; do
    case $1 in
        -v|--verbose) VERBOSE=true; shift ;;
        -c|--clean) CLEAN=true; shift ;;
        -d|--docker) DOCKER=true; shift ;;
        --portal) BUILD_TARGET="portal"; shift ;;
        --mcp) BUILD_TARGET="mcp"; shift ;;
        -h|--help)
            echo "Usage: $0 [options]"
            echo "Options:"
            echo "  -d, --docker   Build Docker images (default: Go binaries to bin/)"
            echo "  --portal       Build only vire-portal"
            echo "  --mcp          Build only vire-mcp"
            echo "  -v, --verbose  Show verbose build output"
            echo "  -c, --clean    Remove existing artifacts before building"
            echo "  -h, --help     Show this help message"
            exit 0
            ;;
        *) echo "Unknown option: $1"; exit 1 ;;
    esac
done

cd "$PROJECT_ROOT"

VERSION_FILE="$PROJECT_ROOT/.version"
BIN_DIR="$PROJECT_ROOT/bin"
CONFIG_DIR="$PROJECT_ROOT/config"

# get_section_version extracts a version from a .version section.
# Usage: get_section_version <section-name>
get_section_version() {
    local section="$1"
    awk -v sec="[$section]" '$0==sec{f=1;next} /^\[/{f=0} f && /^version:/{gsub(/version:\s*/,""); gsub(/ /,""); print; exit}' "$VERSION_FILE"
}

# update_section_build updates the build timestamp in a .version section.
# Usage: update_section_build <section-name> <timestamp>
update_section_build() {
    local section="$1" ts="$2"
    sed -i '/^\['"$section"'\]/,/^\[/{s/^build:.*/build: '"$ts"'/}' "$VERSION_FILE"
}

# Note: local builds use MM-DD-HH-MM-SS format; CI uses YYYYMMDDHHmmss (intentional difference)
BUILD_TS=$(date +"%m-%d-%H-%M-%S")
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
CONTRIBUTOR=$(git config user.email 2>/dev/null || echo "unknown")

# ---------------------------------------------------------------------------
# Go binary build
# ---------------------------------------------------------------------------
build_binary() {
    local cmd_path="$1"
    local binary_name="$2"
    local section="$3"
    local ldflags_pkg="$4"

    local version="dev"
    if [[ -f "$VERSION_FILE" ]]; then
        version=$(get_section_version "$section")
        update_section_build "$section" "$BUILD_TS"
    fi

    if [ -z "$version" ]; then
        echo "Error: version is empty for [$section] in .version"
        exit 1
    fi

    echo "Building $binary_name v$version (commit: $GIT_COMMIT)..."
    mkdir -p "$BIN_DIR"

    go build -ldflags="-s -w \
        -X '${ldflags_pkg}.Version=${version}' \
        -X '${ldflags_pkg}.Build=${BUILD_TS}' \
        -X '${ldflags_pkg}.GitCommit=${GIT_COMMIT}'" \
        -o "$BIN_DIR/$binary_name" "./$cmd_path"
}

build_binaries() {
    if [[ "$BUILD_TARGET" == "all" || "$BUILD_TARGET" == "portal" ]]; then
        build_binary "cmd/vire-portal" "vire-portal" "vire-portal" \
            "github.com/bobmcallan/vire-portal/internal/config"
    fi

    if [[ "$BUILD_TARGET" == "all" || "$BUILD_TARGET" == "mcp" ]]; then
        build_binary "cmd/vire-mcp" "vire-mcp" "vire-mcp" \
            "github.com/bobmcallan/vire-portal/internal/vire/common"
    fi

    # Stage configs and assets into bin/
    if [[ "$BUILD_TARGET" == "all" || "$BUILD_TARGET" == "portal" ]]; then
        [ -f "$CONFIG_DIR/vire-portal.toml" ] && cp "$CONFIG_DIR/vire-portal.toml" "$BIN_DIR/"
        rm -rf "$BIN_DIR/pages"
        cp -r "$PROJECT_ROOT/pages" "$BIN_DIR/pages"
    fi
    if [[ "$BUILD_TARGET" == "all" || "$BUILD_TARGET" == "mcp" ]]; then
        [ -f "$CONFIG_DIR/vire-mcp.toml" ] && cp "$CONFIG_DIR/vire-mcp.toml" "$BIN_DIR/"
    fi

}

# ---------------------------------------------------------------------------
# Docker image build
# ---------------------------------------------------------------------------
build_image() {
    local dockerfile="$1"
    local image_name="$2"
    local section="$3"

    local version="dev"
    if [[ -f "$VERSION_FILE" ]]; then
        version=$(get_section_version "$section")
        update_section_build "$section" "$BUILD_TS"
    fi

    if [ -z "$version" ]; then
        echo "Error: version is empty for [$section] in .version"
        exit 1
    fi

    echo "Building $image_name v$version (commit: $GIT_COMMIT)..."

    if [[ "$VERBOSE" == "true" ]]; then
        docker build \
            -f "$dockerfile" \
            --build-arg "VERSION=$version" \
            --build-arg "BUILD=$BUILD_TS" \
            --build-arg "GIT_COMMIT=$GIT_COMMIT" \
            --build-arg "CONTRIBUTOR=$CONTRIBUTOR" \
            --progress=plain \
            -t "$image_name:latest" \
            -t "$image_name:$version" .
    else
        docker build \
            -f "$dockerfile" \
            --build-arg "VERSION=$version" \
            --build-arg "BUILD=$BUILD_TS" \
            --build-arg "GIT_COMMIT=$GIT_COMMIT" \
            --build-arg "CONTRIBUTOR=$CONTRIBUTOR" \
            -t "$image_name:latest" \
            -t "$image_name:$version" .
    fi

    local image_size
    image_size=$(docker image inspect "$image_name:latest" --format='{{.Size}}' 2>/dev/null | awk '{printf "%.1fMB", $1/1024/1024}')
    echo "  Built: $image_name:latest ($image_size)"
    echo "  Built: $image_name:$version"
}

build_images() {
    if [[ "$BUILD_TARGET" == "all" || "$BUILD_TARGET" == "portal" ]]; then
        build_image "docker/Dockerfile" "vire-portal" "vire-portal"
    fi

    if [[ "$BUILD_TARGET" == "all" || "$BUILD_TARGET" == "mcp" ]]; then
        build_image "docker/Dockerfile.mcp" "vire-mcp" "vire-mcp"
    fi
}

# ---------------------------------------------------------------------------
# Clean
# ---------------------------------------------------------------------------
if [[ "$CLEAN" == "true" ]]; then
    echo "Removing existing artifacts..."
    if [[ "$DOCKER" == "true" ]]; then
        PORTAL_VER="unknown"
        MCP_VER="unknown"
        if [[ -f "$VERSION_FILE" ]]; then
            PORTAL_VER=$(get_section_version "vire-portal")
            MCP_VER=$(get_section_version "vire-mcp")
        fi
        docker image rm vire-portal:latest "vire-portal:$PORTAL_VER" vire-mcp:latest "vire-mcp:$MCP_VER" 2>/dev/null || true
    else
        rm -rf "$BIN_DIR"
    fi
fi

# ---------------------------------------------------------------------------
# Ensure configs exist (binary builds only)
# ---------------------------------------------------------------------------
if [[ "$DOCKER" == "false" ]]; then
    if [ ! -f "$CONFIG_DIR/vire-portal.toml" ] && [ -f "$CONFIG_DIR/vire-portal.toml.example" ]; then
        echo "Creating config/vire-portal.toml from example..."
        cp "$CONFIG_DIR/vire-portal.toml.example" "$CONFIG_DIR/vire-portal.toml"
    fi
    if [ ! -f "$CONFIG_DIR/vire-mcp.toml" ] && [ -f "$CONFIG_DIR/vire-mcp.toml.example" ]; then
        echo "Creating config/vire-mcp.toml from example..."
        cp "$CONFIG_DIR/vire-mcp.toml.example" "$CONFIG_DIR/vire-mcp.toml"
    fi
fi

# ---------------------------------------------------------------------------
# Build
# ---------------------------------------------------------------------------
if [[ "$DOCKER" == "true" ]]; then
    build_images
else
    build_binaries
fi

echo ""
echo "Done."
