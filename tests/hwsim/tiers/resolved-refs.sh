#!/usr/bin/env bash
# Prove spiderw resolves iwd object PATHS to friendly identifiers, against a REAL
# iwd 3.12.
#
# spiderw's status bundles turn iwd's cross-object paths into the human names they
# stand for - a connected network shows its SSID, an access point its MAC, a
# device its adapter name - by reading the Name/Address properties out of one
# GetManagedObjects tree. That resolver is unit-tested only against the mock,
# whose object tree merely IMITATES iwd's device/network/bss-mac layout. If real
# iwd's tree or property placement differs from those assumptions, the resolver
# silently falls back to the raw /net/connman/iwd/... path and no other tier
# notices (they read these fields with `|| true`, or check only one of them).
#
# This tier connects a station, then asserts every cross-object reference in the
# status bundles is RESOLVED - and, where there is ground truth (the SSID, the
# AP's address, the station's name), that it resolved to the RIGHT value, not just
# to something that is not a path. Asserts and exits non-zero on the first
# failure. Two radios (one AP, one station).
set -uo pipefail

SSID="${SSID:-spiderw-refs}"
PASSPHRASE="${PASSPHRASE:-spiderw-secret}"
SCAN_TRIES="${SCAN_TRIES:-10}"

dump_state() {
    local sta="${STA:-}" net="${NET:-}"
    [ -n "$sta" ] || return 0   # nothing to dump before devices are picked
    echo "--- station status ---"; spiderw station "$sta" status 2>&1 | sed 's/^/  /' || true
    [ -n "$net" ] && { echo "--- network status ---"; spiderw network "$net" status 2>&1 | sed 's/^/  /' || true; }
    echo "--- device status ---";  spiderw device "$sta" status 2>&1 | sed 's/^/  /' || true
}
fail() { echo "[refs] FAIL: $*" >&2; dump_state >&2; exit 1; }
step() { echo; echo "== $* =="; }
lc()   { printf '%s' "$1" | tr '[:upper:]' '[:lower:]'; }

# field_of LABEL  (status text on stdin) -> the value after "LABEL:"
field_of() { sed -n "s/^$1:[[:space:]]*//p" | head -n1; }

# not_a_path LABEL VALUE: fail if VALUE is empty or a raw iwd object path (which is
# what an UNRESOLVED ref renders as).
not_a_path() {
    case "$2" in
    "")  fail "$1 is empty" ;;
    /*)  fail "$1 is an unresolved raw object path: $2" ;;
    esac
}
is_mac() { case "$1" in [0-9a-fA-F]*:*:*:*:*:[0-9a-fA-F]*) return 0 ;; *) return 1 ;; esac; }

# --- devices: one AP, one station -------------------------------------------
mapfile -t DEVICES < <(spiderw device list | cut -f1 | grep -v '^/' | awk 'NF')
[ "${#DEVICES[@]}" -ge 2 ] || fail "need >=2 named devices, saw: ${DEVICES[*]:-none}"
AP="${DEVICES[0]}"; STA="${DEVICES[1]}"
echo "[refs] AP=$AP  STA=$STA  SSID=$SSID"

# --- bring up the AP and connect --------------------------------------------
step "device $AP mode ap; access-point start $SSID"
spiderw device "$AP" mode ap || fail "$AP -> ap mode"
spiderw access-point "$AP" start "$SSID" "$PASSPHRASE" || fail "AP start"
# The AP's radio address is the BSSID the station associates to - ground truth
# for the resolved ConnectedAccessPoint / BasicServiceSets MAC.
AP_ADDR=$(spiderw device "$AP" address) || fail "read $AP address"

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
step "network $SSID connect"
spiderw network "$NET" connect --passphrase="$PASSPHRASE" || fail "connect failed"
[ "$(spiderw network "$NET" connected)" = "true" ] || fail "not connected after connect"
echo "[refs] connected; AP BSSID (ground truth) = $AP_ADDR"

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
[ "$(lc "$cap")" = "$(lc "$AP_ADDR")" ] \
    || fail "ConnectedAccessPoint '$cap' != the AP BSSID '$AP_ADDR'"
echo "[refs] station status resolved: network=$cn  ap=$cap"

# --- network status: Device -> station name, BasicServiceSets -> MAC(s) -----
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
printf '%s' "$(lc "$bss")" | grep -qF "$(lc "$AP_ADDR")" \
    || fail "BasicServiceSets '$bss' does not include the AP BSSID '$AP_ADDR'"
echo "[refs] network status resolved: device=$dev  bsses=$bss"

# --- device status: Adapter -> adapter name ---------------------------------
step "device $STA status: resolved Adapter"
dev_status=$(spiderw device "$STA" status) || fail "device status failed"
echo "$dev_status" | sed 's/^/  /'
adapter=$(printf '%s\n' "$dev_status" | field_of Adapter)
not_a_path Adapter "$adapter"
# Cross-check the resolved name is a REAL adapter, not just any non-path string.
spiderw adapter list | cut -f1 | grep -Fxq "$adapter" \
    || fail "device Adapter '$adapter' is not in 'adapter list' (not a real adapter name)"
echo "[refs] device status resolved: adapter=$adapter"

spiderw station "$STA" disconnect >/dev/null 2>&1 || true

echo
echo "[refs] PASS (every cross-object ref resolved to its friendly identifier:"
echo "       SSID, BSSID/MAC, station name, adapter name - none left as raw paths)"
