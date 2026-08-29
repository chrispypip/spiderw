#!/usr/bin/env bash
# Prove spiderw drives iwd AP MODE on real hardware - the write paths every other
# hardware tier skips. All the station tiers are station-only (a single-radio DUT
# can't host an AP AND a station), so spiderw's AccessPoint path gets ZERO
# real-hardware coverage otherwise. Here the DUT itself becomes the AP: spiderw
# sets AP mode, starts an AP, iwd's driver brings it up, and we assert the AP is
# Started and broadcasting the SSID, then stop it.
#
# WHY IT MATTERS. examples/README documents this path as fullmac-fragile: the
# inline `access-point start` (Start) often fails on brcmfmac with a generic
# "failed starting", while `start-profile` (StartProfile, from a stored .ap file)
# works. Started=true is not iwd's fantasy - on fullmac an inline Start ERRORS and
# Started stays false - so asserting it genuinely proves the driver started the
# AP. This tier exercises the reliable StartProfile path (the assertion) and also
# PROBES the inline Start (informational - a failure there is the known fullmac
# caveat, not a tier failure).
#
# Uses the DUT's own radio, so no external AP is needed. AP quality varies by
# driver (brcmfmac = ok basic AP via a profile; iwlwifi = a poor AP), so if even
# StartProfile fails, that is likely a driver limitation - reported as such.
#
# Env: SSID / PASSPHRASE (the AP the DUT HOSTS); SETTLE_TRIES (Started-wait).
set -uo pipefail

# shellcheck source=tests/hardware/tiers/common.sh
. "$(dirname "${BASH_SOURCE[0]}")/common.sh"

SSID="${SSID:-spiderw-ap-hw}"
PASSPHRASE="${PASSPHRASE:-spiderw-ap-secret}"
SETTLE_TRIES="${SETTLE_TRIES:-10}"
# iwd stores AP profiles as <name>.ap here (localstatedir=/var; see the
# Dockerfile). The tier runs as root alongside iwd in the container, so it writes
# the profile itself.
AP_DIR="${AP_DIR:-/var/lib/iwd/ap}"
PROFILE="$AP_DIR/$SSID.ap"

dump_state() {
    [ -f /tmp/iwd.log ] || return 0
    echo "--- iwd log (ap/start/profile lines) ---"
    grep -iE 'ap_|access.?point|profile|start|security|mlme|nl80211' /tmp/iwd.log \
        | tail -n 30 | sed 's/^/  /'
}
fail() { echo "[hw-ap] FAIL: $*" >&2; dump_state >&2; exit 1; }
step() { echo; echo "== $* =="; }

# ap_started DEV - true iff iwd reports the AP Started (polls up to SETTLE_TRIES).
ap_started() {
    local dev="$1" i status
    for ((i = 0; i < SETTLE_TRIES; i++)); do
        status=$(spiderw access-point "$dev" status 2>/dev/null) || true
        printf '%s' "$status" | grep -qiE 'Started:[[:space:]]*true' && return 0
        sleep 1
    done
    return 1
}
ap_stopped() {
    local dev="$1" i
    for ((i = 0; i < SETTLE_TRIES; i++)); do
        spiderw access-point "$dev" status 2>/dev/null \
            | grep -qiE 'Started:[[:space:]]*false' && return 0
        sleep 1
    done
    return 1
}

# --- resolve the DUT device (waits for iwd to enumerate it) -----------------
DEV=$(resolve_sta) \
    || fail "iwd never presented a wireless device (rfkill? wlan0 taken by NetworkManager? radio not up?)"
echo "[hw-ap] DEV=$DEV  SSID=$SSID (the DUT will HOST this AP)"

step "device $DEV mode ap"
spiderw device "$DEV" mode ap || fail "$DEV -> ap mode (does the radio support AP?)"

# --- probe the inline Start (informational; fullmac-fragile) -----------------
step "PROBE: access-point $DEV start $SSID (inline; may fail on fullmac)"
if spiderw access-point "$DEV" start "$SSID" "$PASSPHRASE" 2>&1 | sed 's/^/  /' \
    && ap_started "$DEV"; then
    echo "[hw-ap] inline Start WORKED on this driver"
    spiderw access-point "$DEV" stop >/dev/null 2>&1 || true
    ap_stopped "$DEV" || true
else
    echo "[hw-ap] inline Start did not bring the AP up (expected on fullmac" \
         "brcmfmac; StartProfile below is the real assertion)"
    spiderw access-point "$DEV" stop >/dev/null 2>&1 || true
fi

# --- write a stored profile and start from it (the assertion) ----------------
# 0700 dir / 0600 file: the profile holds a passphrase, and iwd refuses a secret
# in a group/world-readable file.
step "write stored profile $PROFILE"
mkdir -p "$AP_DIR" && chmod 700 "$AP_DIR" || fail "could not prepare $AP_DIR"
cat >"$PROFILE" <<EOF
[Security]
Passphrase=$PASSPHRASE
EOF
chmod 600 "$PROFILE" || fail "could not chmod $PROFILE"

step "access-point $DEV start-profile $SSID"
spiderw access-point "$DEV" start-profile "$SSID" \
    || fail "start-profile failed - the driver may not support AP mode (iwlwifi is a poor AP), or the profile is invalid"

ap_started "$DEV" \
    || fail "AP not Started after start-profile within ${SETTLE_TRIES}s"
status=$(spiderw access-point "$DEV" status); echo "$status" | sed 's/^/  /'
echo "$status" | grep -qE "SSID:[[:space:]]*$SSID" \
    || fail "AP is Started but SSID is not '$SSID'"
echo "[hw-ap] AP is up from the profile, broadcasting $SSID"

# --- stop and assert it went down -------------------------------------------
step "access-point $DEV stop"
spiderw access-point "$DEV" stop || fail "AP stop failed"
ap_stopped "$DEV" || fail "AP still Started after stop"
echo "[hw-ap] AP stopped cleanly"

# --- clean up (leave the radio back in station mode; container is --rm) ------
spiderw device "$DEV" mode station >/dev/null 2>&1 || true
rm -f "$PROFILE" || true

echo
echo "[hw-ap] PASS (spiderw drove iwd AP mode on real hardware: start-profile up, broadcasting, stop down)"
