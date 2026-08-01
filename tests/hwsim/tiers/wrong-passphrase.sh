#!/usr/bin/env bash
# Prove spiderw surfaces a FAILED connect correctly, against a REAL iwd 3.12.
#
# Every other connect in the suite uses the right passphrase and asserts success.
# This asserts a FAILURE path, which is where the pure-Go mock is most likely to
# be more forgiving than iwd: with a wrong (but valid-length) passphrase the AP's
# 4-way handshake rejects the station, and iwd's Network.Connect returns an error.
# The test asserts spiderw:
#   1. FAILS the connect (non-zero exit) rather than reporting a phantom success,
#   2. surfaces a non-empty error (iwd's failure mapped through, not swallowed),
#   3. leaves the station DISCONNECTED, and
#   4. RECOVERS - a follow-up connect with the correct passphrase succeeds, so the
#      failed attempt did not wedge iwd into a stuck state.
#
# iwd is both the AP (authenticator) and the station here, so the handshake is
# really enforced. Asserts and exits non-zero on the first failure. Two radios.
set -uo pipefail

SSID="${SSID:-spiderw-wrongpsk}"
GOOD_PASSPHRASE="${GOOD_PASSPHRASE:-spiderw-secret}"
# Valid length (>=8, so iwd accepts the FORMAT and actually attempts the
# handshake) but not the AP's passphrase - this must fail at the handshake, not
# be rejected for being malformed.
BAD_PASSPHRASE="${BAD_PASSPHRASE:-wrong-passphrase}"
SCAN_TRIES="${SCAN_TRIES:-10}"
SETTLE_TRIES="${SETTLE_TRIES:-10}"

dump_iwd_log() {
    [ -f /tmp/iwd.log ] || return 0
    echo "--- iwd log (connect/handshake lines) ---"
    grep -iE 'connect|handshake|4.?way|mic|deauth|eapol|fail|abort' /tmp/iwd.log \
        | tail -n 30 | sed 's/^/  /'
}
fail() { echo "[wrongpsk] FAIL: $*" >&2; dump_iwd_log >&2; exit 1; }
step() { echo; echo "== $* =="; }

# --- devices: one AP, one station -------------------------------------------
mapfile -t DEVICES < <(spiderw device list | cut -f1 | grep -v '^/' | awk 'NF')
[ "${#DEVICES[@]}" -ge 2 ] || fail "need >=2 named devices, saw: ${DEVICES[*]:-none}"
AP="${DEVICES[0]}"; STA="${DEVICES[1]}"
echo "[wrongpsk] AP=$AP  STA=$STA  SSID=$SSID"

# --- bring up the AP with the GOOD passphrase -------------------------------
step "device $AP mode ap; access-point start $SSID"
spiderw device "$AP" mode ap || fail "$AP -> ap mode"
spiderw access-point "$AP" start "$SSID" "$GOOD_PASSPHRASE" || fail "AP start"

# --- station: scan until the AP is visible ----------------------------------
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
    NET=$(net_path); [ -n "$NET" ] && break
    sleep 1
done
[ -n "$NET" ] || fail "station never saw $SSID after $SCAN_TRIES scans"

connected_now() { spiderw network "$NET" connected 2>/dev/null; }

# --- 1+2. connect with the WRONG passphrase: must fail, with an error --------
step "network $SSID connect with the WRONG passphrase (must fail)"
if out=$(spiderw network "$NET" connect --passphrase="$BAD_PASSPHRASE" 2>&1); then
    echo "$out"
    fail "connect SUCCEEDED with the wrong passphrase (iwd or the mapping is too forgiving)"
fi
echo "[wrongpsk] connect failed as expected; spiderw reported:"
echo "$out" | sed 's/^/  /'
[ -n "$out" ] || fail "connect failed but spiderw surfaced an EMPTY error"

# --- 3. the station must be left DISCONNECTED -------------------------------
# iwd may take a beat to settle back to disconnected after the failed handshake.
state=""
for _ in $(seq 1 "$SETTLE_TRIES"); do
    state=$(connected_now)
    [ "$state" = "false" ] && break
    sleep 1
done
[ "$state" = "false" ] \
    || fail "station is connected=$state after a failed connect (want false)"
echo "[wrongpsk] station left disconnected after the failed connect"

# --- 4. RECOVER: the correct passphrase must now connect --------------------
# Proves the failed attempt did not wedge iwd; the same network is still usable.
step "network $SSID connect with the CORRECT passphrase (must succeed)"
spiderw network "$NET" connect --passphrase="$GOOD_PASSPHRASE" \
    || fail "connect with the correct passphrase failed after the wrong-passphrase attempt"
state=""
for _ in $(seq 1 "$SETTLE_TRIES"); do
    state=$(connected_now)
    [ "$state" = "true" ] && break
    sleep 1
done
[ "$state" = "true" ] || fail "not connected=$state with the correct passphrase (want true)"
echo "[wrongpsk] recovered: correct passphrase connected after the wrong one failed"

spiderw station "$STA" disconnect >/dev/null 2>&1 || true

echo
echo "[wrongpsk] PASS (wrong passphrase failed cleanly with an error and no phantom"
echo "           connect; correct passphrase then recovered)"
