#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
CONFIG_DIR="$PROJECT_DIR/config"
BIN_DIR="$PROJECT_DIR/bin"
PID_FILE="$BIN_DIR/vire-portal.pid"

# Read port from config or default
PORT="${VIRE_SERVER_PORT:-4241}"

stop_server() {
    if [ ! -f "$PID_FILE" ]; then
        echo "No PID file found."
        return 0
    fi

    OLD_PID=$(cat "$PID_FILE")

    if kill -0 "$OLD_PID" 2>/dev/null; then
        # Try graceful HTTP shutdown first
        echo "Requesting graceful shutdown..."
        if curl -sf -X POST "http://localhost:$PORT/api/shutdown" --max-time 5 > /dev/null 2>&1; then
            for i in $(seq 1 10); do
                if ! kill -0 "$OLD_PID" 2>/dev/null; then
                    echo "Server stopped gracefully."
                    rm -f "$PID_FILE"
                    return 0
                fi
                sleep 0.5
            done
        fi

        # Fallback to SIGTERM
        echo "Sending SIGTERM..."
        kill "$OLD_PID" 2>/dev/null || true
        sleep 2
    fi

    rm -f "$PID_FILE"
}

case "${1:-start}" in
  start)
    # Stop existing instance
    stop_server

    # Ensure config exists
    if [ ! -f "$CONFIG_DIR/vire-portal.toml" ]; then
        echo "Creating config/vire-portal.toml from example..."
        cp "$CONFIG_DIR/vire-portal.toml.example" "$CONFIG_DIR/vire-portal.toml"
    fi

    # Extract version info
    VERSION="dev"
    BUILD_TS=$(date +"%m-%d-%H-%M-%S")
    if [ -f "$PROJECT_DIR/.version" ]; then
        VERSION=$(grep "^version:" "$PROJECT_DIR/.version" | sed 's/version:\s*//' | tr -d ' ')
    fi
    GIT_COMMIT=$(git -C "$PROJECT_DIR" rev-parse --short HEAD 2>/dev/null || echo "unknown")

    # Build
    LDFLAGS="-s -w \
        -X 'github.com/bobmcallan/vire-portal/internal/config.Version=${VERSION}' \
        -X 'github.com/bobmcallan/vire-portal/internal/config.Build=${BUILD_TS}' \
        -X 'github.com/bobmcallan/vire-portal/internal/config.GitCommit=${GIT_COMMIT}'"

    echo "Building vire-portal v$VERSION (commit: $GIT_COMMIT)..."
    mkdir -p "$BIN_DIR"
    cd "$PROJECT_DIR"
    go build -ldflags="$LDFLAGS" -o "$BIN_DIR/vire-portal" ./cmd/vire-portal

    # Stage self-contained bin directory
    cp "$CONFIG_DIR/vire-portal.toml" "$BIN_DIR/vire-portal.toml"
    rm -rf "$BIN_DIR/pages"
    cp -r "$PROJECT_DIR/pages" "$BIN_DIR/pages"
    if [ -d "$PROJECT_DIR/data" ]; then
        mkdir -p "$BIN_DIR/data"
        cp -f "$PROJECT_DIR/data"/*.json "$BIN_DIR/data/" 2>/dev/null || true
    fi

    # Start detached from bin directory
    cd "$BIN_DIR"
    nohup ./vire-portal > /dev/null 2>&1 &
    SERVER_PID=$!
    echo "$SERVER_PID" > "$PID_FILE"

    sleep 1
    if kill -0 "$SERVER_PID" 2>/dev/null; then
        echo "vire-portal v$VERSION running (PID $SERVER_PID)"
        echo "  http://localhost:$PORT"
        echo "  http://localhost:$PORT/api/health"
        echo "  Stop: ./scripts/run.sh stop"
    else
        echo "vire-portal failed to start"
        rm -f "$PID_FILE"
        exit 1
    fi
    ;;
  stop)
    stop_server
    ;;
  restart)
    stop_server
    exec "$0" start
    ;;
  status)
    if [ -f "$PID_FILE" ] && kill -0 "$(cat "$PID_FILE")" 2>/dev/null; then
        echo "vire-portal running (PID $(cat "$PID_FILE"))"
        curl -sf "http://localhost:$PORT/api/version" 2>/dev/null || true
    else
        echo "vire-portal not running"
        rm -f "$PID_FILE" 2>/dev/null
    fi
    ;;
  *)
    echo "Usage: ./scripts/run.sh [start|stop|restart|status]"
    exit 1
    ;;
esac
