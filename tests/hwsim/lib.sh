#!/usr/bin/env bash
# Shared harness for running the hwsim tiers NATIVELY on a VM - no Docker.
#
# Background: the tiers were first driven inside a container (see Dockerfile +
# entrypoint.sh), where `docker run --rm` gave each tier a clean slate for free:
# a fresh system D-Bus, a fresh pinned iwd, an empty /var/lib/iwd, and every
# radio back in its default mode. That container path is still used AGAINST REAL
# HARDWARE on the Pi (tests/hardware/run.sh). But mac80211_hwsim would not work
# under Docker on the test VMs, so on the VM we run the tiers directly and this
# library reproduces that per-run cleanliness itself.
#
# The key idea: run.sh invokes ONE tier per call, and each call reloads the
# radios and starts a fresh iwd, so the CALL is the isolation boundary - exactly
# what `docker run --rm` was. A fresh module means fresh phys in the default
# station mode, so no device-mode state can leak between tiers; wiping
# /var/lib/iwd means no known-network or stored-AP profile can; a fresh `hwsim`
# medium process means no leftover fade rule can. iwd's log lands at the same
# /tmp/iwd.log the tiers already read for diagnostics, so the tier scripts need
# no changes between the container and native paths.
#
# Requires root (iwd manages the phys and owns net.connman.iwd; the state wipe
# and modprobe need it too) - run.sh re-execs under sudo. Assumes a running
# system D-Bus (a normal systemd VM has one) and that no OTHER wireless manager
# (NetworkManager, a packaged iwd service) is holding the hwsim interfaces; the
# VM image is provisioned that way and iwd_start stops a packaged iwd defensively.
set -uo pipefail

HWSIM_MODULE=mac80211_hwsim
IWD_LOG=/tmp/iwd.log
HWSIM_LOG=/tmp/hwsim.log
IWD_STATE_DIR=/var/lib/iwd
# PIDs of the processes this library starts, killed by harness_down.
IWD_PID=""
HWSIM_PID=""

log() { echo "[harness] $*"; }
fail() { echo "[harness] ERROR: $*" >&2; exit 1; }

# Locate the iwd daemon. A source install (matching the container's pinned build)
# lands in libexec; distro packages vary, so fall back through the usual spots.
find_iwd() {
    local p
    for p in /usr/libexec/iwd /usr/lib/iwd/iwd /usr/sbin/iwd; do
        [ -x "$p" ] && { echo "$p"; return 0; }
    done
    command -v iwd 2>/dev/null && return 0
    return 1
}

# The hwsim medium-control tool (net.connman.hwsim), built when iwd is configured
# --enable-hwsim. Only the roam and ordered-networks tiers need it.
find_hwsim_tool() {
    local p
    for p in /usr/bin/hwsim /usr/libexec/hwsim; do
        [ -x "$p" ] && { echo "$p"; return 0; }
    done
    command -v hwsim 2>/dev/null && return 0
    return 1
}

# wait_bus_name NAME [tries] - poll the system bus until NAME is owned (default
# ~10s at 0.2s intervals). Returns 0 once present, 1 on timeout.
wait_bus_name() {
    local name="$1" tries="${2:-50}" _
    for ((i = 0; i < tries; i++)); do
        if dbus-send --system --print-reply --dest=org.freedesktop.DBus \
             /org/freedesktop/DBus org.freedesktop.DBus.ListNames 2>/dev/null \
             | grep -q "$name"; then
            return 0
        fi
        sleep 0.2
    done
    return 1
}

# Reload the virtual radios so every run starts from identical, default-mode
# phys. -r first clears a stale module from a crashed run that could otherwise
# pin the wrong radio count. Wait for the netdevs to appear before returning.
hwsim_reload() {
    local radios="$1" _
    command -v modprobe >/dev/null || fail "modprobe not found"
    log "reloading $HWSIM_MODULE radios=$radios"
    modprobe -r "$HWSIM_MODULE" 2>/dev/null || true
    modprobe "$HWSIM_MODULE" "radios=$radios" \
        || fail "modprobe $HWSIM_MODULE radios=$radios failed"
    for ((i = 0; i < 25; i++)); do
        [ "$(iw dev 2>/dev/null | grep -c Interface)" -ge "$radios" ] && return 0
        sleep 0.2
    done
    fail "only $(iw dev 2>/dev/null | grep -c Interface) of $radios radios came up"
}

# Start a fresh iwd (and, when HWSIM_MEDIUM=1, the hwsim medium controller
# first, as iwd's own autotests do). Flags mirror entrypoint.sh: IWD_DEBUG -> -d,
# IWD_EXPERIMENTAL -> -E. Wipes the state dir so no saved profile survives, and
# stops a packaged iwd service so it cannot fight ours for the phys and the bus
# name. Logs go where the tiers look for them.
iwd_start() {
    local iwd_bin
    iwd_bin=$(find_iwd) || fail "iwd daemon not found (is it installed on the VM?)"

    # A distro may ship iwd as an enabled service; stop it so our controlled,
    # version-pinned instance is the only one on net.connman.iwd.
    systemctl stop iwd 2>/dev/null || true
    pkill -x iwd 2>/dev/null || true

    # Fresh state: no known networks, no stored AP profiles from a prior tier.
    rm -rf "${IWD_STATE_DIR:?}/"* 2>/dev/null || true
    mkdir -p "$IWD_STATE_DIR"

    if [ "${HWSIM_MEDIUM:-}" = "1" ]; then
        local hwsim_bin
        hwsim_bin=$(find_hwsim_tool) \
            || fail "hwsim tool not found (build iwd --enable-hwsim on the VM)"
        "$hwsim_bin" >"$HWSIM_LOG" 2>&1 &
        HWSIM_PID=$!
        if ! wait_bus_name net.connman.hwsim; then
            sed 's/^/    /' "$HWSIM_LOG" 2>/dev/null || true
            fail "net.connman.hwsim never appeared (medium controller)"
        fi
        log "net.connman.hwsim is up (medium controller)"
    fi

    local args=()
    [ "${IWD_DEBUG:-}" = "1" ] && args+=(-d)
    [ "${IWD_EXPERIMENTAL:-}" = "1" ] && args+=(-E)
    "$iwd_bin" "${args[@]}" >"$IWD_LOG" 2>&1 &
    IWD_PID=$!

    if ! wait_bus_name net.connman.iwd; then
        log "net.connman.iwd never appeared; iwd log follows:"
        sed 's/^/    /' "$IWD_LOG" 2>/dev/null || true
        log "wireless phys iwd should see:"
        iw dev 2>&1 | sed 's/^/    /' || true
        fail "iwd did not come up on the system bus"
    fi
    log "net.connman.iwd is up (pid $IWD_PID, $iwd_bin ${args[*]})"
    if ! kill -0 "$IWD_PID" 2>/dev/null; then
        log "iwd disappeared immediately after D-Bus registration"
        cat "$IWD_LOG" >&2
        fail "iwd exited after startup"
    fi
}

# Teardown, safe to call more than once (run.sh traps it on EXIT). Stops the
# processes this library started and unloads the module, so a REUSED VM is left
# as clean as a fresh one - a fresh VM discards all of it anyway.
harness_down() {
    [ "$IWD_PID" != "" ] && kill "$IWD_PID" 2>/dev/null || true
    [ "$HWSIM_PID" != "" ] && kill "$HWSIM_PID" 2>/dev/null || true
    if [ "$IWD_PID" != "" ]; then
        kill "$IWD_PID" 2>/dev/null || true
        wait "$IWD_PID" 2>/dev/null || true
    fi
    if [ "$HWSIM_PID" != "" ]; then
        kill "$HWSIM_PID" 2>/dev/null || true
        wait "$HWSIM_PID" 2>/dev/null || true
    fi
    pkill -x iwd 2>/dev/null || true
    pkill -x hwsim 2>/dev/null || true
    IWD_PID=""; HWSIM_PID=""
    modprobe -r "$HWSIM_MODULE" 2>/dev/null || true
}

cleanup() {
    rc=$?

    if [ "$rc" -ne 0 ]; then
        echo
        echo "===== iwd log ====="
        if [ -f "$IWD_LOG" ]; then
            cat "$IWD_LOG"
        else
            echo "(no $IWD_LOG)"
        fi

        echo
        echo "===== hwsim log ====="
        if [ -f "$HWSIM_LOG" ]; then
            cat "$HWSIM_LOG"
        else
            echo "(no $HWSIM_LOG)"
        fi

        echo
        echo "===== processes ====="
        ps -ef | grep -E '[i]wd|[h]wsim' || true

        echo
        echo "===== D-Bus owners ====="
        busctl --system list --no-pager \
            | grep -E 'net\.connman\.(iwd|hwsim)' || true
    fi

    harness_down
    exit "$rc"
}
