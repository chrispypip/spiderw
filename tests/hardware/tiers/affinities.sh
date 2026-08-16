#!/usr/bin/env bash
# Exercise Station.SetAffinities on the Pi's REAL brcmfmac station connected to
# an EXTERNAL AP, against a real iwd 3.12.
#
# FEASIBILITY NOTE. Affinities is an iwd [experimental] read-write property, so
# iwd hides it unless started with -E (tests/hardware/run.sh sets
# IWD_EXPERIMENTAL=1 for this tier). It only exists on a CONNECTED station - an
# affinity is a BSS the station should stay pinned to within its ESS.
#
# iwd ties the affinity to the LIFETIME OF THE D-BUS CLIENT that set it: when
# that client disconnects iwd drops it (station_affinity_disconnected_cb), so a
# dead controller cannot leave a roaming pin. The spiderw CLI is one-shot -
# `affinities set` opens a connection, writes, and EXITS - so a cross-invocation
# round-trip cannot be observed; only a long-lived client (the library) can hold
# one. What IS observable, and what this asserts, is the write lifecycle: iwd
# ACCEPTS the set (echoes the MAC) and DROPS it on client exit. Both together
# prove the write reached and was applied.
#
# ADDED HARDWARE RISK: brcmfmac is fullmac (firmware roaming); iwd may REJECT
# SetAffinities if the driver lacks the roaming control it needs. That rejection
# is a real finding (reported as the feasibility wall), not a tier bug.
#
# Env: SSID / PASSPHRASE / SECURITY(psk|open) / SCAN_TRIES / SETTLE_TRIES
set -uo pipefail

# shellcheck source=tests/hardware/tiers/common.sh
. "$(dirname "${BASH_SOURCE[0]}")/common.sh"

SSID="${SSID:-}"
PASSPHRASE="${PASSPHRASE:-}"
SECURITY="${SECURITY:-psk}"
SCAN_TRIES="${SCAN_TRIES:-15}"
SETTLE_TRIES="${SETTLE_TRIES:-10}"

dump_iwd_log() {
    [ -f /tmp/iwd.log ] || return 0
    echo "--- iwd log (affinity/experimental lines) ---"
    grep -iE 'affinit|experimental|not support|unknown' /tmp/iwd.log \
        | tail -n 30 | sed 's/^/  /'
}
fail() { echo "[hw-affinities] FAIL: $*" >&2; dump_iwd_log >&2; exit 1; }
step() { echo; echo "== $* =="; }

[ "$SSID" != "" ] || fail "SSID is required (the external AP to connect to)"
if [ "$SECURITY" = "psk" ] && [ "$PASSPHRASE" = "" ]; then
    fail "SECURITY=psk needs PASSPHRASE (or set SECURITY=open)"
fi

# --- the single real station device (waits for iwd to enumerate it) ---------
STA=$(resolve_sta) \
    || fail "iwd never presented a wireless device (rfkill? wlan0 taken by NetworkManager? radio not up?)"
echo "[hw-affinities] STA=$STA  SSID=$SSID  SECURITY=$SECURITY"

step "device $STA mode station"
spiderw device "$STA" mode station || fail "$STA -> station mode"

STA_PATH=$(sta_path "$STA")
[ "$STA_PATH" != "" ] || fail "could not resolve the device path for $STA"
net_path() {
    spiderw network list \
      | awk -F'\t' -v ssid="$SSID" -v pfx="$STA_PATH/" \
            '$1 == ssid && index($2, pfx) == 1 { print $2; exit }'
}

# --- connect (affinities need a connected station) --------------------------
NET=""
for ((i = 0; i < SCAN_TRIES; i++)); do
    step "station $STA scan (try $i/$SCAN_TRIES)"
    spiderw station "$STA" scan
    NET=$(net_path)
    [ "$NET" != "" ] && break
    sleep 2
done
[ "$NET" != "" ] || fail "station $STA never saw SSID $SSID after $SCAN_TRIES scans"

step "network $SSID connect"
if [ "$SECURITY" = "open" ]; then
    spiderw network "$NET" connect || fail "connect (open) failed"
else
    spiderw network "$NET" connect --passphrase="$PASSPHRASE" || fail "connect failed"
fi
[ "$(spiderw network "$NET" connected)" = "true" ] || fail "not connected after connect"
echo "[hw-affinities] connected to $SSID"

# --- the connected BSS is the affinity target -------------------------------
AP_MAC=$(spiderw station "$STA" status \
    | sed -n 's/^ConnectedAccessPoint:[[:space:]]*//p' | head -n1)
case "$AP_MAC" in
    "" | "-") fail "no ConnectedAccessPoint MAC in status (cannot pick an affinity target)" ;;
    *:*:*:*:*:*) : ;;
    *) fail "ConnectedAccessPoint is not a MAC: '$AP_MAC'" ;;
esac
echo "[hw-affinities] affinity target (connected BSS): $AP_MAC"

# --- assertion 1: iwd ACCEPTS the write -------------------------------------
# A non-zero exit here is the real feasibility wall (property missing without
# -E, or fullmac brcmfmac refusing the pin). Success means iwd took the write.
step "station $STA affinities set $AP_MAC"
if ! out=$(spiderw station "$STA" affinities set "$AP_MAC" 2>&1); then
    echo "$out"
    case "$out" in
    *NotSupported* | *not*support* | *xperimental* | *Failed* | *failed*)
        fail "iwd rejected SetAffinities ($out): the property is unavailable (no -E?) or fullmac brcmfmac will not pin - a real feasibility wall on this driver, not verifiable here" ;;
    esac
    fail "affinities set failed: $out"
fi
echo "$out"
grep -Fiq "$AP_MAC" <<<"$out" \
    || fail "set did not echo the affinity $AP_MAC (got: $out)"
echo "[hw-affinities] iwd accepted SetAffinities for $AP_MAC"

# --- assertion 2: iwd DROPS it when the setting CLI exits -------------------
# The set above ran in its own one-shot process, now gone; iwd clears the
# affinity on that disconnect, so a read from THIS fresh connection shows none.
has_affinity() { spiderw station "$STA" affinities 2>/dev/null | grep -Fiq "$1"; }
dropped=false
for ((i = 0; i < SETTLE_TRIES; i++)); do
    has_affinity "$AP_MAC" || { dropped=true; break; }
    sleep 1
done
[ "$dropped" = true ] \
    || fail "affinity $AP_MAC still set after the setting client exited (expected iwd to drop it)"
echo "[hw-affinities] affinity dropped after the setting client exited (client-scoped, as iwd intends)"
grep -q 'client that set affinity has disconnected' /tmp/iwd.log 2>/dev/null \
    && echo "[hw-affinities] confirmed in iwd log: station_affinity_disconnected_cb fired"

spiderw station "$STA" disconnect >/dev/null 2>&1 || true

echo
echo "[hw-affinities] PASS (iwd accepted SetAffinities and dropped it on client exit;"
echo "               a persistent affinity is only holdable by a long-lived client)"
