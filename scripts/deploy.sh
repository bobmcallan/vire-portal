#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
COMPOSE_DIR="$PROJECT_DIR/docker"

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
    COMPOSE_FILES="-f $COMPOSE_DIR/docker-compose.yml -f $COMPOSE_DIR/docker-compose.dev.yml"

    # Extract version info from [vire-portal] section
    VERSION="dev"
    # Note: local builds use MM-DD-HH-MM-SS format; CI uses YYYYMMDDHHmmss (intentional difference)
    BUILD_TS=$(date +"%m-%d-%H-%M-%S")
    if [ -f "$PROJECT_DIR/.version" ]; then
        VERSION=$(awk '/^\[vire-portal\]/{found=1;next} /^\[/{found=0} found && /^version:/{gsub(/version:\s*/,""); gsub(/ /,""); print; exit}' "$PROJECT_DIR/.version")
        # Update build timestamp in [vire-portal] section
        sed -i '/^\[vire-portal\]/,/^\[/{s/^build:.*/build: '"$BUILD_TS"'/}' "$PROJECT_DIR/.version"
    fi
    GIT_COMMIT=$(git -C "$PROJECT_DIR" rev-parse --short HEAD 2>/dev/null || echo "unknown")

    # Validate version is not empty (would produce invalid docker tags)
    if [ -z "$VERSION" ]; then
        echo "Error: version is empty after extraction from .version"
        exit 1
    fi

    export VERSION BUILD=$BUILD_TS GIT_COMMIT

    # Smart rebuild check
    NEEDS_REBUILD=false
    if [ "$FORCE" = true ]; then
        NEEDS_REBUILD=true
    elif [ ! -f "$COMPOSE_DIR/.last_build" ]; then
        NEEDS_REBUILD=true
    else
        if find "$PROJECT_DIR" -name "*.go" -newer "$COMPOSE_DIR/.last_build" 2>/dev/null | grep -q . || \
           find "$PROJECT_DIR/pages" -newer "$COMPOSE_DIR/.last_build" 2>/dev/null | grep -q . || \
           [ "$PROJECT_DIR/go.mod" -nt "$COMPOSE_DIR/.last_build" ] || \
           [ "$PROJECT_DIR/go.sum" -nt "$COMPOSE_DIR/.last_build" ] || \
           [ "$PROJECT_DIR/config/vire-portal.toml.docker" -nt "$COMPOSE_DIR/.last_build" ] || \
           [ "$PROJECT_DIR/docker/docker-compose.dev.yml" -nt "$COMPOSE_DIR/.last_build" ] || \
           [ "$PROJECT_DIR/docker/Dockerfile" -nt "$COMPOSE_DIR/.last_build" ] || \
           [ "$PROJECT_DIR/docker/Dockerfile.mcp" -nt "$COMPOSE_DIR/.last_build" ] || \
           [ "$PROJECT_DIR/.version" -nt "$COMPOSE_DIR/.last_build" ]; then
            NEEDS_REBUILD=true
        fi
    fi

    if [ "$NEEDS_REBUILD" = true ]; then
        echo "Building vire stack v$VERSION (commit: $GIT_COMMIT)..."
        # Stop any ghcr container first (different compose file)
        docker compose -f "$COMPOSE_DIR/docker-compose.ghcr.yml" down --remove-orphans 2>/dev/null || true
        # Build new images while old containers keep running
        if [ "$FORCE" = true ]; then
            docker image rm vire-portal:latest 2>/dev/null || true
            docker compose $COMPOSE_FILES build --no-cache
        else
            docker compose $COMPOSE_FILES build
        fi
        touch "$COMPOSE_DIR/.last_build"
        echo " Images built "
    else
        echo "No changes detected, skipping rebuild."
    fi

    # Start or recreate containers with latest images
    docker compose $COMPOSE_FILES up -d --force-recreate --remove-orphans
    ;;
  ghcr)
    echo "Deploying ghcr images with auto-update..."
    # Stop any local-build container first (different compose file)
    docker compose -f "$COMPOSE_DIR/docker-compose.yml" down --remove-orphans 2>/dev/null || true
    # Pull new images and swap containers in one step
    docker compose -f "$COMPOSE_DIR/docker-compose.ghcr.yml" up --pull always -d --force-recreate --remove-orphans
    ;;
  down)
    echo "Stopping all vire containers..."
    docker compose -f "$COMPOSE_DIR/docker-compose.yml" -f "$COMPOSE_DIR/docker-compose.dev.yml" down --remove-orphans 2>/dev/null || true
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
docker ps --filter "name=vire" --format "table {{.Names}}\t{{.Image}}\t{{.Status}}"
echo ""
PORTAL_PORT="${PORTAL_PORT:-4241}"
echo "Logs: docker logs -f vire-portal"
echo "Health: curl http://localhost:$PORTAL_PORT/api/health"
