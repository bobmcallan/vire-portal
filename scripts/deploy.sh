#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
COMPOSE_DIR="$PROJECT_DIR/docker"

# Sync .version -> package.json (uses | delimiter to avoid sed injection from version strings)
sync_version() {
    local ver="$1"
    if [ -f "$PROJECT_DIR/package.json" ]; then
        sed -i "s|\"version\": \"[^\"]*\"|\"version\": \"$ver\"|" "$PROJECT_DIR/package.json"
    fi
}

# Parse arguments
MODE="${1:-local}"
FORCE=false
shift || true
for arg in "$@"; do
    case "$arg" in
        --force) FORCE=true ;;
    esac
done

case "$MODE" in
  local)
    # Extract version info
    VERSION="dev"
    # Note: local builds use MM-DD-HH-MM-SS format; CI uses YYYYMMDDHHmmss (intentional difference)
    BUILD_TS=$(date +"%m-%d-%H-%M-%S")
    if [ -f "$PROJECT_DIR/.version" ]; then
        VERSION=$(grep "^version:" "$PROJECT_DIR/.version" | sed 's/version:\s*//' | tr -d ' ')
        sed -i "s/^build:.*/build: $BUILD_TS/" "$PROJECT_DIR/.version"
    fi
    GIT_COMMIT=$(git -C "$PROJECT_DIR" rev-parse --short HEAD 2>/dev/null || echo "unknown")

    # Validate version is not empty (would produce invalid docker tags)
    if [ -z "$VERSION" ]; then
        echo "Error: version is empty after extraction from .version"
        exit 1
    fi

    export VERSION BUILD=$BUILD_TS GIT_COMMIT

    # Sync version to package.json
    sync_version "$VERSION"

    # Load docker/.env if it exists (API_URL, DOMAIN, PORTAL_PORT defaults)
    # Uses grep+xargs to safely export only VAR=value lines, skipping comments
    if [ -f "$COMPOSE_DIR/.env" ]; then
        export $(grep -v '^#' "$COMPOSE_DIR/.env" | grep -v '^\s*$' | xargs) 2>/dev/null || true
    fi

    # Smart rebuild check
    NEEDS_REBUILD=false
    if [ "$FORCE" = true ]; then
        NEEDS_REBUILD=true
    elif [ ! -f "$COMPOSE_DIR/.last_build" ]; then
        NEEDS_REBUILD=true
    else
        if find "$PROJECT_DIR/src" -newer "$COMPOSE_DIR/.last_build" 2>/dev/null | grep -q . || \
           [ "$PROJECT_DIR/package.json" -nt "$COMPOSE_DIR/.last_build" ] || \
           [ "$PROJECT_DIR/package-lock.json" -nt "$COMPOSE_DIR/.last_build" ] || \
           [ "$PROJECT_DIR/index.html" -nt "$COMPOSE_DIR/.last_build" ] || \
           [ "$PROJECT_DIR/vite.config.ts" -nt "$COMPOSE_DIR/.last_build" ] || \
           [ "$PROJECT_DIR/tsconfig.json" -nt "$COMPOSE_DIR/.last_build" ] || \
           [ "$PROJECT_DIR/nginx.conf" -nt "$COMPOSE_DIR/.last_build" ] || \
           [ "$PROJECT_DIR/Dockerfile" -nt "$COMPOSE_DIR/.last_build" ] || \
           [ "$PROJECT_DIR/.version" -nt "$COMPOSE_DIR/.last_build" ]; then
            NEEDS_REBUILD=true
        fi
    fi

    if [ "$NEEDS_REBUILD" = true ]; then
        echo "Building vire-portal v$VERSION (commit: $GIT_COMMIT)..."
        # Stop any ghcr container first (different compose file)
        docker compose -f "$COMPOSE_DIR/docker-compose.ghcr.yml" down --remove-orphans 2>/dev/null || true
        # Build new image while old containers keep running
        if [ "$FORCE" = true ]; then
            docker image rm vire-portal:latest 2>/dev/null || true
            docker compose -f "$COMPOSE_DIR/docker-compose.yml" build --no-cache
        else
            docker compose -f "$COMPOSE_DIR/docker-compose.yml" build
        fi
        touch "$COMPOSE_DIR/.last_build"
        echo " Image vire-portal:latest built "
    else
        echo "No changes detected, skipping rebuild."
    fi

    # Start or recreate container with latest image
    docker compose -f "$COMPOSE_DIR/docker-compose.yml" up -d --force-recreate --remove-orphans
    ;;
  ghcr)
    echo "Deploying ghcr image with auto-update..."
    # Stop any local-build container first (different compose file)
    docker compose -f "$COMPOSE_DIR/docker-compose.yml" down --remove-orphans 2>/dev/null || true
    # Pull new image and swap container in one step
    docker compose -f "$COMPOSE_DIR/docker-compose.ghcr.yml" up --pull always -d --force-recreate --remove-orphans
    ;;
  down)
    echo "Stopping all vire-portal containers..."
    docker compose -f "$COMPOSE_DIR/docker-compose.yml" down --remove-orphans 2>/dev/null || true
    docker compose -f "$COMPOSE_DIR/docker-compose.ghcr.yml" down --remove-orphans 2>/dev/null || true
    exit 0
    ;;
  prune)
    echo "Pruning stopped containers, dangling images, and unused volumes..."
    docker container prune -f
    docker image prune -f
    docker volume prune -f
    echo "Prune complete."
    exit 0
    ;;
  *)
    echo "Usage: ./scripts/deploy.sh [local|ghcr|down|prune] [--force]"
    exit 1
    ;;
esac

sleep 2
docker ps --filter "name=vire-portal" --format "table {{.Names}}\t{{.Image}}\t{{.Status}}"
echo ""
PORTAL_PORT="${PORTAL_PORT:-8080}"
echo "Logs: docker logs -f vire-portal"
echo "Health: curl http://localhost:$PORTAL_PORT/health"
