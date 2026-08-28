#!/usr/bin/env bash
# Prove spiderw's SignalLevelAgent registers and delivers on the DUT's REAL
# station radio, connected to an EXTERNAL AP, against a real iwd 3.12.
#
# `station monitor-signal <dBm>...` registers a net.connman.iwd.SignalLevelAgent
# with RSSI thresholds; iwd calls it back with a band index (0 = strongest) and
# spiderw decodes that into a `level=N (range)` line. The hwsim signal tier
# proves this against virtual radios; this proves it against the real driver.
#
# WHAT THIS CAN AND CANNOT SHOW. The band index tracks the RSSI only through
# iwd's MULTI-threshold CQM monitor (NL80211_EXT_FEATURE_CQM_RSSI_LIST). Fullmac
# brcmfmac does its CQM in firmware and, like mac80211/hwsim, is not expected to
# deliver the RSSI-list events - so LIVE band tracking as the signal moves may
# not fire here either; that needs a softmac station (mt76/iwlwifi), a later
# diversity tier. What IS driver-independent, and what this asserts, is the rest
# of the path: the agent registers, iwd delivers the INITIAL callback at register
# time, and spiderw decodes it into a valid band index with the correct range
# string. Asserts and exits non-zero on the first failure.
#
# The target AP is supplied by env (no AP is created here):
#   SSID / PASSPHRASE / SECURITY(psk|open) / SCAN_TRIES   (as in connect.sh)
#   THRESHOLDS  descending dBm band edges (default "-40 -50 -60")
#   REG_TIMEOUT seconds to wait for the initial band line (default 10)
set -uo pipefail

# shellcheck source=tests/hardware/tiers/common.sh
. "$(dirname "${BASH_SOURCE[0]}")/common.sh"

SSID="${SSID:-}"
PASSPHRASE="${PASSPHRASE:-}"
SECURITY="${SECURITY:-psk}"
SCAN_TRIES="${SCAN_TRIES:-15}"
THRESHOLDS="${THRESHOLDS:--40 -50 -60}"
REG_TIMEOUT="${REG_TIMEOUT:-10}"

MON=/tmp/signal-monitor.log
read -r -a TH <<<"$THRESHOLDS"     # thresholds as an array for the range check

dump_state() {
    echo "--- monitor-signal capture ---"
    [ -f "$MON" ] && sed 's/^/  /' "$MON" || echo "  (no monitor output)"
    [ -f /tmp/iwd.log ] || return 0
    echo "--- iwd log (signal-agent lines) ---"
    grep -iE 'signal_agent|rssi_report_level|SignalLevelAgent' /tmp/iwd.log \
        | tail -n 20 | sed 's/^/  /'
}
fail() { echo "[hw-signal] FAIL: $*" >&2; dump_state >&2; exit 1; }
step() { echo; echo "== $* =="; }

[ "$SSID" != "" ] || fail "SSID is required (the external AP to connect to)"
if [ "$SECURITY" = "psk" ] && [ "$PASSPHRASE" = "" ]; then
    fail "SECURITY=psk needs PASSPHRASE (or set SECURITY=open)"
fi

# expected_range BAND: re-derive the dBm range string spiderw should print for a
# band index given the (descending) thresholds - an independent check of the
# decode, so the decode is verified against separate math, not against itself.
expected_range() {
    local band="$1" n="${#TH[@]}"
    if [ "$band" -le 0 ]; then
        echo ">= ${TH[0]} dBm"
    elif [ "$band" -ge "$n" ]; then
        echo "< ${TH[$((n - 1))]} dBm"
    else
        echo "${TH[$band]} to ${TH[$((band - 1))]} dBm"
    fi
}

# --- the single real station device (waits for iwd to enumerate it) ---------
STA=$(resolve_sta) \
    || fail "iwd never presented a wireless device (rfkill soft-block? wlan0 taken by NetworkManager? radio not up?)"
echo "[hw-signal] STA=$STA  SSID=$SSID  SECURITY=$SECURITY  thresholds='$THRESHOLDS'"

step "device $STA mode station"
spiderw device "$STA" mode station || fail "$STA -> station mode"

STA_PATH=$(sta_path "$STA")
[ "$STA_PATH" != "" ] || fail "could not resolve the device path for $STA"
# --- connect (the agent monitors the CONNECTED link); scan + connect + retry -
# --- via the shared helper --------------------------------------------------
NET=$(connect_sta "$STA" "$STA_PATH") \
    || fail "could not connect to $SSID (scan/connect retries exhausted)"
echo "[hw-signal] connected to $SSID"

# --- register the signal-level agent and capture the initial callback -------
# monitor-signal blocks and prints `level=N (range)` on each callback; the first
# line is iwd's register-time delivery. Capture it, then stop the monitor (its
# SIGTERM handler unregisters the agent cleanly).
step "station $STA monitor-signal $THRESHOLDS"
: >"$MON"
# shellcheck disable=SC2086  # deliberate word-split: one dBm per argument
spiderw station "$STA" monitor-signal $THRESHOLDS >"$MON" 2>&1 &
mon_pid=$!
line=""
for ((i = 0; i < REG_TIMEOUT; i++)); do
    line=$(grep -m1 -E '^level=[0-9]+ \(.*\)$' "$MON" 2>/dev/null) \
        && [ "$line" != "" ] && break
    sleep 1
done
kill "$mon_pid" 2>/dev/null
wait "$mon_pid" 2>/dev/null

echo "[hw-signal] initial callback: '${line:-<none>}'"
[ "$line" != "" ] \
    || fail "agent registered but iwd delivered no initial callback within ${REG_TIMEOUT}s"

# --- check the decode -------------------------------------------------------
# band index in range, and the range string matches the independent re-derivation
# (robust to whatever band iwd reports: 0 with no CQM list, or the real band).
band="${line#level=}"; band="${band%% *}"
range="${line#*(}"; range="${range%)}"
n="${#TH[@]}"
{ [ "$band" -ge 0 ] && [ "$band" -le "$n" ]; } \
    || fail "band index $band out of range 0..$n"
want_range="$(expected_range "$band")"
[ "$range" = "$want_range" ] \
    || fail "band $band decoded to '$range', expected '$want_range'"
echo "[hw-signal] band $band decoded correctly as '$range'"

# --- clean up (leave wlan0 disconnected) ------------------------------------
spiderw station "$STA" disconnect >/dev/null 2>&1 || true

echo
echo "[hw-signal] PASS (SignalLevelAgent registered, delivered, and decoded on real hardware)"
