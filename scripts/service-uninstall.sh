#!/bin/bash
set -euo pipefail

# Uninstall script for vire-portal systemd service

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
BIN_DIR="$PROJECT_ROOT/bin"
SERVICE_NAME="vire-portal"

VERBOSE=false
PURGE=false

while [[ $# -gt 0 ]]; do
    case $1 in
        -v|--verbose) VERBOSE=true; shift ;;
        --purge) PURGE=true; shift ;;
        -h|--help)
            echo "Usage: $0 [options]"
            echo "Options:"
            echo "  --purge    Also remove binaries and PID file"
            echo "  -v, --verbose  Show verbose output"
            echo "  -h, --help     Show this help message"
            exit 0
            ;;
        *) echo "Unknown option: $1"; exit 1 ;;
    esac
done

if [[ $EUID -ne 0 ]]; then
    echo "This script must be run as root (use sudo)"
    exit 1
fi

echo "=== vire-portal Service Uninstaller ==="
echo ""

# Stop the service if running
if systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
    echo "Stopping $SERVICE_NAME service..."
    systemctl stop "$SERVICE_NAME"
    echo "  Service stopped"
else
    echo "Service is not running"
fi

# Disable the service if enabled
if systemctl is-enabled --quiet "$SERVICE_NAME" 2>/dev/null; then
    echo "Disabling $SERVICE_NAME service..."
    systemctl disable "$SERVICE_NAME"
    echo "  Service disabled"
else
    echo "Service is not enabled"
fi

# Remove the systemd service file
if [ -f "/etc/systemd/system/${SERVICE_NAME}.service" ]; then
    echo "Removing systemd service file..."
    rm -f "/etc/systemd/system/${SERVICE_NAME}.service"
    systemctl daemon-reload
    echo "  Service file removed"
else
    echo "No systemd service file found"
fi

# Optionally purge binaries and PID file
if [[ "$PURGE" == "true" ]]; then
    echo ""
    echo "Purging binaries and PID file..."

    PID_FILE="$BIN_DIR/vire-portal.pid"
    if [ -f "$PID_FILE" ]; then
        rm -f "$PID_FILE"
        echo "  Removed: $PID_FILE"
    fi

    if [ -f "$BIN_DIR/vire-portal" ]; then
        rm -f "$BIN_DIR/vire-portal"
        echo "  Removed: $BIN_DIR/vire-portal"
    fi
fi

echo ""
echo "=== Uninstall Complete ==="
echo ""
echo "To reinstall, run: sudo $SCRIPT_DIR/service-install.sh"
echo ""
