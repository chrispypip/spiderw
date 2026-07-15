#!/usr/bin/env bash
# Drive the full AP + station + connect flow through the spiderw CLI,
# against a REAL iwd on mac80211_hwsim radios.
#
# Where the smoke tier (smoke.sh) only READS iwd, this WRITES: mode switch, AP
# start, scan, connect, disconnect - the paths where a mock-vs-reality gap bites.
# So unlike the read-only smoke, this ASSERTS and exits non-zero on the first
# failure. It is a test, not a report.
#
# The whole chain is one assertion: if the station ends up connected to the SSID
# the AP started, every write path in between worked against the real daemon.
set -uo pipefail

SSID="${SSID:-spiderw-hwsim}"
PASSPHRASE="${PASSPHRASE:-spiderw-secret}"
SCAN_TRIES="${SCAN_TRIES:-10}"

fail() { echo "[connect] FAIL: $*" >&2; exit 1; }
step() { echo; echo "== $* =="; }

# --- pick two devices -------------------------------------------------------
# Device names (wlanN) are stable within a boot; the object-path tail (ifindex)
# is not, so address everything by name. `device list` prints "name<TAB>path"
# per device (or just the path when a ref has no name), so keep the first
# tab-field and drop any bare-path lines. Need two: one AP, one station.
mapfile -t DEVICES < <(spiderw device list | cut -f1 | grep -v '^/' | awk 'NF')
[ "${#DEVICES[@]}" -ge 2 ] || fail "need >=2 named devices, saw: ${DEVICES[*]:-none}"
AP="${DEVICES[0]}"
STA="${DEVICES[1]}"
echo "[connect] AP=$AP  STA=$STA  SSID=$SSID"

# --- bring up the access point ----------------------------------------------
step "device $AP mode ap"
spiderw device "$AP" mode ap || fail "could not set $AP to ap mode"

step "access-point $AP start $SSID"
spiderw access-point "$AP" start "$SSID" "$PASSPHRASE" || fail "AP start failed"
spiderw access-point "$AP" status || true

# --- station: scan until the AP is visible ----------------------------------
# hwsim beacons are not always seen on the first scan, so scan-and-check in a
# loop. The station device defaults to station mode; assert it explicitly so a
# surprise mode does not read as a scan failure.
step "device $STA mode station"
spiderw device "$STA" mode station || fail "could not set $STA to station mode"

found=0
for try in $(seq 1 "$SCAN_TRIES"); do
    step "station $STA scan (try $try/$SCAN_TRIES)"
    spiderw station "$STA" scan || true
    if spiderw network list | cut -f1 | grep -qxF "$SSID"; then
        found=1
        break
    fi
    echo "[connect] $SSID not visible yet"
    sleep 1
done
[ "$found" -eq 1 ] || fail "station never saw SSID $SSID after $SCAN_TRIES scans"

# --- connect ----------------------------------------------------------------
step "network $SSID connect"
spiderw network "$SSID" connect --passphrase="$PASSPHRASE" || fail "connect failed"

# The one assertion that matters: iwd reports the network connected.
connected=$(spiderw network "$SSID" connected)
[ "$connected" = "true" ] || fail "network $SSID connected=$connected (want true)"
echo "[connect] connected to $SSID"
spiderw station "$STA" status || true

# --- disconnect (exercise the teardown write path too) ----------------------
step "station $STA disconnect"
spiderw station "$STA" disconnect || fail "disconnect failed"
connected=$(spiderw network "$SSID" connected)
[ "$connected" = "false" ] || fail "still connected after disconnect (connected=$connected)"
echo "[connect] disconnected from $SSID"

echo
echo "[connect] PASS"
