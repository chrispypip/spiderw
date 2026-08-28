#!/usr/bin/env bash
# Connect the DUT's REAL station radio to a WPA3-Personal (SAE) external AP and
# assert iwd genuinely used SAE, against a real iwd 3.12.
#
# WHY A SEPARATE TIER. The connect tier already connects with a passphrase and
# iwd picks PSK vs SAE from the AP's AKM - but iwd reports Network.Type=psk for
# BOTH WPA2-PSK and WPA3-SAE, so a plain connect cannot prove which handshake
# ran. This points at a WPA3-ONLY SSID (no PSK fallback, so a successful connect
# MUST be SAE) AND asserts iwd's log shows SAE, making the WPA3 path explicit.
#
# FINDING CONTEXT (real hw): brcmfmac + iwd WPA3-SAE WORKS against a WPA3-only
# AP. An earlier WPA2/WPA3-TRANSITION (mixed) AP failed the SAE handshake
# (reason 15) - so use a WPA3-ONLY SSID here, not a transition one. 6GHz is out
# (the brcmfmac radio has no 6GHz band); use a 2.4/5GHz WPA3 SSID.
#
# Env: SSID / PASSPHRASE / SCAN_TRIES   (as in connect.sh; PASSPHRASE required).
set -uo pipefail

# shellcheck source=tests/hardware/tiers/common.sh
. "$(dirname "${BASH_SOURCE[0]}")/common.sh"

SSID="${SSID:-}"
PASSPHRASE="${PASSPHRASE:-}"
SCAN_TRIES="${SCAN_TRIES:-15}"

dump_iwd_log() {
    [ -f /tmp/iwd.log ] || return 0
    echo "--- iwd log (sae/connect/handshake lines) ---"
    grep -iE 'sae|external_auth|handshake|connect|akm|rsn' /tmp/iwd.log \
        | tail -n 30 | sed 's/^/  /'
}
fail() { echo "[hw-wpa3] FAIL: $*" >&2; dump_iwd_log >&2; exit 1; }
step() { echo; echo "== $* =="; }

[ "$SSID" != "" ] || fail "SSID is required (the WPA3-only external AP)"
[ "$PASSPHRASE" != "" ] || fail "PASSPHRASE is required (the SAE password)"

# --- the single real station device (waits for iwd to enumerate it) ---------
STA=$(resolve_sta) \
    || fail "iwd never presented a wireless device (rfkill? wlan0 taken by NetworkManager? radio not up?)"
echo "[hw-wpa3] STA=$STA  SSID=$SSID"

step "device $STA mode station"
spiderw device "$STA" mode station || fail "$STA -> station mode"

STA_PATH=$(sta_path "$STA")
[ "$STA_PATH" != "" ] || fail "could not resolve the device path for $STA"
net_path() {
    spiderw network list \
      | awk -F'\t' -v ssid="$SSID" -v pfx="$STA_PATH/" \
            '$1 == ssid && index($2, pfx) == 1 { print $2; exit }'
}

# --- scan + connect ---------------------------------------------------------
NET=""
for ((i = 0; i < SCAN_TRIES; i++)); do
    step "station $STA scan (try $i/$SCAN_TRIES)"
    spiderw station "$STA" scan
    NET=$(net_path)
    [ "$NET" != "" ] && break
    sleep 2
done
[ "$NET" != "" ] || fail "station $STA never saw SSID $SSID after $SCAN_TRIES scans"

step "network $SSID connect (WPA3-SAE)"
spiderw network "$NET" connect --passphrase="$PASSPHRASE" \
    || fail "connect failed (SAE handshake? transition-mode AP? wrong password?)"
[ "$(spiderw network "$NET" connected)" = "true" ] || fail "not connected after connect"
echo "[hw-wpa3] connected to $SSID"

# --- assert iwd actually negotiated SAE (not PSK) ---------------------------
# On a WPA3-only AP a successful connect can only be SAE, but confirm it directly
# from iwd's log too: the connect-info line names the AKM (brcmfmac fullmac logs
# "using SAE"; a softmac station logs sae_* lines under IWD_DEBUG, which run.sh
# sets for this tier).
if grep -qi sae /tmp/iwd.log 2>/dev/null; then
    echo "[hw-wpa3] confirmed: iwd used SAE"
    grep -i sae /tmp/iwd.log | tail -n 3 | sed 's/^/  /'
else
    fail "connected, but iwd's log shows no SAE - the AP may be PSK/transition (iwd would then use PSK), not WPA3-only; point this tier at a WPA3-ONLY SSID"
fi

spiderw station "$STA" disconnect >/dev/null 2>&1 || true

echo
echo "[hw-wpa3] PASS (WPA3-SAE connect on real brcmfmac; iwd negotiated SAE)"
