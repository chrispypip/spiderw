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

# iwd creates a Network object per station device. Any OTHER radio left in
# station mode (the roam tier needs three radios, and this tier claims only two)
# scans on its own and produces a second object with the same SSID, which makes
# an SSID reference ambiguous. So resolve the network belonging to OUR station
# and address it by path.
STA_PATH=$(spiderw device list | awk -F'\t' -v d="$STA" '$1 == d {print $2}')
[ -n "$STA_PATH" ] || fail "could not resolve the device path for $STA"

net_path() {
    spiderw network list \
      | awk -F'\t' -v ssid="$SSID" -v pfx="$STA_PATH/" \
            '$1 == ssid && index($2, pfx) == 1 { print $2; exit }'
}

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

NET=""
for try in $(seq 1 "$SCAN_TRIES"); do
    step "station $STA scan (try $try/$SCAN_TRIES)"
    spiderw station "$STA" scan || true
    # Only count a hit under OUR station: another station seeing the SSID says
    # nothing about whether this one can connect to it.
    NET=$(net_path)
    [ -n "$NET" ] && break
    echo "[connect] $SSID not visible to $STA yet"
    sleep 1
done
[ -n "$NET" ] || fail "station $STA never saw SSID $SSID after $SCAN_TRIES scans"
echo "[connect] network object under $STA: $NET"

# --- connect ----------------------------------------------------------------
step "network $SSID connect"
spiderw network "$NET" connect --passphrase="$PASSPHRASE" || fail "connect failed"

# The one assertion that matters: iwd reports the network connected.
connected=$(spiderw network "$NET" connected)
[ "$connected" = "true" ] || fail "network $SSID connected=$connected (want true)"
echo "[connect] connected to $SSID"
spiderw station "$STA" status || true

# --- disconnect (exercise the teardown write path too) ----------------------
step "station $STA disconnect"
spiderw station "$STA" disconnect || fail "disconnect failed"
connected=$(spiderw network "$NET" connected)
[ "$connected" = "false" ] || fail "still connected after disconnect (connected=$connected)"
echo "[connect] disconnected from $SSID"

echo
echo "[connect] PASS"
