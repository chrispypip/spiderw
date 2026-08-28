#!/usr/bin/env bash
# Exercise station connect-hidden on the DUT's REAL station radio against a
# truly HIDDEN external AP, against a real iwd 3.12.
#
# ConnectHiddenNetwork is a distinct path from Network.Connect (the connect
# tier): iwd runs a DIRECTED probe scan for the named SSID - which a hidden AP
# does not advertise in its beacons - and connects to whatever answers. The
# hwsim tier could not actually hide an AP (iwd AP mode has no
# broadcast-suppression), so it fell back to a visible AP and only drove the
# method. Here the external AP is a REAL hidden one (hostapd
# ignore_broadcast_ssid on the mt76), so this drives the real hidden-beacon path
# AND can confirm the SSID is genuinely absent from a broadcast scan first.
#
# Asserts: the SSID is NOT in a broadcast scan (it is hidden), then connect-hidden
# connects. If iwd rejects connect-hidden as not-hidden/visible, that is the
# feasibility wall - the AP is probably not actually configured hidden.
#
# Requires the AP configured HIDDEN (hostapd ignore_broadcast_ssid=1). Env:
#   SSID / PASSPHRASE / SECURITY(psk|open) / SCAN_TRIES
#   CONNECT_TRIES  directed-probe retries (default 4)
#   SETTLE_TRIES   seconds to confirm connected (default 10)
set -uo pipefail

# shellcheck source=tests/hardware/tiers/common.sh
. "$(dirname "${BASH_SOURCE[0]}")/common.sh"

SSID="${SSID:-}"
PASSPHRASE="${PASSPHRASE:-}"
SECURITY="${SECURITY:-psk}"
SCAN_TRIES="${SCAN_TRIES:-5}"
CONNECT_TRIES="${CONNECT_TRIES:-6}"
SETTLE_TRIES="${SETTLE_TRIES:-10}"

dump_iwd_log() {
    [ -f /tmp/iwd.log ] || return 0
    echo "--- iwd log (hidden/probe/connect lines) ---"
    grep -iE 'hidden|probe|directed|connect|scan' /tmp/iwd.log \
        | tail -n 30 | sed 's/^/  /'
}
fail() { echo "[hw-hidden] FAIL: $*" >&2; dump_iwd_log >&2; exit 1; }
step() { echo; echo "== $* =="; }

[ "$SSID" != "" ] || fail "SSID is required (the hidden external AP)"
if [ "$SECURITY" = "psk" ] && [ "$PASSPHRASE" = "" ]; then
    fail "SECURITY=psk needs PASSPHRASE (or set SECURITY=open)"
fi

# --- the single real station device (waits for iwd to enumerate it) ---------
STA=$(resolve_sta) \
    || fail "iwd never presented a wireless device (rfkill? wlan0 taken by NetworkManager? radio not up?)"
echo "[hw-hidden] STA=$STA  SSID=$SSID  SECURITY=$SECURITY"

step "device $STA mode station"
spiderw device "$STA" mode station || fail "$STA -> station mode"

STA_PATH=$(sta_path "$STA")
[ "$STA_PATH" != "" ] || fail "could not resolve the device path for $STA"
net_path() {
    spiderw network list \
      | awk -F'\t' -v ssid="$SSID" -v pfx="$STA_PATH/" \
            '$1 == ssid && index($2, pfx) == 1 { print $2; exit }'
}
in_broadcast() {
    spiderw station "$STA" networks 2>/dev/null | cut -f1 | grep -Fxq "$SSID"
}

# --- confirm the AP is actually HIDDEN (SSID absent from a broadcast scan) ---
step "broadcast scan: confirm $SSID is hidden (should be ABSENT)"
# iwd auto-scans, so an explicit Scan often collides ("operation already in
# progress") - harmless, a scan IS running. Suppress it and wait for a populated
# result; break as soon as the list has entries. A hidden AP never broadcasts
# its SSID, so one good scan is enough to confirm absence.
for ((i = 0; i < SCAN_TRIES; i++)); do
    spiderw station "$STA" scan >/dev/null 2>&1 || true
    sleep 2
    [ -n "$(spiderw station "$STA" networks 2>/dev/null | awk 'NF{print; exit}')" ] && break
done
echo "-- broadcast networks --"; spiderw station "$STA" networks | sed 's/^/  /'
nbcast=$(spiderw station "$STA" networks 2>/dev/null | awk 'NF' | wc -l)
if in_broadcast; then
    echo "[hw-hidden] WARNING: '$SSID' IS in the broadcast scan - the AP is not"
    echo "            configured hidden (hostapd ignore_broadcast_ssid); iwd may"
    echo "            then reject connect-hidden as not-hidden."
elif [ "$nbcast" -ge 1 ]; then
    echo "[hw-hidden] confirmed hidden: '$SSID' absent from a broadcast of $nbcast networks"
else
    echo "[hw-hidden] NOTE: broadcast scan returned no networks; cannot confirm"
    echo "            hidden here - connect-hidden below is the real assertion."
fi

# --- connect-hidden (iwd's directed probe) ----------------------------------
# Do NOT rely on a broadcast result (there isn't one); connect-hidden triggers
# iwd's own directed probe. Retry: the directed probe can miss a beacon, like the
# connect tier's scan loop.
step "station $STA connect-hidden $SSID"
connected=false
for ((i = 0; i < CONNECT_TRIES; i++)); do
    echo "-- connect-hidden try $((i + 1))/$CONNECT_TRIES --"
    if [ "$SECURITY" = "open" ]; then
        out=$(spiderw station "$STA" connect-hidden "$SSID" 2>&1) \
            && { echo "$out"; connected=true; break; }
    else
        out=$(spiderw station "$STA" connect-hidden "$SSID" --passphrase="$PASSPHRASE" 2>&1) \
            && { echo "$out"; connected=true; break; }
    fi
    echo "$out"
    # Distinguish a genuine feasibility wall from a retryable miss. iwd returns
    # NotHidden / AlreadyProvisioned when a network with this SSID is already
    # VISIBLE or KNOWN (the AP is not actually hidden) - fatal. But a directed
    # probe that finds nothing returns "Object not found" (NotFound), which is
    # transient - it raced iwd's autoconnect scan or missed a beacon - and must
    # RETRY. Match ONLY the CamelCase iwd error names and the contiguous phrases,
    # so "...not found ... hidden network" no longer trips this (the old greedy
    # `*not*hidden*` did, mis-killing the retryable case = the intermittent FAIL).
    case "$out" in
    *NotHidden* | *"not hidden"* | *AlreadyProvisioned* | *"already provisioned"*)
        fail "iwd rejected connect-hidden as not-hidden/visible: the AP is not actually hidden (set hostapd ignore_broadcast_ssid=1)" ;;
    esac
    sleep 2
done
[ "$connected" = true ] || fail "connect-hidden did not succeed after $CONNECT_TRIES tries"

# --- assert it actually connected -------------------------------------------
NET=""
for ((i = 0; i < SETTLE_TRIES; i++)); do
    NET=$(net_path)
    [ "$NET" != "" ] && break
    sleep 1
done
[ "$NET" != "" ] || fail "no network object for $SSID after connect-hidden"
state=$(spiderw network "$NET" connected)
[ "$state" = "true" ] || fail "network $SSID connected=$state (want true)"
echo "[hw-hidden] connected to $SSID via connect-hidden (directed probe)"
spiderw station "$STA" status || true

spiderw station "$STA" disconnect >/dev/null 2>&1 || true

echo
echo "[hw-hidden] PASS (ConnectHiddenNetwork drove a directed-probe connect to a hidden AP)"
