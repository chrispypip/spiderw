#!/usr/bin/env bash
# Prove spiderw completes a WPS (WSC) push-button enrollment on real hardware: the
# station JOINS a WPS-registrar AP with NO passphrase entered - WPS delivers the
# credentials. This is the coverage hwsim could NOT give: hwsim has no WPS
# registrar (iwd AP mode is not one, and the image ships no hostapd), so the hwsim
# wsc tier can only initiate + cancel. Here a REAL hostapd AP runs WPS PBC (an
# autonomous wps_pbc loop on the AP host), so the enrollment actually completes.
#
# spiderw's WSC is enrollee-side: `station <dev> wsc push-button` initiates a PBC
# enrollment and BLOCKS until iwd reports the result; on success iwd provisions
# the network and connects. This tier runs push-button, then asserts the station
# connected to the WPS AP's SSID - the connection is the authoritative proof the
# enrollment completed.
#
# Requires a WPS-PBC AP up on the AP host with wps_pbc active. Env:
#   SSID          the WPS AP's SSID (to verify the join; WPS delivers the psk)
#   WPS_TIMEOUT   seconds to wait for the enrollment (default 60)
#   SETTLE_TRIES  seconds to confirm connected afterwards (default 15)
set -uo pipefail

# shellcheck source=tests/hardware/tiers/common.sh
. "$(dirname "${BASH_SOURCE[0]}")/common.sh"

SSID="${SSID:-}"
WPS_TIMEOUT="${WPS_TIMEOUT:-60}"
SETTLE_TRIES="${SETTLE_TRIES:-15}"
SCAN_TRIES="${SCAN_TRIES:-10}"
# iwd's WSC PushButton runs its own scan and will not start while another scan is
# in progress (autoconnect scans right after entering station mode), so retry a
# few times to catch an idle window.
WPS_TRIES="${WPS_TRIES:-6}"
WSC_LOG=/tmp/wsc-pb.log

dump_state() {
    echo "--- wsc push-button output ---"
    [ -f "$WSC_LOG" ] && sed 's/^/  /' "$WSC_LOG" || echo "  (none)"
    [ -f /tmp/iwd.log ] || return 0
    echo "--- iwd log (wsc/wps/enroll lines) ---"
    grep -iE 'wsc|wps|simpleconfig|push.?button|enroll|credential|eapol' /tmp/iwd.log \
        | tail -n 30 | sed 's/^/  /'
}
fail() { echo "[hw-wps] FAIL: $*" >&2; dump_state >&2; exit 1; }
step() { echo; echo "== $* =="; }

[ "$SSID" != "" ] || fail "SSID is required (the WPS AP's SSID, to verify the join)"

# --- the single real station device (waits for iwd to enumerate it) ---------
STA=$(resolve_sta) \
    || fail "iwd never presented a wireless device (rfkill? wlan0 taken by NetworkManager? radio not up?)"
echo "[hw-wps] STA=$STA  SSID=$SSID (enroll via WPS push-button, no passphrase)"

step "device $STA mode station"
spiderw device "$STA" mode station || fail "$STA -> station mode"
STA_PATH=$(sta_path "$STA")
[ "$STA_PATH" != "" ] || fail "could not resolve the device path for $STA"

# --- seed a scan so iwd has the PBC AP in its results -----------------------
# (also settles the initial autoconnect scan so PushButton's own scan can start).
step "scan so iwd sees the WPS AP $SSID"
for ((i = 0; i < SCAN_TRIES; i++)); do
    spiderw station "$STA" scan >/dev/null 2>&1 || true
    sleep 2
    [ "$(net_path_for "$STA_PATH" "$SSID")" != "" ] && break
done

# --- run the push-button enrollment (retry across scan-busy windows) --------
# `wsc push-button` blocks until iwd reports the result; timeout guards a stuck
# session. It fails to START if a scan is in progress, so retry. The connection
# check is the authoritative proof - the command exit is only a hint.
NET=""
enrolled=false
for ((k = 0; k < WPS_TRIES; k++)); do
    step "station $STA wsc push-button (try $((k + 1))/$WPS_TRIES, <=${WPS_TIMEOUT}s)"
    : >"$WSC_LOG"
    timeout "$WPS_TIMEOUT" spiderw station "$STA" wsc push-button >"$WSC_LOG" 2>&1
    rc=$?
    sed 's/^/  /' "$WSC_LOG"
    [ "$rc" -eq 124 ] && echo "[hw-wps] note: hit the ${WPS_TIMEOUT}s timeout"
    for ((i = 0; i < SETTLE_TRIES; i++)); do
        NET=$(net_path_for "$STA_PATH" "$SSID")
        [ "$NET" != "" ] \
            && [ "$(spiderw network "$NET" connected 2>/dev/null)" = "true" ] \
            && { enrolled=true; break; }
        sleep 1
    done
    [ "$enrolled" = true ] && break
    # "failed starting" is a scan-busy race; give it a moment and retry.
    grep -qi 'failed starting' "$WSC_LOG" && sleep 3
done
if [ "$enrolled" != true ]; then
    echo "[hw-wps] not connected to $SSID after WPS push-button." >&2
    echo "  Is the WPS AP up with wps_pbc active (the WPS AP + pbc loop running)?" \
         "A PBC session overlap (two enrollees at once) also aborts it." >&2
    fail "WPS enrollment did not result in a connection"
fi
echo "[hw-wps] connected to $SSID via WPS (no passphrase entered)"

# --- clean up: WPS saved a known network; forget it so the next run re-enrolls
spiderw station "$STA" disconnect >/dev/null 2>&1 || true
spiderw known-network "$SSID" forget >/dev/null 2>&1 || true

echo
echo "[hw-wps] PASS (WPS push-button enrollment completed against a real registrar)"
