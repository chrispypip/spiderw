#!/usr/bin/env bash
# Exercise the WSC (WPS) enrollee interface against a REAL iwd 3.12.
#
# FEASIBILITY NOTE. spiderw's WSC support is STATION-side only (enrollee):
# PushButton / StartPin / GeneratePin / Cancel on a device's SimpleConfiguration
# interface. A COMPLETED enrollment needs an access point advertising active
# WPS/PBC as a registrar - and iwd's AP mode is not a WPS registrar (its
# AccessPoint interface has no WSC method), while this image ships no hostapd. So
# no registrar exists on hwsim and a join cannot complete here; that would need
# real WPS hardware or hostapd with wps_state configured.
#
# What this tier CAN drive, and asserts, is the enrollee interface short of a
# join:
#   1. GeneratePin returns a valid 8-digit WPS PIN (correct check digit) - a pure
#      iwd call, no AP required, and the strongest assertion here.
#   2. PushButton INITIATES an enrollment on the real interface, and Cancel aborts
#      it on iwd's side (the background call returns once iwd drops the session).
#   3. Cancel with nothing in progress is rejected by iwd (matchable no-op error).
#
# Asserts and exits non-zero on the first failure. Needs one station radio (uses
# the first device); tolerates more.
set -uo pipefail

SETTLE_TRIES="${SETTLE_TRIES:-10}"

dump_iwd_log() {
    [ -f /tmp/iwd.log ] || return 0
    echo "--- iwd log (wsc/wps lines) ---"
    grep -iE 'wsc|wps|simpleconfig|push.?button|pin|enroll' /tmp/iwd.log \
        | tail -n 30 | sed 's/^/  /'
}
fail() { echo "[wsc] FAIL: $*" >&2; dump_iwd_log >&2; exit 1; }
step() { echo; echo "== $* =="; }

# --- one station device -----------------------------------------------------
mapfile -t DEVICES < <(spiderw device list | cut -f1 | grep -v '^/' | awk 'NF')
[ "${#DEVICES[@]}" -ge 1 ] || fail "need >=1 named device, saw: ${DEVICES[*]:-none}"
STA="${DEVICES[0]}"
echo "[wsc] STA=$STA"
spiderw device "$STA" mode station || fail "$STA -> station mode"

# The SimpleConfiguration interface is exported once the device is in station
# mode; give it a moment to appear before the first WSC call.
for _ in $(seq 1 "$SETTLE_TRIES"); do
    spiderw station "$STA" status >/dev/null 2>&1 && break
    sleep 1
done

# --- 1. GeneratePin -> a valid 8-digit WPS PIN ------------------------------
# `wsc pin` with no argument prints iwd's generated PIN, then BLOCKS on StartPin
# (scanning for a PIN-mode AP that will never appear). Run it in the background,
# capture the printed PIN, validate it, then cancel to unblock and reap it.
step "wsc pin (GeneratePin) - capture and validate the PIN"
PIN_LOG=/tmp/wsc-pin.log
: >"$PIN_LOG"
spiderw station "$STA" wsc pin >"$PIN_LOG" 2>&1 &
pin_pid=$!
pin=""
for _ in $(seq 1 "$SETTLE_TRIES"); do
    pin=$(grep -oE 'WSC PIN [0-9]+' "$PIN_LOG" | awk '{print $3}' | head -n1)
    [ -n "$pin" ] && break
    kill -0 "$pin_pid" 2>/dev/null || break   # it exited before printing a PIN
    sleep 1
done
spiderw station "$STA" wsc cancel >/dev/null 2>&1 || true   # unblock StartPin
kill "$pin_pid" 2>/dev/null; wait "$pin_pid" 2>/dev/null

echo "[wsc] pin output: $(sed 's/^/  /' "$PIN_LOG")"
[ -n "$pin" ] || fail "wsc pin never printed a generated PIN"
case "$pin" in
    [0-9][0-9][0-9][0-9][0-9][0-9][0-9][0-9]) : ;;
    *) fail "generated PIN '$pin' is not exactly 8 digits" ;;
esac
# WPS check digit: with weights 3,1,3,1,3,1,3,1 across the 8 digits, the weighted
# sum must be a multiple of 10. This is what makes it a real WPS PIN, not just 8
# random digits - exactly the kind of thing a mock could get wrong.
valid=$(awk -v p="$pin" 'BEGIN {
    accum = 0
    for (i = 1; i <= 8; i++) { w = (i % 2 == 1) ? 3 : 1; accum += w * substr(p, i, 1) }
    print (accum % 10 == 0) ? 1 : 0
}')
[ "$valid" = 1 ] || fail "generated PIN '$pin' fails the WPS checksum (bad check digit)"
echo "[wsc] GeneratePin -> $pin (valid 8-digit WPS PIN)"

# --- 2. PushButton initiates, Cancel aborts it on iwd's side ----------------
step "wsc push-button (initiate) then cancel"
PB_LOG=/tmp/wsc-pb.log
: >"$PB_LOG"
spiderw station "$STA" wsc push-button >"$PB_LOG" 2>&1 &
pb_pid=$!
started=false
for _ in $(seq 1 "$SETTLE_TRIES"); do
    grep -q 'press the WPS button' "$PB_LOG" && { started=true; break; }
    kill -0 "$pb_pid" 2>/dev/null || break
    sleep 1
done
[ "$started" = true ] || fail "push-button never printed its initiation line: $(cat "$PB_LOG")"
echo "[wsc] PushButton initiated"
sleep 2   # let PushButton register the PBC session with iwd before cancelling

out=$(spiderw station "$STA" wsc cancel 2>&1) || fail "wsc cancel failed: $out"
echo "$out" | grep -qi 'canceled' || fail "cancel did not report success: $out"

# Killing the CLI would not stop iwd's session, so proof the cancel WORKED is the
# background PushButton returning (iwd dropped the session). If it is still
# running, the cancel did not take effect on iwd's side.
ended=false
for _ in $(seq 1 "$SETTLE_TRIES"); do
    kill -0 "$pb_pid" 2>/dev/null || { ended=true; break; }
    sleep 1
done
kill "$pb_pid" 2>/dev/null; wait "$pb_pid" 2>/dev/null
[ "$ended" = true ] || fail "PushButton still running after cancel (iwd session not aborted)"
echo "[wsc] Cancel aborted the in-progress PushButton on iwd's side"

# --- 3. Cancel with nothing in progress is rejected -------------------------
step "wsc cancel with no operation in progress (must be rejected)"
if out=$(spiderw station "$STA" wsc cancel 2>&1); then
    fail "cancel succeeded with nothing in progress (expected iwd to reject it): $out"
fi
echo "[wsc] no-op cancel rejected as expected: $out"

echo
echo "[wsc] PASS (enrollee interface exercised: GeneratePin validated,"
echo "           PushButton initiate+cancel, no-op cancel rejected;"
echo "           a completed join needs a WPS registrar, absent on hwsim)"
