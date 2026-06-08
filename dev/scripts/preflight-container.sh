#!/usr/bin/env bash
#-------------------------------------------------------------------------------
# spiderw - preflight-container.sh
#
# Description:  Ensures the development container is set up and configured
#               properly for development and running unit/integration tests.
# Usage:
#   ./preflight-container.sh
#
# This script is expected to run inside the development container.
#
#-------------------------------------------------------------------------------
set -euo pipefail

echo "=== spiderw Container Preflight Check ==="

# Ensure DBUS_SESSION_BUS_ADDRESS is available
if [[ -z "${DBUS_SESSION_BUS_ADDRESS:-}" ]]; then
    echo "[ERROR] DBUS_SESSION_BUS_ADDRESS is not set."
    echo "       Make sure you are inside the container *after* entrypoint.sh"
    echo "       (entrypoint.sh uses dbus-run-session to set it)"
    exit 1
fi

echo "[OK] DBUS_SESSION_BUS_ADDRESS = $DBUS_SESSION_BUS_ADDRESS"

sleep 1

# Basic bus connectivity test
echo "[INFO] checking DBus connectivity..."
if ! dbus-send --session --print-reply \
  --dest=org.freedesktop.DBus --type=method_call \
  / org.freedesktop.DBus.ListNames &>/dev/null; then
    echo "[ERROR] Cannot communicate with DBus session bus."
    exit 1
fi
echo "[OK] DBus session bus responding"


# ----------------------------------------------------------
# Start Go mock (background)
# ----------------------------------------------------------
echo "[INFO] Starting Go iwd mock..."
go run /workspace/tools/test-mocks/iwdmock &
MOCK_PID=$!
cleanup() {
    if [[ -n "${MOCK_PID:-}" ]]; then
        kill "$MOCK_PID" 2>/dev/null | true
    fi
}
trap cleanup EXIT
sleep 6

# Ensure it is still alive
if ! kill -0 "$MOCK_PID" 2>/dev/null; then
    echo "[ERROR] iwd mock crashed during startup"
    exit 1
fi
echo "[OK] iwd mock running (pid $MOCK_PID)"


# ----------------------------------------------------------
# Verify mock registered net.connman.iwd
# ----------------------------------------------------------
echo "[INFO] Checking DBus name registration..."

if dbus-send --session --print-reply \
    --dest=org.freedesktop.DBus \
    / org.freedesktop.DBus.GetNameOwner \
    string:net.connman.iwd >/dev/null 2>&1; then
    echo "[OK] net.connman.iwd registered on DBus"
else
    echo "[ERROR] iwd mock did not register DBus name net.connman.iwd"
    kill "$MOCK_PID" 2>/dev/null || true
    exit 1
fi

# ----------------------------------------------------------
# Preflight succeeded
# ----------------------------------------------------------
echo "=== Preflight Completed Successfully ==="
echo "[INFO] iwd mock PID: $MOCK_PID"
echo "[INFO] DBus session working"
echo "[INFO] You're ready to develop!"
