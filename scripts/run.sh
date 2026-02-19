#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
BIN_DIR="$PROJECT_DIR/bin"
CONFIG_DIR="$PROJECT_DIR/config"
PID_FILE="$BIN_DIR/vire-portal.pid"

# Extract port from TOML config file
get_port_from_config() {
    local config_file="$1"
    if [[ -f "$config_file" ]]; then
        grep -E '^port\s*=' "$config_file" | head -1 | sed 's/port\s*=\s*//' | tr -d '"' | tr -d "'"
    else
        echo ""
    fi
}

# Get host from TOML config file
get_host_from_config() {
    local config_file="$1"
    if [[ -f "$config_file" ]]; then
        grep -E '^host\s*=' "$config_file" | head -1 | sed 's/host\s*=\s*//' | tr -d '"' | tr -d "'" || echo "localhost"
    else
        echo "localhost"
    fi
}

stop_server() {
    local port="$1"
    if [ ! -f "$PID_FILE" ]; then
        return 0
    fi

    OLD_PID=$(cat "$PID_FILE")

    if kill -0 "$OLD_PID" 2>/dev/null; then
        # Try graceful HTTP shutdown first
        echo "Requesting graceful shutdown..."
        if curl -sf -X POST "http://localhost:$port/api/shutdown" --max-time 5 > /dev/null 2>&1; then
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
    # Build binaries to bin/
    "$SCRIPT_DIR/build.sh"

    # Copy config files to bin/
    echo "Copying configs..."
    cp "$CONFIG_DIR/vire-portal.toml" "$BIN_DIR/"
    cp "$CONFIG_DIR/vire-mcp.toml" "$BIN_DIR/"

    # Read port from config (for shutdown and display)
    PORT=$(get_port_from_config "$BIN_DIR/vire-portal.toml")
    if [[ -z "$PORT" ]]; then
        PORT="${VIRE_SERVER_PORT:-8881}"
    fi

    HOST=$(get_host_from_config "$BIN_DIR/vire-portal.toml")
    if [[ -z "$HOST" ]] || [[ "$HOST" == "0.0.0.0" ]]; then
        HOST="localhost"
    fi

    # Stop existing instance
    stop_server "$PORT"

    # Ensure logs directory exists
    mkdir -p "$BIN_DIR/logs"

    # Start detached from bin directory
    cd "$BIN_DIR"
    nohup ./vire-portal > /dev/null 2>&1 &
    SERVER_PID=$!
    echo "$SERVER_PID" > "$PID_FILE"

    sleep 1
    if kill -0 "$SERVER_PID" 2>/dev/null; then
        echo ""
        echo "vire-portal running (PID $SERVER_PID)"
        echo "  http://$HOST:$PORT"
        echo "  http://$HOST:$PORT/api/health"
        echo ""
        echo "Stop: ./scripts/run.sh stop"
    else
        echo "vire-portal failed to start"
        rm -f "$PID_FILE"
        exit 1
    fi
    ;;
  stop)
    PORT=$(get_port_from_config "$BIN_DIR/vire-portal.toml")
    if [[ -z "$PORT" ]]; then
        PORT="${VIRE_SERVER_PORT:-8881}"
    fi
    stop_server "$PORT"
    echo "Stopped"
    ;;
  restart)
    PORT=$(get_port_from_config "$BIN_DIR/vire-portal.toml")
    if [[ -z "$PORT" ]]; then
        PORT="${VIRE_SERVER_PORT:-8881}"
    fi
    stop_server "$PORT"
    exec "$0" start
    ;;
  status)
    PORT=$(get_port_from_config "$BIN_DIR/vire-portal.toml")
    if [[ -z "$PORT" ]]; then
        PORT="${VIRE_SERVER_PORT:-8881}"
    fi
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
