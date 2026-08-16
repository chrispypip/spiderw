#!/usr/bin/env bash
# Connect the Pi's REAL brcmfmac station to an EXTERNAL access point through the
# spiderw CLI, against a real iwd 3.12.
#
# The hwsim connect tier self-hosts an AP on a second radio; the Pi has ONE radio
# and cannot, so this drives the STATION half against a pre-existing external AP
# (the lab router). It exercises the write paths that matter most on the real
# driver/firmware: mode station, scan, Network.Connect with a passphrase, and
# disconnect. brcmfmac is fullmac (firmware-driven), where iwd behaves
# differently from hwsim's softmac - exactly the divergence worth testing.
#
# The chain is one assertion: if the station reaches (and leaves) the connected
# state against a real AP, every write path in between worked on real hardware.
#
# The target AP is supplied by env (no AP is created here):
#   SSID        the network to connect to (required)
#   PASSPHRASE  its PSK passphrase (required unless SECURITY=open)
#   SECURITY    psk (default) | open
#   SCAN_TRIES  scans to wait for the AP to appear (default 15)
set -uo pipefail

SSID="${SSID:-}"
PASSPHRASE="${PASSPHRASE:-}"
SECURITY="${SECURITY:-psk}"
SCAN_TRIES="${SCAN_TRIES:-15}"

fail() { echo "[hw-connect] FAIL: $*" >&2; exit 1; }
step() { echo; echo "== $* =="; }

[ "$SSID" != "" ] || fail "SSID is required (the external AP to connect to)"
if [ "$SECURITY" = "psk" ] && [ "$PASSPHRASE" = "" ]; then
    fail "SECURITY=psk needs PASSPHRASE (or set SECURITY=open)"
fi

# --- the single real station device -----------------------------------------
# The Pi has one radio; take the first named device as the station. `device
# list` prints "name<TAB>path" per device, so keep the first tab-field and drop
# any bare-path lines.
mapfile -t DEVICES < <(spiderw device list | cut -f1 | grep -v '^/' | awk 'NF')
[ "${#DEVICES[@]}" -ge 1 ] || fail "no named wireless device, saw: ${DEVICES[*]:-none}"
STA="${DEVICES[0]}"
echo "[hw-connect] STA=$STA  SSID=$SSID  SECURITY=$SECURITY"

step "device $STA mode station"
spiderw device "$STA" mode station || fail "could not set $STA to station mode"

# Resolve the network under OUR station and address it by path, so the ref is
# unambiguous (matches how the hwsim connect tier scopes its network).
STA_PATH=$(spiderw device list | awk -F'\t' -v d="$STA" '$1 == d {print $2}')
[ "$STA_PATH" != "" ] || fail "could not resolve the device path for $STA"

net_path() {
    spiderw network list \
      | awk -F'\t' -v ssid="$SSID" -v pfx="$STA_PATH/" \
            '$1 == ssid && index($2, pfx) == 1 { print $2; exit }'
}

# --- scan until the external AP is visible ----------------------------------
NET=""
for ((i = 0; i < SCAN_TRIES; i++)); do
    step "station $STA scan (try $i/$SCAN_TRIES)"
    spiderw station "$STA" scan
    NET=$(net_path)
    [ "$NET" != "" ] && break
    echo "[hw-connect] $SSID not visible to $STA yet"
    sleep 2
done
[ "$NET" != "" ] \
    || fail "station $STA never saw SSID $SSID after $SCAN_TRIES scans (AP off, out of range, or wrong regdomain?)"
echo "[hw-connect] network object under $STA: $NET"

# --- connect ----------------------------------------------------------------
step "network $SSID connect"
if [ "$SECURITY" = "open" ]; then
    spiderw network "$NET" connect || fail "connect (open) failed"
else
    spiderw network "$NET" connect --passphrase="$PASSPHRASE" \
        || fail "connect failed (wrong passphrase, or AP rejected the station?)"
fi

# The one assertion that matters: iwd reports the network connected.
connected=$(spiderw network "$NET" connected)
[ "$connected" = "true" ] || fail "network $SSID connected=$connected (want true)"
echo "[hw-connect] connected to $SSID on real hardware"
spiderw station "$STA" status || true

# --- disconnect (exercise the teardown write path too) ----------------------
step "station $STA disconnect"
spiderw station "$STA" disconnect || fail "disconnect failed"
connected=$(spiderw network "$NET" connected)
[ "$connected" = "false" ] || fail "still connected after disconnect (connected=$connected)"
echo "[hw-connect] disconnected from $SSID"

echo
echo "[hw-connect] PASS"
