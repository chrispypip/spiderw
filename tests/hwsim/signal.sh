#!/usr/bin/env bash
# Prove spiderw's SignalLevelAgent registers and delivers against a REAL iwd 3.12.
#
# `station monitor-signal <dBm>...` registers a net.connman.iwd.SignalLevelAgent
# with RSSI thresholds; iwd calls it back with a band index (0 = strongest) as
# the connected signal crosses a threshold, and spiderw decodes that into a
# `level=N (range)` line. Nothing else in the suite exercises that whole
# interface against a real daemon (the mock covers only the callback plumbing).
#
# WHAT HWSIM CAN AND CANNOT SHOW. The band index tracks the RSSI only through
# iwd's MULTI-threshold CQM monitor (NL80211_EXT_FEATURE_CQM_RSSI_LIST). mac80211
# (which hwsim is) implements only the SINGLE-threshold CQM RSSI monitor - the
# one iwd's roam low-signal trigger uses, which is why the roam tier CAN drive a
# live signal change but this one cannot. iwd initializes its band index to 0 and
# advances it only on those list events, so on hwsim EVERY reading is band 0 no
# matter the real RSSI (verified: at a genuine -45 dBm iwd still reports band 0).
# Live band tracking is therefore only testable on real hardware with a driver
# that supports the RSSI list; here we prove the rest of the path.
#
# So the assertions are: the agent registers, iwd delivers the initial callback,
# and spiderw decodes it into a valid band index with the range string its own
# band math should produce. Asserts and exits non-zero on the first failure.
# Needs two radios (one AP, one station); no medium controller required.
set -uo pipefail

SSID="${SSID:-spiderw-signal}"
PASSPHRASE="${PASSPHRASE:-spiderw-secret}"
SCAN_TRIES="${SCAN_TRIES:-10}"
# The thresholds define four bands; the tier asserts the initial band decodes to
# the correct range string for whatever band iwd reports (band 0 on hwsim).
THRESHOLDS="${THRESHOLDS:--40 -50 -60}"
REG_TIMEOUT="${REG_TIMEOUT:-10}"   # seconds to wait for the initial band line

MON=/tmp/signal-monitor.log
read -r -a TH <<<"$THRESHOLDS"     # thresholds as an array for the range check

dump_iwd_log() {
    echo "--- monitor-signal capture ---"
    [ -f "$MON" ] && sed 's/^/  /' "$MON" || echo "  (no monitor output)"
    [ -f /tmp/iwd.log ] || return 0
    echo "--- iwd log (signal-agent lines) ---"
    grep -iE 'signal_agent|rssi_report_level|SignalLevelAgent' /tmp/iwd.log \
        | tail -n 20 | sed 's/^/  /'
}
fail() { echo "[signal] FAIL: $*" >&2; dump_iwd_log >&2; exit 1; }
step() { echo; echo "== $* =="; }

# expected_range BAND: the dBm range string spiderw should print for a band index
# given the (descending) thresholds - a re-derivation of the CLI's signalBandRange
# so the decode is checked against an independent computation, not itself.
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

# --- two devices: one AP, one station ---------------------------------------
mapfile -t DEVICES < <(spiderw device list | cut -f1 | grep -v '^/' | awk 'NF')
[ "${#DEVICES[@]}" -ge 2 ] || fail "need >=2 named devices, saw: ${DEVICES[*]:-none}"
AP="${DEVICES[0]}"
STA="${DEVICES[1]}"
echo "[signal] AP=$AP  STA=$STA  SSID=$SSID  thresholds='$THRESHOLDS'"

# --- bring up the access point ----------------------------------------------
step "device $AP mode ap"
spiderw device "$AP" mode ap || fail "could not set $AP to ap mode"
step "access-point $AP start $SSID"
spiderw access-point "$AP" start "$SSID" "$PASSPHRASE" || fail "AP start failed"

# --- connect the station ----------------------------------------------------
step "device $STA mode station"
spiderw device "$STA" mode station || fail "could not set $STA to station mode"

STA_PATH=$(spiderw device list | awk -F'\t' -v d="$STA" '$1 == d {print $2}')
[ -n "$STA_PATH" ] || fail "could not resolve the device path for $STA"
net_path() {
    spiderw network list \
      | awk -F'\t' -v ssid="$SSID" -v pfx="$STA_PATH/" \
            '$1 == ssid && index($2, pfx) == 1 { print $2; exit }'
}

NET=""
for try in $(seq 1 "$SCAN_TRIES"); do
    step "station $STA scan (try $try/$SCAN_TRIES)"
    spiderw station "$STA" scan || true
    NET=$(net_path)
    [ -n "$NET" ] && break
    sleep 1
done
[ -n "$NET" ] || fail "station $STA never saw SSID $SSID after $SCAN_TRIES scans"

step "network $SSID connect"
spiderw network "$NET" connect --passphrase="$PASSPHRASE" || fail "connect failed"
connected=$(spiderw network "$NET" connected)
[ "$connected" = "true" ] || fail "network $SSID connected=$connected (want true)"
echo "[signal] connected to $SSID"

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
for _ in $(seq 1 "$REG_TIMEOUT"); do
    line=$(grep -m1 -E '^level=[0-9]+ \(.*\)$' "$MON" 2>/dev/null) && [ -n "$line" ] && break
    sleep 1
done
kill "$mon_pid" 2>/dev/null
wait "$mon_pid" 2>/dev/null

echo "[signal] initial callback: '${line:-<none>}'"
[ -n "$line" ] \
    || fail "agent registered but iwd delivered no initial callback within ${REG_TIMEOUT}s"

# --- check the decode -------------------------------------------------------
# band index in range, and the range string matches an independent re-derivation.
band="${line#level=}"; band="${band%% *}"
range="${line#*(}"; range="${range%)}"
n="${#TH[@]}"
{ [ "$band" -ge 0 ] && [ "$band" -le "$n" ]; } \
    || fail "band index $band out of range 0..$n"
want_range="$(expected_range "$band")"
[ "$range" = "$want_range" ] \
    || fail "band $band decoded to '$range', expected '$want_range'"

echo "[signal] band $band decoded correctly as '$range'"
echo
echo "[signal] PASS (SignalLevelAgent registered, delivered, and decoded against real iwd)"
