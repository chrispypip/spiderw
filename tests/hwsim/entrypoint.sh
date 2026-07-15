#!/usr/bin/env bash
# Bring up a system D-Bus and iwd inside the container, then exec the command.
#
# Deliberately does NOT use systemctl. systemd-as-PID-1 in a container is fragile
# and slow; starting dbus and iwd directly is deterministic and fast, and it is
# what an automated run wants. The container runs its own system bus (in its own
# mount namespace), so iwd owns net.connman.iwd here without touching the host.
set -euo pipefail

log() { echo "[entrypoint] $*"; }

# wait_bus_name NAME - poll the system bus until NAME is owned (10s), 0 on success.
wait_bus_name() {
    local name="$1" _
    for _ in $(seq 1 50); do
        if dbus-send --system --print-reply --dest=org.freedesktop.DBus \
             /org/freedesktop/DBus org.freedesktop.DBus.ListNames 2>/dev/null \
             | grep -q "$name"; then
            return 0
        fi
        sleep 0.2
    done
    return 1
}

# --- system D-Bus -----------------------------------------------------------
mkdir -p /run/dbus
# A container built without systemd usually ships an EMPTY /etc/machine-id (a
# placeholder systemd would fill at first boot). dbus-uuidgen --ensure refuses to
# fix an empty-but-present file, so write a fresh id when it is missing or empty,
# and point dbus's other lookup path at it.
if [ ! -s /etc/machine-id ]; then
    dbus-uuidgen > /etc/machine-id
fi
mkdir -p /var/lib/dbus
ln -sf /etc/machine-id /var/lib/dbus/machine-id
dbus-daemon --system --fork
log "system dbus started"

# --- hwsim medium controller (optional) -------------------------------------
# The roam tier needs per-link RSSI control, which means iwd's hwsim tool taking
# over the mac80211_hwsim medium and owning net.connman.hwsim. Start it BEFORE
# iwd, matching iwd's own autotests. Gated on HWSIM_MEDIUM so the read-only and
# connect tiers, which do not need it, do not depend on it starting.
if [ "${HWSIM_MEDIUM:-}" = "1" ]; then
    HWSIM_LOG=/tmp/hwsim.log
    /usr/bin/hwsim >"$HWSIM_LOG" 2>&1 &
    if ! wait_bus_name net.connman.hwsim; then
        log "ERROR: net.connman.hwsim never appeared (hwsim medium controller)"
        sed 's/^/    /' "$HWSIM_LOG" || true
        exit 1
    fi
    log "net.connman.hwsim is up (medium controller)"
fi

# --- iwd --------------------------------------------------------------------
# The daemon is at /usr/libexec/iwd. It needs NET_ADMIN (passed via
# `docker run --cap-add NET_ADMIN`) to manage the phys, which it sees through the
# shared network namespace (`--network host`). IWD_DEBUG=1 turns on -d so the log
# shows CQM/RSSI/roam decisions - the roam tier sets it to diagnose roams.
IWD_LOG=/tmp/iwd.log
iwd_args=()
if [ "${IWD_DEBUG:-}" = "1" ]; then
    iwd_args+=(-d)
fi
/usr/libexec/iwd "${iwd_args[@]}" >"$IWD_LOG" 2>&1 &

if ! wait_bus_name net.connman.iwd; then
    log "ERROR: net.connman.iwd never appeared on the system bus"
    log "iwd log follows:"
    sed 's/^/    /' "$IWD_LOG" || true
    log "visible wireless phys (should be the host's mac80211_hwsim radios):"
    iw dev 2>&1 | sed 's/^/    /' || true
    exit 1
fi
log "net.connman.iwd is up"

# Hand off. iwd keeps running as a child; the container tears it down on exit.
exec "$@"
