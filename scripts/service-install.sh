#!/bin/bash
set -euo pipefail

# Install script for vire-portal
# Builds binaries, installs vire-portal as a systemd service running from ./bin

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
BIN_DIR="$PROJECT_ROOT/bin"
SERVICE_NAME="vire-portal"
SERVICE_USER="root"

VERBOSE=false
UNINSTALL=false

while [[ $# -gt 0 ]]; do
    case $1 in
        -v|--verbose) VERBOSE=true; shift ;;
        --uninstall) UNINSTALL=true; shift ;;
        -h|--help)
            echo "Usage: $0 [options]"
            echo "Options:"
            echo "  --uninstall    Remove vire-portal service"
            echo "  -v, --verbose  Show verbose output"
            echo "  -h, --help     Show this help message"
            exit 0
            ;;
        *) echo "Unknown option: $1"; exit 1 ;;
    esac
done

uninstall() {
    echo "Uninstalling vire-portal service..."
    
    if systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
        echo "Stopping $SERVICE_NAME service..."
        systemctl stop "$SERVICE_NAME"
    fi
    
    if systemctl is-enabled --quiet "$SERVICE_NAME" 2>/dev/null; then
        echo "Disabling $SERVICE_NAME service..."
        systemctl disable "$SERVICE_NAME"
    fi
    
    if [ -f "/etc/systemd/system/${SERVICE_NAME}.service" ]; then
        echo "Removing systemd service file..."
        rm -f "/etc/systemd/system/${SERVICE_NAME}.service"
        systemctl daemon-reload
    fi
    
    echo "Uninstall complete."
    exit 0
}

if [[ "$UNINSTALL" == "true" ]]; then
    uninstall
fi

if [[ $EUID -ne 0 ]]; then
    echo "This script must be run as root (use sudo)"
    exit 1
fi

echo "=== vire-portal Service Installer ==="
echo ""

PID_FILE="$BIN_DIR/vire-portal.pid"
if [ -f "$PID_FILE" ]; then
    OLD_PID=$(cat "$PID_FILE" 2>/dev/null)
    if [ -n "$OLD_PID" ] && kill -0 "$OLD_PID" 2>/dev/null; then
        echo "Stopping vire-portal (PID $OLD_PID)..."
        kill "$OLD_PID" 2>/dev/null || true
        sleep 2
        rm -f "$PID_FILE"
    fi
fi

echo "[1/3] Building binaries..."
"$SCRIPT_DIR/build.sh"

if [ ! -f "$BIN_DIR/vire-portal" ]; then
    echo "Error: vire-portal binary not found after build"
    exit 1
fi

echo ""
echo "[2/3] Creating systemd service..."

cat > "/etc/systemd/system/${SERVICE_NAME}.service" << EOF
[Unit]
Description=Vire Portal Service
After=network.target

[Service]
Type=simple
User=$SERVICE_USER
ExecStart=$BIN_DIR/vire-portal
WorkingDirectory=$BIN_DIR
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

echo "  Created: /etc/systemd/system/${SERVICE_NAME}.service"
echo "  Binary: $BIN_DIR/vire-portal"
echo "  WorkingDir: $BIN_DIR"

echo ""
echo "[3/3] Enabling and restarting service..."

systemctl daemon-reload
systemctl enable "$SERVICE_NAME"

if systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
    echo "  Restarting $SERVICE_NAME..."
    systemctl restart "$SERVICE_NAME"
else
    echo "  Starting $SERVICE_NAME..."
    systemctl start "$SERVICE_NAME"
fi

sleep 1

if systemctl is-active --quiet "$SERVICE_NAME"; then
    echo "  Service running successfully"
else
    echo "  Warning: Service may not have started correctly"
    echo "  Check status with: systemctl status $SERVICE_NAME"
fi

echo ""
echo "=== Installation Complete ==="
echo ""
echo "Commands:"
echo "  Status:   systemctl status $SERVICE_NAME"
echo "  Stop:     systemctl stop $SERVICE_NAME"
echo "  Start:    systemctl start $SERVICE_NAME"
echo "  Restart:  systemctl restart $SERVICE_NAME"
echo "  Logs:     journalctl -u $SERVICE_NAME -f"
echo ""
