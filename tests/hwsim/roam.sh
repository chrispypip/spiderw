#!/usr/bin/env bash
# Force a ROAM and prove spiderw observes it, against a REAL iwd 3.12.
#
# Roaming is the behaviour this project could never test: it needs the station to
# move from one AP to a better one WITHOUT disconnecting, and with a single real
# radio there is no second AP and no way to make the first one fade. hwsim gives
# both - two AP radios on one SSID, plus per-link RSSI control (net.connman.hwsim)
# to make the connected AP fade so iwd roams to the other.
#
# The assertion is the roam SIGNATURE, seen through `spiderw station monitor
# access-point`: the associated BSS changes to the other AP with NO
# `access-point=none` line between. A reconnect would show that `none`
# (a disassociation); a true roam does not. That distinction is the whole test.
#
# Asserts and exits non-zero on the first failure. Needs three radios (RADIOS=3)
# and the hwsim medium controller (HWSIM_MEDIUM=1); run.sh sets both for roam.sh.
set -uo pipefail

SSID="${SSID:-spiderw-roam}"
PASSPHRASE="${PASSPHRASE:-spiderw-secret}"
# Centi-dBm the connected AP is faded to (-80.00 dBm). Below iwd's default roam
# threshold (~-70), but not so low the link drops - a drop would disconnect and
# reconnect, not roam. The other AP keeps the medium default (-30 dBm).
WEAK_CDBM="${WEAK_CDBM:--8000}"
ROAM_TIMEOUT="${ROAM_TIMEOUT:-40}"
SCAN_TRIES="${SCAN_TRIES:-10}"

dump_iwd_log() {
    [ -f /tmp/iwd.log ] || return 0
    echo "--- iwd log (roam/rssi/cqm/scan lines) ---"
    grep -iE 'roam|rssi|cqm|signal|threshold|scan|candidate|neighbor|transition' \
        /tmp/iwd.log | tail -n 40 | sed 's/^/  /'
    echo "--- iwd log (tail) ---"
    tail -n 20 /tmp/iwd.log | sed 's/^/  /'
}
fail() { echo "[roam] FAIL: $*" >&2; dump_iwd_log >&2; exit 1; }
step() { echo; echo "== $* =="; }

command -v hwsim >/dev/null || fail "hwsim tool not in image (build with --enable-hwsim)"
dbus-send --system --print-reply --dest=org.freedesktop.DBus /org/freedesktop/DBus \
    org.freedesktop.DBus.ListNames 2>/dev/null | grep -q net.connman.hwsim \
    || fail "net.connman.hwsim not on the bus (run with HWSIM_MEDIUM=1)"

# --- three devices: two APs on one SSID, one station ------------------------
mapfile -t DEVICES < <(spiderw device list | cut -f1 | grep -v '^/' | awk 'NF')
[ "${#DEVICES[@]}" -ge 3 ] || fail "need >=3 devices (RADIOS=3), saw: ${DEVICES[*]:-none}"
AP1="${DEVICES[0]}"; AP2="${DEVICES[1]}"; STA="${DEVICES[2]}"

start_ap() {
    spiderw device "$1" mode ap  || fail "$1 -> ap mode"
    spiderw access-point "$1" start "$SSID" "$PASSPHRASE" || fail "$1 AP start"
}
step "start two APs on SSID $SSID"
start_ap "$AP1"
start_ap "$AP2"
# The AP's BSSID is its device (radio) address - what the station associates to.
BSS1=$(spiderw device "$AP1" address) || fail "read $AP1 address"
BSS2=$(spiderw device "$AP2" address) || fail "read $AP2 address"
echo "[roam] AP1=$AP1 ($BSS1)  AP2=$AP2 ($BSS2)  STA=$STA"

# --- connect the station ----------------------------------------------------
spiderw device "$STA" mode station || fail "$STA -> station mode"
found=0
for try in $(seq 1 "$SCAN_TRIES"); do
    step "station $STA scan (try $try/$SCAN_TRIES)"
    spiderw station "$STA" scan || true
    if spiderw network list | cut -f1 | grep -qxF "$SSID"; then found=1; break; fi
    sleep 1
done
[ "$found" -eq 1 ] || fail "station never saw $SSID after $SCAN_TRIES scans"
spiderw network "$SSID" connect --passphrase="$PASSPHRASE" || fail "connect failed"

cur_bss() { spiderw station "$STA" status | awk '/ConnectedAccessPoint:/{print $2}'; }
cur=""
for _ in $(seq 1 5); do            # the association can lag the connect call by a beat
    cur=$(cur_bss)
    [ -n "$cur" ] && break
    sleep 1
done
[ -n "$cur" ] || fail "no ConnectedAccessPoint after connect"
case "$cur" in
    "$BSS1") other="$BSS2" ;;
    "$BSS2") other="$BSS1" ;;
    *) fail "connected BSS $cur is neither AP ($BSS1 / $BSS2)" ;;
esac
echo "[roam] connected to $cur; driving a roam to $other"

# Confirm iwd knows BOTH BSSes before we fade one, so a roam has a candidate the
# moment the RSSI drops. A best-effort nudge-scan may collide with iwd's own
# auto-scan ("operation already in progress"), which is harmless - silence it.
spiderw station "$STA" scan >/dev/null 2>&1 || true
echo "[roam] BSSes iwd knows for $SSID:"
spiderw network "$SSID" bsses | sed 's/^/  /' || true

# --- watch the association through spiderw, across the roam ------------------
MON=/tmp/roam-monitor.log
spiderw station "$STA" monitor access-point >"$MON" 2>&1 &
mon_pid=$!
sleep 1   # let the monitor print the seed line for the current AP

# --- force the roam: fade the CURRENT AP with a hwsim SignalStrength rule ----
# AddRule returns a new /ruleN path; rules start disabled and match nothing, so
# set Source (frames FROM the current AP), SignalStrength, then Enabled=true.
step "fade $cur to ${WEAK_CDBM} cdBm via net.connman.hwsim"
rule=$(dbus-send --system --print-reply --dest=net.connman.hwsim / \
        net.connman.hwsim.RuleManager.AddRule 2>/dev/null \
        | awk '/object path/{print $NF}' | tr -d '"')
[ -n "$rule" ] || fail "AddRule returned no rule path"
# --print-reply is not optional here: without it dbus-send fires and exits before
# the reply, so a rejected Set looks like success and the fade silently no-ops.
set_rule() {   # PROP SIG VALUE
    dbus-send --system --print-reply --dest=net.connman.hwsim "$rule" \
        org.freedesktop.DBus.Properties.Set \
        string:net.connman.hwsim.Rule "string:$1" "variant:$2:$3" >/dev/null \
        || fail "set rule $1=$3 failed"
}
set_rule Source string "$cur"
set_rule SignalStrength int16 "$WEAK_CDBM"
set_rule Enabled boolean true

# Read the rule back and ASSERT the fade stuck, so a Set that quietly did nothing
# fails here (with the values) rather than after a 40s no-roam timeout.
get_rule() {   # PROP
    dbus-send --system --print-reply --dest=net.connman.hwsim "$rule" \
        org.freedesktop.DBus.Properties.Get \
        string:net.connman.hwsim.Rule "string:$1" 2>/dev/null \
        | awk 'NR>1{$1="";print}' | sed 's/^ *//'
}
src=$(get_rule Source); sig=$(get_rule SignalStrength); en=$(get_rule Enabled)
echo "[roam] rule $rule: Source=$src SignalStrength=$sig Enabled=$en"
case "$en" in *true*) ;; *) fail "rule did not enable (Enabled=$en); the fade never applied" ;; esac
case "$sig" in *"$WEAK_CDBM"*) ;; *) fail "SignalStrength did not take ($sig); the fade never applied" ;; esac

# --- wait for the roam ------------------------------------------------------
step "waiting up to ${ROAM_TIMEOUT}s for the station to roam to $other"
now="$cur"
for _ in $(seq 1 "$ROAM_TIMEOUT"); do
    now=$(cur_bss)
    [ "$now" = "$other" ] && break
    sleep 1
done

# The status poll and the monitor's subscription race on the same property change;
# give the monitor a moment to write the roam line before killing it.
sleep 1
kill "$mon_pid" 2>/dev/null
wait "$mon_pid" 2>/dev/null

echo
echo "== monitor access-point capture =="
sed 's/^/  /' "$MON"

[ "$now" = "$other" ] || fail "station did not roam to $other within ${ROAM_TIMEOUT}s (still $now)"

# Roam signature: the new AP appears in the stream, and no disassociation
# (access-point=none) happened between association and the roam.
grep -q "access-point=$other" "$MON" || fail "monitor never reported the new AP $other"
grep -q "access-point=none" "$MON" \
    && fail "saw a disassociation (access-point=none): that is a reconnect, not a roam"

echo "[roam] clean roam $cur -> $other, no disassociation"
echo
echo "[roam] PASS"
