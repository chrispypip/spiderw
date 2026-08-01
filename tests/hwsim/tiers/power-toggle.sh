#!/usr/bin/env bash
# Toggle Powered on a device and its adapter against a REAL iwd 3.12.
#
# The last write path in the suite not otherwise exercised: SetPowered, on both
# the Device and the Adapter object. Powering a device off turns its radio off -
# the Device object persists with Powered=false (its Station/AP interfaces drop) -
# and powering it back on must leave it usable. The Adapter's Powered is a
# distinct write path (Adapter.SetPowered vs Device.SetPowered); powering the
# adapter off may also drop its device from the tree, so this half asserts the
# adapter property round-trips and the device is usable again afterwards, without
# depending on the exact device-object lifecycle.
#
# Asserts and exits non-zero on the first failure. Uses one device (a second is
# harmless).
set -uo pipefail

SETTLE_TRIES="${SETTLE_TRIES:-15}"

dump_state() {
    echo "--- device list ---"; spiderw device list 2>&1 | sed 's/^/  /' || true
    [ -n "${STA:-}" ] && { echo "--- device status ---"; spiderw device "$STA" status 2>&1 | sed 's/^/  /' || true; }
}
fail() { echo "[power] FAIL: $*" >&2; dump_state >&2; exit 1; }
step() { echo; echo "== $* =="; }

in_list() { spiderw device list | cut -f1 | grep -Fxq "$1"; }
dev_powered() { spiderw device "$STA" powered 2>/dev/null; }
adapter_powered() { spiderw adapter "$ADAPTER" powered 2>/dev/null; }

poll() {   # "CMD" WANT  - poll CMD until it prints WANT
    local want="$2" got=""
    for _ in $(seq 1 "$SETTLE_TRIES"); do
        got=$(eval "$1")
        [ "$got" = "$want" ] && return 0
        sleep 1
    done
    return 1
}
poll_in_list() {   # NAME present(true|false)
    for _ in $(seq 1 "$SETTLE_TRIES"); do
        if [ "$2" = true ]; then in_list "$1" && return 0
        else in_list "$1" || return 0; fi
        sleep 1
    done
    return 1
}

# --- pick a device ----------------------------------------------------------
mapfile -t DEVICES < <(spiderw device list | cut -f1 | grep -v '^/' | awk 'NF')
[ "${#DEVICES[@]}" -ge 1 ] || fail "need >=1 named device, saw: ${DEVICES[*]:-none}"
STA="${DEVICES[0]}"
echo "[power] device=$STA"

# --- 1. device Powered off -> on --------------------------------------------
step "device $STA powered off"
[ "$(dev_powered)" = "true" ] || fail "device is not powered at start (got '$(dev_powered)')"
spiderw device "$STA" powered off >/dev/null || fail "set powered off failed"
poll 'dev_powered' false || fail "device still powered=$(dev_powered) after off"
# Powering OFF is not removal: the Device object persists, just unpowered.
in_list "$STA" || fail "device vanished from the list when only powered off (should persist)"
spiderw device "$STA" status | grep -qiE "Powered:[[:space:]]*false" \
    || fail "device status does not show Powered: false"
echo "[power] device powered off; still present, Powered=false"

step "device $STA powered on"
spiderw device "$STA" powered on >/dev/null || fail "set powered on failed"
poll 'dev_powered' true || fail "device still powered=$(dev_powered) after on"
# Prove it is usable again: a scan needs a live radio.
spiderw device "$STA" mode station || fail "mode station after power-on failed"
spiderw station "$STA" scan || fail "scan after power-on failed (radio not usable again)"
echo "[power] device powered back on and usable (scan worked)"

# --- 2. adapter Powered off -> on (distinct write path) ---------------------
ADAPTER=$(spiderw device "$STA" status | sed -n 's/^Adapter:[[:space:]]*//p' | head -n1)
[ -n "$ADAPTER" ] || fail "could not resolve the adapter for $STA"
case "$ADAPTER" in /*) fail "adapter did not resolve to a name: $ADAPTER" ;; esac
echo "[power] adapter=$ADAPTER"

step "adapter $ADAPTER powered off"
[ "$(adapter_powered)" = "true" ] || fail "adapter is not powered at start (got '$(adapter_powered)')"
spiderw adapter "$ADAPTER" powered off >/dev/null || fail "adapter set powered off failed"
poll 'adapter_powered' false || fail "adapter still powered=$(adapter_powered) after off"
echo "[power] adapter powered off (Powered=false)"

step "adapter $ADAPTER powered on"
spiderw adapter "$ADAPTER" powered on >/dev/null || fail "adapter set powered on failed"
poll 'adapter_powered' true || fail "adapter still powered=$(adapter_powered) after on"
# The device may have been dropped while the adapter was off; it must return and
# be usable. (Whether iwd keeps or drops the device object when the adapter is
# off is iwd's business - this waits for the device either way.)
poll_in_list "$STA" true || fail "device $STA did not return after the adapter power cycle"
spiderw device "$STA" mode station || fail "mode station after adapter power-on failed"
spiderw station "$STA" scan || fail "scan after adapter power-on failed"
echo "[power] adapter powered back on; device returned and is usable"

echo
echo "[power] PASS (device and adapter Powered both round-tripped; device usable after each)"
