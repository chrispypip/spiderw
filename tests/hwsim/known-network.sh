#!/usr/bin/env bash
# Drive the KnownNetwork lifecycle through spiderw against a REAL iwd 3.12.
#
# Connecting to a PSK network makes iwd SAVE a known-network profile. That
# profile is its own object with autoconnect / hidden / last-connected
# properties, it OUTLIVES a disconnect, and it is removed only by Forget. Nothing
# else in the suite touches the KnownNetwork interface against a real daemon - the
# connect tier connects but never inspects the saved profile.
#
# The chain is the assertion: connect saves it; it shows up with the right
# properties; it survives a disconnect; autoconnect round-trips AND its change
# subscription fires; Forget removes it. Asserts and exits non-zero on the first
# failure. Needs two radios (one AP, one station).
set -uo pipefail

SSID="${SSID:-spiderw-known}"
PASSPHRASE="${PASSPHRASE:-spiderw-secret}"
SCAN_TRIES="${SCAN_TRIES:-10}"

MON=/tmp/known-monitor.log

dump_state() {
    echo "--- known-network status (all) ---"
    spiderw known-network status 2>&1 | sed 's/^/  /' || true
    echo "--- autoconnect monitor capture ---"
    [ -f "$MON" ] && sed 's/^/  /' "$MON" || echo "  (none)"
}
fail() { echo "[known] FAIL: $*" >&2; dump_state >&2; exit 1; }
step() { echo; echo "== $* =="; }

# in_known_list SSID: 0 if the SSID appears as a known-network name. `list` prints
# "Name<TAB>Path" per profile, so match the first tab-field exactly.
in_known_list() { spiderw known-network list | cut -f1 | grep -Fxq "$1"; }

# --- devices: one AP, one station -------------------------------------------
mapfile -t DEVICES < <(spiderw device list | cut -f1 | grep -v '^/' | awk 'NF')
[ "${#DEVICES[@]}" -ge 2 ] || fail "need >=2 named devices, saw: ${DEVICES[*]:-none}"
AP="${DEVICES[0]}"; STA="${DEVICES[1]}"
echo "[known] AP=$AP  STA=$STA  SSID=$SSID"

# Fresh iwd per container means no saved profiles, but be defensive: forget any
# stale '$SSID' so the save we assert below is a real one, not a leftover.
if in_known_list "$SSID"; then
    echo "[known] stale '$SSID' profile present; forgetting it first"
    spiderw known-network "$SSID" forget || true
fi

# --- bring up AP + connect (this SAVES the known network) -------------------
step "device $AP mode ap; access-point start $SSID"
spiderw device "$AP" mode ap || fail "$AP -> ap mode"
spiderw access-point "$AP" start "$SSID" "$PASSPHRASE" || fail "AP start"

step "device $STA mode station"
spiderw device "$STA" mode station || fail "$STA -> station mode"
STA_PATH=$(spiderw device list | awk -F'\t' -v d="$STA" '$1==d{print $2}')
[ -n "$STA_PATH" ] || fail "resolve device path for $STA"
net_path() {
    spiderw network list \
      | awk -F'\t' -v ssid="$SSID" -v pfx="$STA_PATH/" \
            '$1==ssid && index($2,pfx)==1 {print $2; exit}'
}
NET=""
for try in $(seq 1 "$SCAN_TRIES"); do
    step "station $STA scan (try $try/$SCAN_TRIES)"
    spiderw station "$STA" scan || true
    NET=$(net_path); [ -n "$NET" ] && break; sleep 1
done
[ -n "$NET" ] || fail "station never saw $SSID after $SCAN_TRIES scans"
step "network $SSID connect"
spiderw network "$NET" connect --passphrase="$PASSPHRASE" || fail "connect failed"
[ "$(spiderw network "$NET" connected)" = "true" ] || fail "not connected after connect"
echo "[known] connected to $SSID"

# --- the connect must have SAVED the profile with sane properties -----------
step "known-network list / status after connect"
in_known_list "$SSID" || fail "$SSID not in known-network list after connect"
status=$(spiderw known-network "$SSID" status) || fail "known-network status failed"
echo "$status" | sed 's/^/  /'
echo "$status" | grep -qiE "Name:.*$SSID"     || fail "status Name is not $SSID"
echo "$status" | grep -qiE "Type:.*psk"       || fail "status Type is not psk"
echo "$status" | grep -qiE "AutoConnect:.*true" || fail "AutoConnect not true by default"

# --- it must OUTLIVE a disconnect -------------------------------------------
step "station $STA disconnect; profile must persist"
spiderw station "$STA" disconnect || fail "disconnect failed"
in_known_list "$SSID" || fail "$SSID vanished from known networks after disconnect"
echo "[known] profile survived disconnect"

# --- autoconnect round-trip, with the change subscription watching ----------
# Stop the AP first so flipping autoconnect back to true does not race an
# auto-reconnect into the assertions; the saved profile is unaffected.
step "access-point $AP stop (so autoconnect=true will not reconnect)"
spiderw access-point "$AP" stop || true

step "monitor autoconnect while toggling it"
: >"$MON"
spiderw known-network "$SSID" monitor autoconnect >"$MON" 2>&1 &
mon_pid=$!
sleep 2   # let it print the initial autoconnect= line and establish the sub

set_ac() {   # VALUE
    # iwd applies AutoConnect asynchronously: the Set reply can precede the value
    # update, so an immediate read (including the CLI's own set-command echo) may
    # still show the old value. Trust neither - poll the read until it settles.
    spiderw known-network "$SSID" autoconnect "$1" >/dev/null 2>&1 \
        || fail "set autoconnect $1 failed"
    local got=""
    for _ in $(seq 1 10); do
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
echo "[known] autoconnect round-tripped and the change subscription fired"

# --- Forget removes it ------------------------------------------------------
step "known-network $SSID forget"
spiderw known-network "$SSID" forget || fail "forget failed"
in_known_list "$SSID" && fail "$SSID still in known-network list after forget"
spiderw known-network "$SSID" status >/dev/null 2>&1 \
    && fail "known-network status still resolves $SSID after forget (not removed)"
echo "[known] profile forgotten and gone"

echo
echo "[known] PASS"
