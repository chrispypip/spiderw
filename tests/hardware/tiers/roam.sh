#!/usr/bin/env bash
# Prove spiderw observes a clean ROAM on real hardware: the station moves from one
# AP to another of the SAME ESS WITHOUT disassociating. Roaming is behaviour a
# single real radio could never test - it needs two APs and a way to move the
# station between them.
#
# HOW THE ROAM IS DRIVEN (externally, not here). Two APs on the AP host share the
# SSID on different channels; an orchestrator there (spiderw-test's roam rig)
# keeps BOTH APs up, sees which one the station associates to, and triggers a
# roam to the OTHER - EITHER an 802.11v BSS Transition Management steer (works on
# any DUT) OR by fading the current AP's TX power so a SOFTMAC station self-roams.
# This tier is agnostic to which: it is purely OBSERVATIONAL - it connects and
# watches, and does NOT drive the APs (no SSH / coordination from the DUT).
# (Signal-triggered self-roam is softmac-only: run it on the iwlwifi DUT, not
# brcmfmac, which does not self-roam.)
#
# THE ASSERTION - the roam SIGNATURE, via `spiderw station monitor access-point`:
# the associated BSS changes to a DIFFERENT BSS of the ESS with NO
# `access-point=none` between. A reconnect shows that `none` (a disassociation);
# a true roam does not. That distinction is the whole test.
#
# Requires the roam AP rig up on the AP host (two APs on one SSID + the BTM
# orchestrator). Env:
#   SSID / PASSPHRASE / SECURITY(psk|open) / SCAN_TRIES / CONNECT_TRIES
#   ROAM_TIMEOUT  seconds to wait for the steered roam (default 60)
set -uo pipefail

# shellcheck source=tests/hardware/tiers/common.sh
. "$(dirname "${BASH_SOURCE[0]}")/common.sh"

SSID="${SSID:-}"
PASSPHRASE="${PASSPHRASE:-}"
SECURITY="${SECURITY:-psk}"
ROAM_TIMEOUT="${ROAM_TIMEOUT:-60}"

MON=/tmp/roam-monitor.log

dump_state() {
    echo "--- monitor access-point capture ---"
    [ -f "$MON" ] && sed 's/^/  /' "$MON" || echo "  (no monitor output)"
    [ -f /tmp/iwd.log ] || return 0
    echo "--- iwd log (roam/transition/candidate lines) ---"
    grep -iE 'roam|transition|candidate|neighbor|bss_tm|rssi|scan' /tmp/iwd.log \
        | tail -n 30 | sed 's/^/  /'
}
fail() { echo "[hw-roam] FAIL: $*" >&2; dump_state >&2; exit 1; }
step() { echo; echo "== $* =="; }

[ "$SSID" != "" ] || fail "SSID is required (the roam ESS - both APs share it)"
if [ "$SECURITY" = "psk" ] && [ "$PASSPHRASE" = "" ]; then
    fail "SECURITY=psk needs PASSPHRASE (or set SECURITY=open)"
fi

# --- the single real station device (waits for iwd to enumerate it) ---------
STA=$(resolve_sta) \
    || fail "iwd never presented a wireless device (rfkill? wlan0 taken by NetworkManager? radio not up?)"
echo "[hw-roam] STA=$STA  SSID=$SSID  SECURITY=$SECURITY"

step "device $STA mode station"
spiderw device "$STA" mode station || fail "$STA -> station mode"

STA_PATH=$(sta_path "$STA")
[ "$STA_PATH" != "" ] || fail "could not resolve the device path for $STA"

# --- connect (the orchestrator steers us once it sees us associate) ----------
# NET follows the common.sh connect idiom but is not read back in this tier
# (the connection is asserted by watching the monitor below).
# shellcheck disable=SC2034
NET=$(connect_sta "$STA" "$STA_PATH") \
    || fail "could not connect to $SSID (is the roam rig up on the AP host?)"
echo "[hw-roam] connected to $SSID"

# brcmfmac resets power-save to ON after each (re)connect, which can delay BTM
# action-frame delivery; best-effort disable so the steer is heard promptly.
# (No-op where iw is absent or the driver ignores it.)
iw dev "$STA" set power_save off 2>/dev/null || true

# --- watch the association across the roam -----------------------------------
# Start the monitor RIGHT AFTER connecting (not after a status-poll delay) and
# decide pass/fail from THIS stream - the authoritative source. The old code
# detected the roam by polling `station status` once a second, which races the
# steer: a fast or bouncing transition slips between samples, and a steer that
# lands before the first poll leaves the baseline reading the post-roam AP. The
# monitor captures every change in order, so neither race can hide the roam.
: >"$MON"
spiderw station "$STA" monitor access-point >"$MON" 2>&1 &
mon_pid=$!

# Parse the capture (lines in order): the SEED is the first associated BSS; a
# clean ROAM is a later, DIFFERENT associated BSS with NO `access-point=none` (a
# disassociation) between them - a `none` there is a reconnect, not a roam.
# Prints one of: "ROAM seed new" | "RECONNECT seed new" | "NOROAM seed" | "NONE".
roam_verdict() {
    awk '
        /access-point=/ {
            ap = $0; sub(/.*access-point=/, "", ap); sub(/[ \t].*/, "", ap)
            if (ap == "none" || ap == "") { if (seed != "") saw_none = 1; next }
            if (seed == "") { seed = ap; next }
            if (ap != seed) { new = ap; exit }
        }
        END {
            if (new != "")       print (saw_none ? "RECONNECT " : "ROAM ") seed " " new
            else if (seed != "") print "NOROAM " seed
            else                 print "NONE"
        }' "$1"
}

echo "[hw-roam] watching up to ${ROAM_TIMEOUT}s for the AP host to steer a roam"
verdict="NONE"
for _ in $(seq 1 "$ROAM_TIMEOUT"); do
    verdict=$(roam_verdict "$MON")
    case "$verdict" in ROAM*|RECONNECT*) break ;; esac
    sleep 1
done

sleep 1   # let the monitor flush its final line
kill "$mon_pid" 2>/dev/null
wait "$mon_pid" 2>/dev/null

echo
echo "== monitor access-point capture =="
sed 's/^/  /' "$MON"

# shellcheck disable=SC2086
set -- $verdict
case "$1" in
    ROAM)
        echo "[hw-roam] clean roam $2 -> $3, no disassociation" ;;
    RECONNECT)
        fail "saw access-point=none between $2 and $3: a reconnect, not a roam" ;;
    NOROAM)
        echo "[hw-roam] no roam within ${ROAM_TIMEOUT}s (still on $2)." >&2
        echo "  Is the AP host's roam orchestrator running and steering this" \
             "station?" >&2
        fail "station did not roam" ;;
    *)
        fail "never associated to $SSID (no access-point in the monitor stream)" ;;
esac

spiderw station "$STA" disconnect >/dev/null 2>&1 || true

echo
echo "[hw-roam] PASS (clean roam observed on real hardware)"
