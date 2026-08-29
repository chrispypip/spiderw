#!/usr/bin/env bash
# Prove spiderw observes a clean ROAM on real hardware: the station moves from one
# AP to another of the SAME ESS WITHOUT disassociating. Roaming is behaviour a
# single real radio could never test - it needs two APs and a way to move the
# station between them.
#
# HOW THE ROAM IS DRIVEN (externally, not here). Two APs on the AP host share the
# SSID on different channels; an orchestrator there (spiderw-test's roam rig)
# SEEDS the station onto AP1 (AP2 starts down), then, when it sees the station
# associate, brings AP2 up and triggers a roam to it - EITHER an 802.11v BSS
# Transition Management steer (works on any DUT) OR by fading AP1's TX power so a
# SOFTMAC station self-roams on low signal. This tier is agnostic to which: it is
# purely OBSERVATIONAL - it connects and watches, and does NOT drive the APs (no
# SSH / coordination from the DUT). (Signal-triggered self-roam is softmac-only:
# run it on the iwlwifi DUT, not brcmfmac, which does not self-roam.)
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

# --- connect (only AP1 is up during seeding, so we land on it) ---------------
NET=$(connect_sta "$STA" "$STA_PATH") \
    || fail "could not connect to $SSID (is AP1 up on the AP host?)"
echo "[hw-roam] connected to $SSID"

# brcmfmac resets power-save to ON after each (re)connect, which can delay BTM
# action-frame delivery; best-effort disable so the steer is heard promptly.
# (No-op where iw is absent or the driver ignores it.)
iw dev "$STA" set power_save off 2>/dev/null || true

cur_bss() {
    spiderw station "$STA" status 2>/dev/null \
        | awk '/ConnectedAccessPoint:/ {print $2}'
}
cur=""
for _ in $(seq 1 5); do   # association can lag the connect call by a beat
    cur=$(cur_bss)
    [ "$cur" != "" ] && break
    sleep 1
done
[ "$cur" != "" ] || fail "no ConnectedAccessPoint after connect"
echo "[hw-roam] seeded on BSS $cur; waiting up to ${ROAM_TIMEOUT}s for the AP" \
     "host to steer a roam"

# --- watch the association across the roam -----------------------------------
: >"$MON"
spiderw station "$STA" monitor access-point >"$MON" 2>&1 &
mon_pid=$!
sleep 1   # let the monitor print the seed line for the current AP

# The orchestrator fires the BTM steer when it sees us associate; wait for the
# associated BSS to change to a DIFFERENT one of the ESS.
now="$cur"
for _ in $(seq 1 "$ROAM_TIMEOUT"); do
    now=$(cur_bss)
    [ "$now" != "" ] && [ "$now" != "$cur" ] && break
    sleep 1
done

# The status poll and the monitor's subscription race on the same property
# change; give the monitor a moment to write the roam line before killing it.
sleep 1
kill "$mon_pid" 2>/dev/null
wait "$mon_pid" 2>/dev/null

echo
echo "== monitor access-point capture =="
sed 's/^/  /' "$MON"

if ! { [ "$now" != "" ] && [ "$now" != "$cur" ]; }; then
    echo "[hw-roam] no roam within ${ROAM_TIMEOUT}s (still on $cur)." >&2
    echo "  Is the AP host's roam orchestrator running (AP2 up + a BTM steer)," \
         "and is the steer reaching this station?" >&2
    fail "station did not roam"
fi

# --- the roam signature -----------------------------------------------------
# The new AP appears in the stream, and no disassociation (access-point=none)
# happened between the association and the roam.
grep -q "access-point=$now" "$MON" \
    || fail "monitor never reported the new AP $now"
grep -q "access-point=none" "$MON" \
    && fail "saw a disassociation (access-point=none): that is a reconnect, not a roam"

echo "[hw-roam] clean roam $cur -> $now, no disassociation"
spiderw station "$STA" disconnect >/dev/null 2>&1 || true

echo
echo "[hw-roam] PASS (clean roam observed on real hardware)"
