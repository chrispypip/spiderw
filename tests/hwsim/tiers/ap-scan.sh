#!/usr/bin/env bash
# Scan from the AP side against a REAL iwd 3.12.
#
# The station scan paths are covered elsewhere (connect, ordered-networks); this
# drives the DISTINCT AccessPoint-side scan: AccessPoint.Scan plus
# GetOrderedNetworks, which iwd exports on a device in AP mode so a running AP can
# survey nearby networks. iwd only allows the scan on a STARTED AP - an unstarted
# AP-mode device returns "Operation not available" - so both APs are started; the
# scanner then surveys and the tier asserts the target AP's SSID comes back
# through `access-point <ap> networks`, resolved to its name (not a raw path) with
# a signal and a security type.
#
# Asserts and exits non-zero on the first failure. Two radios (two APs).
set -uo pipefail

TARGET_SSID="${TARGET_SSID:-spiderw-apscan-target}"
SCANNER_SSID="${SCANNER_SSID:-spiderw-apscan-scanner}"
PASSPHRASE="${PASSPHRASE:-spiderw-secret}"
SCAN_TRIES="${SCAN_TRIES:-15}"

dump_state() {
    echo "--- ${SCANNER:-?} networks ---"
    [ -n "${SCANNER:-}" ] && spiderw access-point "$SCANNER" networks 2>&1 | sed 's/^/  /' || true
    [ -f /tmp/iwd.log ] && { echo "--- iwd log (scan lines) ---"; grep -iE 'scan|ap_' /tmp/iwd.log | tail -n 20 | sed 's/^/  /'; }
}
fail() { echo "[ap-scan] FAIL: $*" >&2; dump_state >&2; exit 1; }
step() { echo; echo "== $* =="; }

# --- two devices: one scanning AP, one target AP ----------------------------
mapfile -t DEVICES < <(spiderw device list | cut -f1 | grep -v '^/' | awk 'NF')
[ "${#DEVICES[@]}" -ge 2 ] || fail "need >=2 named devices, saw: ${DEVICES[*]:-none}"
SCANNER="${DEVICES[0]}"; TARGET="${DEVICES[1]}"
echo "[ap-scan] SCANNER=$SCANNER  TARGET=$TARGET (SSID $TARGET_SSID)"

# --- bring up the target AP (the network to be found) -----------------------
step "device $TARGET mode ap; access-point start $TARGET_SSID"
spiderw device "$TARGET" mode ap || fail "$TARGET -> ap mode"
spiderw access-point "$TARGET" start "$TARGET_SSID" "$PASSPHRASE" || fail "target AP start"

# --- start the scanner AP too (iwd only scans from a STARTED AP) -------------
step "device $SCANNER mode ap; access-point start $SCANNER_SSID"
spiderw device "$SCANNER" mode ap || fail "$SCANNER -> ap mode"
spiderw access-point "$SCANNER" start "$SCANNER_SSID" "$PASSPHRASE" \
    || fail "scanner AP start"

# --- scan from the AP side until the target network shows up -----------------
# access-point <ap> networks lists "Name<TAB>signal dBm<TAB>Type" per network,
# from AccessPoint.GetOrderedNetworks.
target_line() {
    spiderw access-point "$SCANNER" networks 2>/dev/null \
        | awk -F'\t' -v s="$TARGET_SSID" '$1==s {print; exit}'
}
line=""
for try in $(seq 1 "$SCAN_TRIES"); do
    step "access-point $SCANNER scan (try $try/$SCAN_TRIES)"
    spiderw access-point "$SCANNER" scan || fail "AccessPoint scan failed"
    line=$(target_line); [ -n "$line" ] && break
    sleep 1
done

echo
echo "== access-point $SCANNER networks =="
spiderw access-point "$SCANNER" networks | sed 's/^/  /'

# --- assertions -------------------------------------------------------------
[ -n "$line" ] || fail "$TARGET_SSID never appeared in the AP-side scan after $SCAN_TRIES tries"
# Field 1 matched the SSID exactly above, so it resolved to a name, not a path.
sig=$(printf '%s' "$line" | awk -F'\t' '{print $2}')
type=$(printf '%s' "$line" | awk -F'\t' '{print $3}')
printf '%s' "$sig" | grep -qiE 'dBm' || fail "AP-side entry has no dBm signal: '$line'"
[ -n "$type" ] || fail "AP-side entry has no security type: '$line'"
echo "[ap-scan] AP-side scan found $TARGET_SSID  signal=$sig  type=$type"

spiderw access-point "$TARGET" stop >/dev/null 2>&1 || true

echo
echo "[ap-scan] PASS (AccessPoint.Scan + GetOrderedNetworks found and resolved a neighbor AP)"
