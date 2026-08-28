#!/usr/bin/env bash
# Connect the DUT's REAL station radio to an EXTERNAL access point through the
# spiderw CLI, against a real iwd 3.12.
#
# The hwsim connect tier self-hosts an AP on a second radio; a real DUT has ONE
# radio and cannot, so this drives the STATION half against a pre-existing
# external AP (the lab router). It exercises the write paths that matter most on
# the real driver/firmware: mode station, scan, Network.Connect with a
# passphrase, and disconnect. A fullmac DUT (e.g. brcmfmac) is firmware-driven,
# where iwd behaves differently from hwsim's softmac - exactly the divergence
# worth testing.
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

# shellcheck source=tests/hardware/tiers/common.sh
. "$(dirname "${BASH_SOURCE[0]}")/common.sh"

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

# --- the single real station device (waits for iwd to enumerate it) ---------
# The DUT has one radio; take the first named device as the station.
STA=$(resolve_sta) \
    || fail "iwd never presented a wireless device (rfkill soft-block? wlan0 taken by NetworkManager? radio not up?)"
echo "[hw-connect] STA=$STA  SSID=$SSID  SECURITY=$SECURITY"

step "device $STA mode station"
spiderw device "$STA" mode station || fail "could not set $STA to station mode"

# Resolve the network under OUR station and address it by path, so the ref is
# unambiguous (matches how the hwsim connect tier scopes its network).
STA_PATH=$(sta_path "$STA")
[ "$STA_PATH" != "" ] || fail "could not resolve the device path for $STA"

# --- scan + connect (retried) via the shared helper -------------------------
# The chain that matters: scan sees the AP, connect drives mode station -> scan
# -> Network.Connect, and the station reaches the connected state. connect_sta
# does all three (and retries a transient RF connect); a real failure here is
# the AP off / out of range / a wrong passphrase / the station being rejected.
NET=$(connect_sta "$STA" "$STA_PATH") \
    || fail "could not connect to $SSID (AP off, out of range, wrong passphrase, or rejected?)"
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
