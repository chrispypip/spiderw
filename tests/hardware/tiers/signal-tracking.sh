#!/usr/bin/env bash
# Prove spiderw's SignalLevelAgent tracks a MOVING signal LIVE - the band index
# changes as the RSSI crosses thresholds, not just the one initial callback.
# Softmac-only: this is the coverage neither hwsim nor a fullmac station can give.
#
# WHY THIS IS A SEPARATE TIER FROM signal.sh. The band index rides iwd's
# MULTI-threshold CQM monitor (NL80211_EXT_FEATURE_CQM_RSSI_LIST). hwsim
# implements only the single-threshold monitor, and fullmac brcmfmac does its CQM
# in firmware and does not deliver the list events - so on both, EVERY reading is
# the initial band and signal.sh can only assert register + initial callback +
# decode. A softmac station (mt76, iwlwifi) DOES deliver the list events, so here
# the band should actually move as the signal fades - which is what this asserts.
#
# HOW THE SIGNAL MOVES (external, not driven here). This tier only OBSERVES; it
# does not control the RF. Something must vary the RSSI while it runs - the lab
# rig is a "breathing" AP whose TX power is swept up and down by a standalone
# loop on the AP host (spiderw-test's spiderw-ap-fade.sh), so the DUT sees the
# received signal rise and fall with NO coordination or SSH. Point this tier at
# that AP and start the fade before running it.
#
# WHAT IT ASSERTS. Over a window, iwd calls the agent each time the band crosses
# a threshold; spiderw decodes each into `level=N (range)`. This asserts (a) at
# least MIN_DISTINCT_BANDS DIFFERENT band indices are seen (i.e. the band really
# moved - the whole point), and (b) every one decodes to the correct range string
# (the same independent re-derivation signal.sh uses). Exits non-zero on failure.
#
# The target AP is supplied by env (no AP is created here):
#   SSID / PASSPHRASE / SECURITY(psk|open) / SCAN_TRIES   (as in connect.sh)
#   THRESHOLDS   descending dBm band edges - place them INSIDE the AP's fade
#                range so the sweep crosses them (default "-80 -84 -88", which
#                suits a breathing AP swinging ~ -78..-90 dBm at the DUT)
#   TRACK_WINDOW seconds to watch the agent for band movement (default 90)
#   MIN_DISTINCT_BANDS  distinct band indices required to pass (default 2)
set -uo pipefail

# shellcheck source=tests/hardware/tiers/common.sh
. "$(dirname "${BASH_SOURCE[0]}")/common.sh"

SSID="${SSID:-}"
PASSPHRASE="${PASSPHRASE:-}"
SECURITY="${SECURITY:-psk}"
SCAN_TRIES="${SCAN_TRIES:-15}"
# Initial association must land in a high-power window of the breathing AP, so
# retry the connect across (at least) one fade cycle: CONNECT_TRIES x CONNECT_WAIT
# should exceed the AP's sweep period (~72s with the default fade).
CONNECT_TRIES="${CONNECT_TRIES:-12}"
CONNECT_WAIT="${CONNECT_WAIT:-6}"
THRESHOLDS="${THRESHOLDS:--80 -84 -88}"
TRACK_WINDOW="${TRACK_WINDOW:-90}"
MIN_DISTINCT_BANDS="${MIN_DISTINCT_BANDS:-2}"

MON=/tmp/signal-tracking-monitor.log
read -r -a TH <<<"$THRESHOLDS"     # thresholds as an array for the range check

dump_state() {
    echo "--- monitor-signal capture ---"
    [ -f "$MON" ] && sed 's/^/  /' "$MON" || echo "  (no monitor output)"
    [ -f /tmp/iwd.log ] || return 0
    echo "--- iwd log (signal-agent lines) ---"
    grep -iE 'signal_agent|rssi_report_level|SignalLevelAgent' /tmp/iwd.log \
        | tail -n 20 | sed 's/^/  /'
}
fail() { echo "[hw-signal-track] FAIL: $*" >&2; dump_state >&2; exit 1; }
step() { echo; echo "== $* =="; }

[ "$SSID" != "" ] || fail "SSID is required (the external breathing AP)"
if [ "$SECURITY" = "psk" ] && [ "$PASSPHRASE" = "" ]; then
    fail "SECURITY=psk needs PASSPHRASE (or set SECURITY=open)"
fi

# expected_range BAND: re-derive the dBm range string spiderw should print for a
# band index given the (descending) thresholds - an independent check of the
# decode, identical to signal.sh so both tiers agree on the mapping.
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
    || fail "iwd never presented a wireless device (rfkill soft-block? wlan taken by NetworkManager? radio not up?)"
echo "[hw-signal-track] STA=$STA SSID=$SSID SECURITY=$SECURITY thresholds='$THRESHOLDS' window=${TRACK_WINDOW}s"

step "device $STA mode station"
spiderw device "$STA" mode station || fail "$STA -> station mode"

STA_PATH=$(sta_path "$STA")
[ "$STA_PATH" != "" ] || fail "could not resolve the device path for $STA"
net_path() {
    spiderw network list \
      | awk -F'\t' -v ssid="$SSID" -v pfx="$STA_PATH/" \
            '$1 == ssid && index($2, pfx) == 1 { print $2; exit }'
}

# --- connect (the agent monitors the CONNECTED link) ------------------------
NET=""
for ((i = 0; i < SCAN_TRIES; i++)); do
    step "station $STA scan (try $i/$SCAN_TRIES)"
    spiderw station "$STA" scan
    NET=$(net_path)
    [ "$NET" != "" ] && break
    sleep 2
done
[ "$NET" != "" ] || fail "station $STA never saw SSID $SSID after $SCAN_TRIES scans"

# The AP is BREATHING (its TX power sweeps up and down), so a single connect can
# land during a low-power dip where association / the 4-way handshake times out
# ("Operation failed") - even though a connection, once up, rides the fade fine.
# So retry across at least one fade cycle: some attempts land in a high-power
# window and succeed. Re-resolve NET each pass in case the scan data churned.
step "network $SSID connect (breathing AP; retry across a fade cycle)"
connected=false
for ((i = 0; i < CONNECT_TRIES; i++)); do
    NET=$(net_path)
    if [ "$NET" = "" ]; then
        spiderw station "$STA" scan >/dev/null 2>&1 || true
        sleep "$CONNECT_WAIT"; continue
    fi
    if [ "$SECURITY" = "open" ]; then
        spiderw network "$NET" connect >/dev/null 2>&1 || true
    else
        spiderw network "$NET" connect --passphrase="$PASSPHRASE" >/dev/null 2>&1 || true
    fi
    if [ "$(spiderw network "$NET" connected 2>/dev/null)" = "true" ]; then
        connected=true; break
    fi
    echo "  connect try $((i + 1))/$CONNECT_TRIES failed (AP may be in a fade dip); retrying"
    sleep "$CONNECT_WAIT"
done
[ "$connected" = true ] \
    || fail "connect to $SSID failed after $CONNECT_TRIES tries - AP breathing too low (raise FADE_LOW_MBM on the AP), out of range, or wrong passphrase?"
echo "[hw-signal-track] connected to $SSID"

# --- watch the agent for LIVE band movement --------------------------------
# monitor-signal blocks and prints `level=N (range)` on every callback: the
# initial one at register time, then one each time the band crosses a threshold
# as the breathing AP fades. The band moves at UNPREDICTABLE times (real RF -
# the fade plus ambient noise), so rather than snapshot a fixed window, poll and
# stop the moment enough DISTINCT bands have appeared; TRACK_WINDOW is only the
# upper bound. Passing early is strictly better against that timing
# non-determinism - the full window is spent only when movement is NOT coming.
step "station $STA monitor-signal $THRESHOLDS (<=${TRACK_WINDOW}s)"
: >"$MON"
# shellcheck disable=SC2086  # deliberate word-split: one dBm per argument
spiderw station "$STA" monitor-signal $THRESHOLDS >"$MON" 2>&1 &
mon_pid=$!
elapsed=0
while [ "$elapsed" -lt "$TRACK_WINDOW" ]; do
    # distinct band indices seen so far (authoritative decode check is below)
    d=$(grep -oE '^level=[0-9]+' "$MON" 2>/dev/null | sort -u | wc -l)
    [ "$d" -ge "$MIN_DISTINCT_BANDS" ] && break
    sleep 2
    elapsed=$((elapsed + 2))
done
kill "$mon_pid" 2>/dev/null
wait "$mon_pid" 2>/dev/null

# --- assert the band actually moved, and every reading decoded correctly ----
# Pull the band/range from each callback line; count distinct bands and verify
# each decodes to its expected range (band index sane, range string matches).
n="${#TH[@]}"
declare -A seen=()
lines=0
while IFS= read -r line; do
    lines=$((lines + 1))
    band="${line#level=}"; band="${band%% *}"
    range="${line#*(}";    range="${range%)}"
    { [ "$band" -ge 0 ] && [ "$band" -le "$n" ]; } \
        || fail "band index $band out of range 0..$n (line: '$line')"
    want="$(expected_range "$band")"
    [ "$range" = "$want" ] \
        || fail "band $band decoded to '$range', expected '$want'"
    seen["$band"]=$(( ${seen["$band"]:-0} + 1 ))
done < <(grep -E '^level=[0-9]+ \(.*\)$' "$MON")

distinct="${#seen[@]}"
bands_sorted=$(printf '%s\n' "${!seen[@]}" | sort -n | tr '\n' ' ')
echo "[hw-signal-track] $lines callbacks in ~${elapsed}s (max ${TRACK_WINDOW}s);" \
     "distinct bands: $distinct { $bands_sorted}"

[ "$lines" -gt 0 ] \
    || fail "agent registered but iwd delivered NO callbacks in ${TRACK_WINDOW}s"
if [ "$distinct" -lt "$MIN_DISTINCT_BANDS" ]; then
    echo "[hw-signal-track] band did not move enough:" \
         "saw $distinct distinct band(s) { $bands_sorted}, need" \
         "$MIN_DISTINCT_BANDS." >&2
    echo "  Is the AP breathing (spiderw-ap-fade.sh running)? Is this a" \
         "SOFTMAC station? fullmac/hwsim only ever report the initial band." >&2
    fail "no live band movement observed"
fi

# --- clean up (leave the station disconnected) ------------------------------
spiderw station "$STA" disconnect >/dev/null 2>&1 || true

echo
echo "[hw-signal-track] PASS (SignalLevelAgent tracked a moving signal live:" \
     "$distinct distinct bands, all decoded correctly, on softmac hardware)"
