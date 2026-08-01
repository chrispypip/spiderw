#!/usr/bin/env bash
# Start an access point from a STORED profile against a REAL iwd 3.12.
#
# Every other AP in the suite uses `access-point start <ssid> <psk>` - an inline,
# ad-hoc AP. This drives the distinct StartProfile path: iwd reads a stored .ap
# profile file (which can configure security beyond the inline PSK form) and
# brings the AP up from it. iwd's AP dir is /var/lib/iwd/ap (localstatedir is
# /var; see the Dockerfile), and this tier runs as root alongside iwd in the
# container, so it writes the profile itself.
#
# The tier writes a minimal PSK profile, starts the AP from it, asserts the AP is
# up broadcasting the profile's SSID, and proves the profile actually works by
# connecting a station with the profile's passphrase. Asserts and exits non-zero
# on the first failure. Two radios (one AP, one station).
set -uo pipefail

SSID="${SSID:-spiderw-profile}"
PASSPHRASE="${PASSPHRASE:-spiderw-secret}"
SCAN_TRIES="${SCAN_TRIES:-10}"
# iwd stores AP profiles as <name>.ap here; the profile name is the SSID.
AP_DIR="${AP_DIR:-/var/lib/iwd/ap}"
PROFILE="$AP_DIR/$SSID.ap"

dump_iwd_log() {
    [ -f /tmp/iwd.log ] || return 0
    echo "--- iwd log (ap/profile lines) ---"
    grep -iE 'ap_|access.?point|profile|start|security' /tmp/iwd.log \
        | tail -n 30 | sed 's/^/  /'
}
fail() { echo "[start-profile] FAIL: $*" >&2; dump_iwd_log >&2; exit 1; }
step() { echo; echo "== $* =="; }

# --- devices: one AP, one station -------------------------------------------
mapfile -t DEVICES < <(spiderw device list | cut -f1 | grep -v '^/' | awk 'NF')
[ "${#DEVICES[@]}" -ge 2 ] || fail "need >=2 named devices, saw: ${DEVICES[*]:-none}"
AP="${DEVICES[0]}"; STA="${DEVICES[1]}"
echo "[start-profile] AP=$AP  STA=$STA  SSID=$SSID"

# --- write the stored AP profile --------------------------------------------
# 0700 dir / 0600 file: the profile holds a passphrase, and iwd refuses secrets
# in a group/world-readable file.
step "write stored profile $PROFILE"
mkdir -p "$AP_DIR" && chmod 700 "$AP_DIR" || fail "could not prepare $AP_DIR"
cat > "$PROFILE" <<EOF
[Security]
Passphrase=$PASSPHRASE
EOF
chmod 600 "$PROFILE" || fail "could not chmod $PROFILE"
echo "[start-profile] wrote:"; sed 's/^/  /' "$PROFILE"

# --- start the AP from the profile ------------------------------------------
step "device $AP mode ap; access-point start-profile $SSID"
spiderw device "$AP" mode ap || fail "$AP -> ap mode"
spiderw access-point "$AP" start-profile "$SSID" \
    || fail "start-profile failed (profile not found at $PROFILE, or invalid?)"

# --- assert the AP is up broadcasting the profile's SSID --------------------
status=$(spiderw access-point "$AP" status) || fail "access-point status failed"
echo "$status" | sed 's/^/  /'
echo "$status" | grep -qiE "Started:[[:space:]]*true" \
    || fail "AP not Started after start-profile"
echo "$status" | grep -qE "SSID:[[:space:]]*$SSID" \
    || fail "AP SSID is not '$SSID' after start-profile"
echo "[start-profile] AP is up from the profile, broadcasting $SSID"

# --- prove the profile works: a station connects with its passphrase --------
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
[ -n "$NET" ] || fail "station never saw the profile AP's SSID $SSID after $SCAN_TRIES scans"

step "network $SSID connect (with the profile's passphrase)"
spiderw network "$NET" connect --passphrase="$PASSPHRASE" \
    || fail "connect to the profile AP failed"
[ "$(spiderw network "$NET" connected)" = "true" ] \
    || fail "not connected to the profile AP after connect"
echo "[start-profile] station connected to the profile AP; the profile's security works"

# --- clean up (container is --rm, but leave iwd tidy) -----------------------
spiderw station "$STA" disconnect >/dev/null 2>&1 || true
spiderw access-point "$AP" stop >/dev/null 2>&1 || true
rm -f "$PROFILE" || true

echo
echo "[start-profile] PASS (AP started from a stored profile; a station connected with it)"
