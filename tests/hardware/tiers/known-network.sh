#!/usr/bin/env bash
# Drive the KnownNetwork lifecycle on the DUT's REAL station radio against an
# EXTERNAL AP, through the spiderw CLI, against a real iwd 3.12.
#
# Connecting to a PSK network makes iwd SAVE a known-network profile - its own
# object with autoconnect / type / last-connected, which OUTLIVES a disconnect
# and is removed only by Forget. The hwsim known-network tier proves this with a
# self-hosted AP on a second radio; a real DUT has one radio, so this drives it
# against a pre-existing external AP (the lab router). Nothing else exercises the
# KnownNetwork interface on real hardware.
#
# The chain is the assertion: connect SAVES it; it shows up with the right
# properties; it SURVIVES a disconnect; autoconnect round-trips AND its change
# subscription fires; Forget removes it. Asserts and exits non-zero on the first
# failure.
#
# The target AP is supplied by env (no AP is created here):
#   SSID / PASSPHRASE / SECURITY(psk|open) / SCAN_TRIES   (as in connect.sh)
set -uo pipefail

# shellcheck source=tests/hardware/tiers/common.sh
. "$(dirname "${BASH_SOURCE[0]}")/common.sh"

SSID="${SSID:-}"
PASSPHRASE="${PASSPHRASE:-}"
SECURITY="${SECURITY:-psk}"
SCAN_TRIES="${SCAN_TRIES:-15}"

MON=/tmp/known-monitor.log

dump_state() {
    echo "--- known-network status (all) ---"
    spiderw known-network status 2>&1 | sed 's/^/  /' || true
    echo "--- autoconnect monitor capture ---"
    [ -f "$MON" ] && sed 's/^/  /' "$MON" || echo "  (none)"
}
fail() { echo "[hw-known] FAIL: $*" >&2; dump_state >&2; exit 1; }
step() { echo; echo "== $* =="; }

[ "$SSID" != "" ] || fail "SSID is required (the external AP to connect to)"
if [ "$SECURITY" = "psk" ] && [ "$PASSPHRASE" = "" ]; then
    fail "SECURITY=psk needs PASSPHRASE (or set SECURITY=open)"
fi

# in_known_list SSID: 0 if the SSID appears as a known-network name. `list`
# prints "Name<TAB>Path" per profile, so match the first tab-field exactly.
in_known_list() { spiderw known-network list | cut -f1 | grep -Fxq "$1"; }

# --- the single real station device (waits for iwd to enumerate it) ---------
STA=$(resolve_sta) \
    || fail "iwd never presented a wireless device (rfkill soft-block? wlan0 taken by NetworkManager? radio not up?)"
echo "[hw-known] STA=$STA  SSID=$SSID  SECURITY=$SECURITY"

# Fresh iwd per container means no saved profiles, but be defensive: forget any
# stale '$SSID' so the save we assert below is a real one, not a leftover.
if in_known_list "$SSID"; then
    echo "[hw-known] stale '$SSID' profile present; forgetting it first"
    spiderw known-network "$SSID" forget || true
fi

step "device $STA mode station"
spiderw device "$STA" mode station || fail "$STA -> station mode"

STA_PATH=$(sta_path "$STA")
[ "$STA_PATH" != "" ] || fail "could not resolve the device path for $STA"
# --- connect (this SAVES the known network); scan + connect + retry via the ---
# --- shared helper (a real-RF connect can transiently fail) -----------------
NET=$(connect_sta "$STA" "$STA_PATH") \
    || fail "could not connect to $SSID (scan/connect retries exhausted)"
echo "[hw-known] connected to $SSID"

# --- the connect must have SAVED the profile with sane properties -----------
step "known-network list / status after connect"
in_known_list "$SSID" || fail "$SSID not in known-network list after connect"
status=$(spiderw known-network "$SSID" status) || fail "known-network status failed"
echo "$status" | sed 's/^/  /'
echo "$status" | grep -qiE "Name:.*$SSID" || fail "status Name is not $SSID"
if [ "$SECURITY" = "open" ]; then
    echo "$status" | grep -qiE "Type:.*open" || fail "status Type is not open"
else
    echo "$status" | grep -qiE "Type:.*psk" || fail "status Type is not psk"
fi
echo "$status" | grep -qiE "AutoConnect:.*true" || fail "AutoConnect not true by default"

# --- it must OUTLIVE a disconnect -------------------------------------------
step "station $STA disconnect; profile must persist"
spiderw station "$STA" disconnect || fail "disconnect failed"
in_known_list "$SSID" || fail "$SSID vanished from known networks after disconnect"
echo "[hw-known] profile survived disconnect"

# --- autoconnect round-trip, with the change subscription watching ----------
# The hwsim tier STOPS its AP here so autoconnect=true cannot race a reconnect.
# The external AP can't be stopped, so instead we drop autoconnect to false at
# teardown (below) to avoid an auto-reconnect race on Forget; the round-trip
# assertion itself is unaffected by whether the station reconnects.
step "monitor autoconnect while toggling it"
: >"$MON"
spiderw known-network "$SSID" monitor autoconnect >"$MON" 2>&1 &
mon_pid=$!
sleep 2   # let it print the initial autoconnect= line and establish the sub

set_ac() {   # VALUE - set autoconnect, then poll the read until it settles
    # iwd applies AutoConnect asynchronously: the Set reply can precede the value
    # update, so trust the read, not the set, and poll until it settles.
    spiderw known-network "$SSID" autoconnect "$1" >/dev/null 2>&1 \
        || fail "set autoconnect $1 failed"
    local got=""
    for ((j = 0; j < 10; j++)); do
        got=$(spiderw known-network "$SSID" autoconnect 2>/dev/null)
        [ "$got" = "$1" ] && return 0
        sleep 1
    done
    fail "autoconnect still '$got' after setting $1 (never settled)"
}
set_ac false
set_ac true

sleep 1
kill "$mon_pid" 2>/dev/null; wait "$mon_pid" 2>/dev/null
echo "== autoconnect monitor capture =="; sed 's/^/  /' "$MON"
grep -q "autoconnect=false" "$MON" \
    || fail "monitor never reported autoconnect=false (the change sub never fired)"
echo "[hw-known] autoconnect round-tripped and the change subscription fired"

# --- Forget removes it ------------------------------------------------------
# Drop autoconnect and ensure disconnected first, so the still-present external
# AP cannot auto-reconnect the station into a Forget race (the hwsim tier avoids
# this by stopping its AP instead).
spiderw known-network "$SSID" autoconnect false >/dev/null 2>&1 || true
if [ "$(spiderw network "$NET" connected 2>/dev/null)" = "true" ]; then
    spiderw station "$STA" disconnect || true
fi
step "known-network $SSID forget"
spiderw known-network "$SSID" forget || fail "forget failed"
in_known_list "$SSID" && fail "$SSID still in known-network list after forget"
spiderw known-network "$SSID" status >/dev/null 2>&1 \
    && fail "known-network status still resolves $SSID after forget (not removed)"
echo "[hw-known] profile forgotten and gone"

echo
echo "[hw-known] PASS"
