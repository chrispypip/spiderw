#!/usr/bin/env bash
# Exercise station connect-hidden against a REAL iwd 3.12.
#
# FEASIBILITY NOTE. A truly hidden AP suppresses its SSID in beacons; iwd's AP
# mode cannot do that (its .ap profiles have no broadcast-suppression option), so
# iwd alone cannot stand up a hidden AP on hwsim. What this tier CAN drive is the
# ConnectHiddenNetwork D-Bus METHOD, which is a distinct path from Network.Connect
# (the connect tier): iwd does a DIRECTED probe scan for the named SSID and
# connects to whatever answers. We start a normal AP and, WITHOUT a broad station
# scan first (so the SSID is not already sitting in scan results), call
# connect-hidden. A normal AP answers directed probes, so this should connect.
#
# If iwd instead rejects the call because the network is already visible, that is
# reported as the feasibility wall: verifying the hidden BEACON path would need a
# real hidden AP (e.g. hostapd with ignore_broadcast_ssid), which this image does
# not carry. Asserts and exits non-zero on the first failure. Two radios.
set -uo pipefail

SSID="${SSID:-spiderw-hidden}"
PASSPHRASE="${PASSPHRASE:-spiderw-secret}"
CONNECT_TRIES="${CONNECT_TRIES:-3}"
SETTLE_TRIES="${SETTLE_TRIES:-10}"

dump_iwd_log() {
    [ -f /tmp/iwd.log ] || return 0
    echo "--- iwd log (hidden/probe/connect lines) ---"
    grep -iE 'hidden|probe|directed|connect|scan' /tmp/iwd.log \
        | tail -n 30 | sed 's/^/  /'
}
fail() { echo "[hidden] FAIL: $*" >&2; dump_iwd_log >&2; exit 1; }
step() { echo; echo "== $* =="; }

# --- devices: one AP, one station -------------------------------------------
mapfile -t DEVICES < <(spiderw device list | cut -f1 | grep -v '^/' | awk 'NF')
[ "${#DEVICES[@]}" -ge 2 ] || fail "need >=2 named devices, saw: ${DEVICES[*]:-none}"
AP="${DEVICES[0]}"; STA="${DEVICES[1]}"
echo "[hidden] AP=$AP  STA=$STA  SSID=$SSID"

# --- bring up the AP (visible - the best iwd AP mode can do) -----------------
step "device $AP mode ap; access-point start $SSID"
spiderw device "$AP" mode ap || fail "$AP -> ap mode"
spiderw access-point "$AP" start "$SSID" "$PASSPHRASE" || fail "AP start"

# --- station: connect-hidden WITHOUT a broad scan first ----------------------
step "device $STA mode station"
spiderw device "$STA" mode station || fail "$STA -> station mode"
STA_PATH=$(spiderw device list | awk -F'\t' -v d="$STA" '$1==d{print $2}')
[ -n "$STA_PATH" ] || fail "resolve device path for $STA"
net_path() {
    spiderw network list \
      | awk -F'\t' -v ssid="$SSID" -v pfx="$STA_PATH/" \
            '$1==ssid && index($2,pfx)==1 {print $2; exit}'
}

# connect-hidden triggers iwd's own directed scan, so do not pre-scan. Retry a
# couple of times: the directed probe can miss on the first beacon, exactly like
# the connect tier's scan loop.
step "station $STA connect-hidden $SSID"
connected=false
for try in $(seq 1 "$CONNECT_TRIES"); do
    echo "-- connect-hidden try $try/$CONNECT_TRIES --"
    if out=$(spiderw station "$STA" connect-hidden "$SSID" \
                --passphrase="$PASSPHRASE" 2>&1); then
        echo "$out"
        connected=true
        break
    fi
    echo "$out"
    # A rejection because the network is visible is the feasibility wall, not a
    # flake - stop and say so plainly.
    case "$out" in
    *NotHidden* | *not*hidden* | *already*visible* | *AlreadyProvisioned* | *already*provisioned*)
        fail "iwd rejected connect-hidden as not-hidden/visible: a real hidden AP (hostapd) would be needed; not feasible with iwd AP mode"
        ;;
    esac
    sleep 2
done
[ "$connected" = true ] || fail "connect-hidden did not succeed after $CONNECT_TRIES tries"

# --- assert it actually connected -------------------------------------------
NET=""
for _ in $(seq 1 "$SETTLE_TRIES"); do
    NET=$(net_path); [ -n "$NET" ] && break; sleep 1
done
[ -n "$NET" ] || fail "no network object for $SSID after connect-hidden"
state=$(spiderw network "$NET" connected)
[ "$state" = "true" ] || fail "network $SSID connected=$state (want true)"
echo "[hidden] connected to $SSID via connect-hidden"
spiderw station "$STA" status || true

echo
echo "[hidden] PASS (ConnectHiddenNetwork method drove a directed-probe connect)"
