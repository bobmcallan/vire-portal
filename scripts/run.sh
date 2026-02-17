#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
BIN_DIR="$PROJECT_DIR/bin"
CONFIG_DIR="$PROJECT_DIR/config"
PID_FILE="$BIN_DIR/vire-portal.pid"
NGINX_CONF="$CONFIG_DIR/nginx.conf"
NGINX_PID_FILE="/tmp/vire-nginx.pid"
NGINX_HASH_FILE="/tmp/vire-nginx.hash"

# Read port from config or default
PORT="${VIRE_SERVER_PORT:-8500}"

# Calculate config hash
get_nginx_hash() {
    md5sum "$NGINX_CONF" 2>/dev/null | awk '{print $1}'
}

# Check if nginx config changed
nginx_config_changed() {
    if [ ! -f "$NGINX_HASH_FILE" ]; then
        return 0
    fi
    local current_hash
    current_hash=$(get_nginx_hash)
    local saved_hash
    saved_hash=$(cat "$NGINX_HASH_FILE")
    [ "$current_hash" != "$saved_hash" ]
}

stop_server() {
    if [ ! -f "$PID_FILE" ]; then
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

stop_nginx() {
    if [ ! -f "$NGINX_PID_FILE" ]; then
        return 0
    fi

    PID=$(cat "$NGINX_PID_FILE")
    if kill -0 "$PID" 2>/dev/null; then
        echo "Stopping nginx (PID $PID)..."
        kill "$PID" 2>/dev/null || true
        rm -f "$NGINX_PID_FILE"
        sleep 1
    fi
}

start_nginx() {
    if [ -f "$NGINX_PID_FILE" ]; then
        PID=$(cat "$NGINX_PID_FILE")
        if kill -0 "$PID" 2>/dev/null; then
            if nginx_config_changed; then
                echo "Nginx config changed, restarting..."
                stop_nginx
            else
                echo "Nginx already running, config unchanged"
                return 0
            fi
        fi
    fi

    echo "Starting nginx..."
    nginx -c "$NGINX_CONF"
    get_nginx_hash > "$NGINX_HASH_FILE"
    echo "Nginx started on port 8881"
}

case "${1:-start}" in
  start)
    # Start nginx first
    start_nginx

    # Stop existing instance
    stop_server

    # Build binaries, configs, and assets to bin/
    "$SCRIPT_DIR/build.sh"

    # Start detached from bin directory
    cd "$BIN_DIR"
    nohup ./vire-portal > /dev/null 2>&1 &
    SERVER_PID=$!
    echo "$SERVER_PID" > "$PID_FILE"

    sleep 1
    if kill -0 "$SERVER_PID" 2>/dev/null; then
        echo ""
        echo "vire-portal running (PID $SERVER_PID)"
        echo "  http://localhost:$PORT"
        echo "  http://localhost:$PORT/api/health"
        echo ""
        echo "Access via nginx:"
        echo "  http://localhost:8881/"
        echo "  http://localhost:8881/api/"
        echo ""
        echo "Stop: ./scripts/run.sh stop"
    else
        echo "vire-portal failed to start"
        rm -f "$PID_FILE"
        exit 1
    fi
    ;;
  stop)
    stop_server
    stop_nginx
    echo "Stopped"
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

    if [ -f "$NGINX_PID_FILE" ] && kill -0 "$(cat "$NGINX_PID_FILE")" 2>/dev/null; then
        echo "nginx running (PID $(cat "$NGINX_PID_FILE"))"
    else
        echo "nginx not running"
    fi
    ;;
  *)
    echo "Usage: ./scripts/run.sh [start|stop|restart|status]"
    exit 1
    ;;
esac
