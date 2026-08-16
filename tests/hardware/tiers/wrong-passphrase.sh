#!/usr/bin/env bash
# Prove spiderw surfaces a FAILED connect correctly on the Pi's REAL brcmfmac
# station against an EXTERNAL AP, against a real iwd 3.12.
#
# Every other connect asserts success; this asserts the FAILURE path, where the
# pure-Go mock is most likely to be more forgiving than iwd - and where a REAL
# AP's real 4-way handshake rejection differs from hwsim (iwd-as-both-ends). With
# a wrong (but valid-length) passphrase the AP rejects the station and iwd's
# Network.Connect returns an error. The test asserts spiderw:
#   1. FAILS the connect (non-zero exit) rather than a phantom success,
#   2. surfaces a non-empty error (iwd's failure mapped through, not swallowed),
#   3. leaves the station DISCONNECTED, and
#   4. RECOVERS - a follow-up connect with the CORRECT passphrase succeeds, so
#      the failed attempt did not wedge iwd.
#
# The target AP (PSK) is supplied by env; PASSPHRASE is its CORRECT passphrase:
#   SSID / PASSPHRASE / SCAN_TRIES   (as in connect.sh)
#   BAD_PASSPHRASE  a valid-length wrong passphrase (default "wrong-passphrase")
set -uo pipefail

# shellcheck source=tests/hardware/tiers/common.sh
. "$(dirname "${BASH_SOURCE[0]}")/common.sh"

SSID="${SSID:-}"
PASSPHRASE="${PASSPHRASE:-}"
# Valid length (>=8, so iwd accepts the FORMAT and attempts the handshake) but
# not the AP's passphrase - this must fail AT the handshake, not be rejected for
# being malformed.
BAD_PASSPHRASE="${BAD_PASSPHRASE:-wrong-passphrase}"
SCAN_TRIES="${SCAN_TRIES:-15}"
SETTLE_TRIES="${SETTLE_TRIES:-10}"

dump_iwd_log() {
    [ -f /tmp/iwd.log ] || return 0
    echo "--- iwd log (connect/handshake lines) ---"
    grep -iE 'connect|handshake|4.?way|mic|deauth|eapol|fail|abort' /tmp/iwd.log \
        | tail -n 30 | sed 's/^/  /'
}
fail() { echo "[hw-wrongpsk] FAIL: $*" >&2; dump_iwd_log >&2; exit 1; }
step() { echo; echo "== $* =="; }

[ "$SSID" != "" ] || fail "SSID is required (the external AP to connect to)"
[ "$PASSPHRASE" != "" ] || fail "PASSPHRASE (the AP's CORRECT passphrase) is required"

# --- the single real station device (waits for iwd to enumerate it) ---------
STA=$(resolve_sta) \
    || fail "iwd never presented a wireless device (rfkill soft-block? wlan0 taken by NetworkManager? radio not up?)"
echo "[hw-wrongpsk] STA=$STA  SSID=$SSID"

step "device $STA mode station"
spiderw device "$STA" mode station || fail "$STA -> station mode"

STA_PATH=$(sta_path "$STA")
[ "$STA_PATH" != "" ] || fail "could not resolve the device path for $STA"
net_path() {
    spiderw network list \
      | awk -F'\t' -v ssid="$SSID" -v pfx="$STA_PATH/" \
            '$1 == ssid && index($2, pfx) == 1 { print $2; exit }'
}

NET=""
for ((i = 0; i < SCAN_TRIES; i++)); do
    step "station $STA scan (try $i/$SCAN_TRIES)"
    spiderw station "$STA" scan
    NET=$(net_path)
    [ "$NET" != "" ] && break
    sleep 2
done
[ "$NET" != "" ] || fail "station $STA never saw SSID $SSID after $SCAN_TRIES scans"

connected_now() { spiderw network "$NET" connected 2>/dev/null; }

# --- 1+2. connect with the WRONG passphrase: must fail, with an error --------
step "network $SSID connect with the WRONG passphrase (must fail)"
if out=$(spiderw network "$NET" connect --passphrase="$BAD_PASSPHRASE" 2>&1); then
    echo "$out"
    fail "connect SUCCEEDED with the wrong passphrase (iwd or the mapping is too forgiving)"
fi
echo "[hw-wrongpsk] connect failed as expected; spiderw reported:"
echo "$out" | sed 's/^/  /'
[ "$out" != "" ] || fail "connect failed but spiderw surfaced an EMPTY error"

# --- 3. the station must be left DISCONNECTED -------------------------------
state=""
for ((i = 0; i < SETTLE_TRIES; i++)); do
    state=$(connected_now)
    [ "$state" = "false" ] && break
    sleep 1
done
[ "$state" = "false" ] \
    || fail "station is connected=$state after a failed connect (want false)"
echo "[hw-wrongpsk] station left disconnected after the failed connect"

# --- 4. RECOVER: the correct passphrase must now connect --------------------
step "network $SSID connect with the CORRECT passphrase (must succeed)"
spiderw network "$NET" connect --passphrase="$PASSPHRASE" \
    || fail "connect with the correct passphrase failed after the wrong-passphrase attempt"
state=""
for ((i = 0; i < SETTLE_TRIES; i++)); do
    state=$(connected_now)
    [ "$state" = "true" ] && break
    sleep 1
done
[ "$state" = "true" ] || fail "not connected=$state with the correct passphrase (want true)"
echo "[hw-wrongpsk] recovered: correct passphrase connected after the wrong one failed"

spiderw station "$STA" disconnect >/dev/null 2>&1 || true

echo
echo "[hw-wrongpsk] PASS (wrong passphrase failed cleanly with an error and no"
echo "              phantom connect; correct passphrase then recovered)"
