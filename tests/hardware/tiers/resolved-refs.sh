#!/usr/bin/env bash
# Prove spiderw resolves iwd object PATHS to friendly identifiers on the DUT's
# REAL station radio against an EXTERNAL AP, against a real iwd 3.12.
#
# spiderw's status bundles turn iwd's cross-object paths into the names they
# stand for - a connected network shows its SSID, an access point its MAC, a
# device its adapter name - by reading Name/Address out of one GetManagedObjects
# tree. That resolver is unit-tested only against the mock, whose object tree
# merely IMITATES iwd's layout; if real iwd's tree differs, the resolver silently
# falls back to the raw /net/connman/iwd/... path and no other tier notices.
#
# This connects a station, then asserts every cross-object reference in the
# status bundles is RESOLVED (not a raw path) and mutually consistent with the
# ground truth we DO have: the SSID we connected to, the station device name, a
# real adapter name, and that the connected BSSID is a MAC that appears in the
# network's advertised BSSes. (Unlike the hwsim tier we don't own the AP, so the
# BSSID ground truth is this internal consistency rather than a known AP address.)
#
# Env: SSID / PASSPHRASE / SECURITY(psk|open) / SCAN_TRIES   (as in connect.sh)
set -uo pipefail

# shellcheck source=tests/hardware/tiers/common.sh
. "$(dirname "${BASH_SOURCE[0]}")/common.sh"

SSID="${SSID:-}"
PASSPHRASE="${PASSPHRASE:-}"
SECURITY="${SECURITY:-psk}"
SCAN_TRIES="${SCAN_TRIES:-15}"

dump_state() {
    local sta="${STA:-}" net="${NET:-}"
    [ "$sta" != "" ] || return 0
    echo "--- station status ---"; spiderw station "$sta" status 2>&1 | sed 's/^/  /' || true
    [ "$net" != "" ] && { echo "--- network status ---"; spiderw network "$net" status 2>&1 | sed 's/^/  /' || true; }
    echo "--- device status ---"; spiderw device "$sta" status 2>&1 | sed 's/^/  /' || true
}
fail() { echo "[hw-refs] FAIL: $*" >&2; dump_state >&2; exit 1; }
step() { echo; echo "== $* =="; }
lc()   { printf '%s' "$1" | tr '[:upper:]' '[:lower:]'; }

# field_of LABEL  (status text on stdin) -> the value after "LABEL:"
field_of() { sed -n "s/^$1:[[:space:]]*//p" | head -n1; }

# not_a_path LABEL VALUE: fail if VALUE is empty or a raw iwd object path (which
# is what an UNRESOLVED ref renders as).
not_a_path() {
    case "$2" in
    "") fail "$1 is empty" ;;
    /*) fail "$1 is an unresolved raw object path: $2" ;;
    esac
}
is_mac() { case "$1" in [0-9a-fA-F]*:*:*:*:*:[0-9a-fA-F]*) return 0 ;; *) return 1 ;; esac; }

[ "$SSID" != "" ] || fail "SSID is required (the external AP to connect to)"
if [ "$SECURITY" = "psk" ] && [ "$PASSPHRASE" = "" ]; then
    fail "SECURITY=psk needs PASSPHRASE (or set SECURITY=open)"
fi

# --- the single real station device (waits for iwd to enumerate it) ---------
STA=$(resolve_sta) \
    || fail "iwd never presented a wireless device (rfkill soft-block? wlan0 taken by NetworkManager? radio not up?)"
echo "[hw-refs] STA=$STA  SSID=$SSID  SECURITY=$SECURITY"

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

step "network $SSID connect"
if [ "$SECURITY" = "open" ]; then
    spiderw network "$NET" connect || fail "connect (open) failed"
else
    spiderw network "$NET" connect --passphrase="$PASSPHRASE" || fail "connect failed"
fi
[ "$(spiderw network "$NET" connected)" = "true" ] || fail "not connected after connect"
echo "[hw-refs] connected to $SSID"

# --- station status: ConnectedNetwork -> SSID, ConnectedAccessPoint -> MAC ---
step "station $STA status: resolved ConnectedNetwork / ConnectedAccessPoint"
sta_status=$(spiderw station "$STA" status) || fail "station status failed"
echo "$sta_status" | sed 's/^/  /'
cn=$(printf '%s\n' "$sta_status" | field_of ConnectedNetwork)
cap=$(printf '%s\n' "$sta_status" | field_of ConnectedAccessPoint)
not_a_path ConnectedNetwork "$cn"
not_a_path ConnectedAccessPoint "$cap"
[ "$cn" = "$SSID" ] || fail "ConnectedNetwork resolved to '$cn', want the SSID '$SSID'"
is_mac "$cap" || fail "ConnectedAccessPoint '$cap' is not a MAC"
echo "[hw-refs] station status resolved: network=$cn  ap=$cap"

# --- network status: Device -> station name, BasicServiceSets -> MAC(s) -----
# Ground-truth cross-check: the connected BSSID (cap) must be one of the MACs the
# network advertises, so the resolver's AP identity is internally consistent.
step "network $SSID status: resolved Device / BasicServiceSets"
net_status=$(spiderw network "$NET" status) || fail "network status failed"
echo "$net_status" | sed 's/^/  /'
dev=$(printf '%s\n' "$net_status" | field_of Device)
bss=$(printf '%s\n' "$net_status" | field_of BasicServiceSets)
not_a_path Device "$dev"
not_a_path BasicServiceSets "$bss"
[ "$dev" = "$STA" ] || fail "network Device resolved to '$dev', want the station '$STA'"
printf '%s' "$bss" | grep -qiE '([0-9a-f]{2}:){5}[0-9a-f]{2}' \
    || fail "BasicServiceSets '$bss' has no MAC"
printf '%s' "$(lc "$bss")" | grep -qF "$(lc "$cap")" \
    || fail "BasicServiceSets '$bss' does not include the connected BSSID '$cap'"
echo "[hw-refs] network status resolved: device=$dev  bsses include $cap"

# --- device status: Adapter -> adapter name ---------------------------------
step "device $STA status: resolved Adapter"
dev_status=$(spiderw device "$STA" status) || fail "device status failed"
echo "$dev_status" | sed 's/^/  /'
adapter=$(printf '%s\n' "$dev_status" | field_of Adapter)
not_a_path Adapter "$adapter"
spiderw adapter list | cut -f1 | grep -Fxq "$adapter" \
    || fail "device Adapter '$adapter' is not in 'adapter list' (not a real adapter name)"
echo "[hw-refs] device status resolved: adapter=$adapter"

spiderw station "$STA" disconnect >/dev/null 2>&1 || true

echo
echo "[hw-refs] PASS (every cross-object ref resolved to its friendly identifier:"
echo "          SSID, connected BSSID/MAC, station name, adapter name - none raw paths)"
