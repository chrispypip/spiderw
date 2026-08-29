#!/usr/bin/env bash
# Prove spiderw connects on 6 GHz (Wi-Fi 6E) on real hardware - a band the
# brcmfmac DUT cannot reach (no 6 GHz radio) and hwsim cannot model. The iwlwifi
# AX210 has a 6 GHz radio, so this is SOFTMAC/iwlwifi-only. 6E mandates WPA3-SAE +
# PMF, so it is a WPA3 connect that ADDITIONALLY asserts the associated channel is
# in the 6 GHz band (iwd picks SAE from the AP's AKM; the new coverage is the
# band).
#
# Point it at a 6 GHz WPA3 SSID (the lab router's 6G beacon). Env:
#   SSID / PASSPHRASE / SCAN_TRIES / CONNECT_TRIES   (as in connect.sh; WPA3)
#   MIN_6E_MHZ  6 GHz lower edge (default 5925; 5 GHz tops out ~5895)
set -uo pipefail

# shellcheck source=tests/hardware/tiers/common.sh
. "$(dirname "${BASH_SOURCE[0]}")/common.sh"

SSID="${SSID:-}"
PASSPHRASE="${PASSPHRASE:-}"
MIN_6E_MHZ="${MIN_6E_MHZ:-5925}"

dump_state() {
    [ -n "${STA:-}" ] && { echo "--- iw link ---"; iw dev "$STA" link 2>&1 | sed 's/^/  /'; }
    [ -f /tmp/iwd.log ] || return 0
    echo "--- iwd log (connect/sae/freq lines) ---"
    grep -iE 'connect|sae|freq|channel|band|6ghz|akm' /tmp/iwd.log \
        | tail -n 25 | sed 's/^/  /'
}
fail() { echo "[hw-6e] FAIL: $*" >&2; dump_state >&2; exit 1; }
step() { echo; echo "== $* =="; }

[ "$SSID" != "" ] || fail "SSID is required (a 6 GHz WPA3 SSID)"
[ "$PASSPHRASE" != "" ] || fail "PASSPHRASE is required (6E mandates WPA3-SAE)"

# --- the single real station device (waits for iwd to enumerate it) ---------
STA=$(resolve_sta) \
    || fail "iwd never presented a wireless device (rfkill? wlan0 taken by NetworkManager? radio not up?)"
echo "[hw-6e] STA=$STA  SSID=$SSID"

step "device $STA mode station"
spiderw device "$STA" mode station || fail "$STA -> station mode"
STA_PATH=$(sta_path "$STA")
[ "$STA_PATH" != "" ] || fail "could not resolve the device path for $STA"

# --- connect (WPA3-SAE), retried via the shared helper ----------------------
NET=$(connect_sta "$STA" "$STA_PATH") \
    || fail "connect to $SSID failed - 6E needs the AX210's 6 GHz radio (brcmfmac has none), WPA3-SAE, and the router's 6G beacon up"
echo "[hw-6e] connected to $SSID"

# --- assert the association is on a 6 GHz channel ---------------------------
# spiderw exposes Frequency only for a HOSTED AP, so read the station's connected
# frequency from the kernel (iw is in the image).
freq=$(iw dev "$STA" link 2>/dev/null \
    | sed -n 's/^[[:space:]]*freq:[[:space:]]*\([0-9]*\).*/\1/p' | head -n1)
echo "[hw-6e] connected frequency: ${freq:-unknown} MHz"
[ "$freq" != "" ] || fail "could not read the connected frequency (iw dev $STA link)"
[ "$freq" -ge "$MIN_6E_MHZ" ] \
    || fail "associated on ${freq} MHz, not 6 GHz (>= ${MIN_6E_MHZ}) - is $SSID a 6 GHz net? it may have landed on 2.4/5 GHz"

# 6E mandates SAE; note it from iwd's log too (informational, like the wpa3 tier).
grep -qi sae /tmp/iwd.log 2>/dev/null && echo "[hw-6e] iwd used SAE (6E is WPA3)"

spiderw station "$STA" disconnect >/dev/null 2>&1 || true

echo
echo "[hw-6e] PASS (connected on 6 GHz / Wi-Fi 6E: ${freq} MHz, WPA3)"
