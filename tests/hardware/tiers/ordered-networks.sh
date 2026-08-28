#!/usr/bin/env bash
# Prove spiderw reads iwd's ORDERED networks correctly on the DUT's REAL station
# radio, against a real iwd 3.12 - using the genuine ambient RF around the DUT.
#
# `station networks` calls GetOrderedNetworks: iwd returns the last scan's
# networks in iwd's RANK order, and spiderw resolves each object path to its SSID
# (via GetManagedObjects) and converts the centi-dBm signal to dBm. The hwsim
# tier fabricates a ranking with two faded APs + the medium controller; here the
# real environment already provides many APs, which is more realistic and
# exercises the resolver against a diverse reply.
#
# WHAT IS AND IS NOT ASSERTED. iwd's order is NOT by raw signal - it is a
# COMPOSITE rank (band + data-rate aware): verified on real hardware, iwd ranks a
# 5GHz AP above a STRONGER 2.4GHz one. spiderw faithfully renders iwd's order and
# does not re-sort, so re-deriving that rank here would just reimplement iwd. So
# this asserts what spiderw is actually responsible for and what holds regardless
# of which APs are in range: at least MIN_NETWORKS networks, every signal a valid
# dBm number, and SSIDs resolved to names (not raw /net/connman/iwd/... paths).
# If SSID is set it must appear, resolved. The ORDER itself (iwd's rank) is not
# asserted - that is iwd's ranking logic, not spiderw's rendering.
#
# Env:
#   MIN_NETWORKS  minimum networks that must be visible (default 2)
#   SSID          optional - if set, must appear resolved in the list
#   SCAN_TRIES    scans to wait for MIN_NETWORKS to appear (default 15)
set -uo pipefail

# shellcheck source=tests/hardware/tiers/common.sh
. "$(dirname "${BASH_SOURCE[0]}")/common.sh"

MIN_NETWORKS="${MIN_NETWORKS:-2}"
SSID="${SSID:-}"
SCAN_TRIES="${SCAN_TRIES:-15}"

fail() { echo "[hw-ordered] FAIL: $*" >&2; exit 1; }
step() { echo; echo "== $* =="; }

# --- the single real station device (waits for iwd to enumerate it) ---------
STA=$(resolve_sta) \
    || fail "iwd never presented a wireless device (rfkill soft-block? wlan0 taken by NetworkManager? radio not up?)"
echo "[hw-ordered] STA=$STA  MIN_NETWORKS=$MIN_NETWORKS  SSID='${SSID:-<any>}'"

step "device $STA mode station"
spiderw device "$STA" mode station || fail "$STA -> station mode"

# --- scan until at least MIN_NETWORKS networks are visible -------------------
count=0
mapfile -t LINES < <(true)
for ((i = 0; i < SCAN_TRIES; i++)); do
    step "station $STA scan (try $i/$SCAN_TRIES)"
    spiderw station "$STA" scan
    mapfile -t LINES < <(spiderw station "$STA" networks | awk 'NF')
    count=${#LINES[@]}
    [ "$count" -ge "$MIN_NETWORKS" ] && break
    echo "[hw-ordered] only $count network(s) visible so far"
    sleep 2
done

echo
echo "== station $STA networks =="
printf '  %s\n' "${LINES[@]}"

[ "$count" -ge "$MIN_NETWORKS" ] \
    || fail "only $count network(s) visible after $SCAN_TRIES scans (want >= $MIN_NETWORKS)"

# --- assert each entry is well-formed and SSIDs resolve ---------------------
# Each line is "SSID<TAB>signal dBm<TAB>...". Field 1 is the resolved name (a raw
# /net/connman/iwd/... path if it did NOT resolve - e.g. a hidden network); field
# 2 starts with the signal in dBm. The ORDER (iwd's composite rank) is not
# asserted - see the header.
resolved=0
target_found=0
for line in "${LINES[@]}"; do
    name=$(printf '%s' "$line" | cut -f1)
    sig=$(printf '%s' "$line" | cut -f2)
    sig=${sig%% *}                     # first token of "-49 dBm"
    case "$sig" in
        -[0-9]* | [0-9]*) : ;;
        *) fail "network '$name' has a non-numeric signal: '$(printf '%s' "$line" | cut -f2)'" ;;
    esac
    case "$name" in
        /* | "") : ;;                  # unresolved (hidden) - tolerated
        *) resolved=$((resolved + 1)); [ "$name" = "$SSID" ] && target_found=1 ;;
    esac
done

[ "$resolved" -ge 1 ] \
    || fail "no network resolved to an SSID (all raw paths - the SSID resolver failed)"
if [ "$SSID" != "" ] && [ "$target_found" != 1 ]; then
    fail "SSID '$SSID' not present (resolved) in the ordered list"
fi

echo
echo "[hw-ordered] PASS ($count networks, $resolved resolved to SSIDs, all signals" \
     "valid dBm - GetOrderedNetworks rendered + SSID-resolved + dBm-decoded on real hardware)"
