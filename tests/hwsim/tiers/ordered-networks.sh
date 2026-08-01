#!/usr/bin/env bash
# Prove spiderw reads iwd's ORDERED networks correctly, against a REAL iwd 3.12.
#
# `station networks` calls GetOrderedNetworks: iwd returns the last scan's
# networks ranked best-signal-first, and spiderw resolves each object path to its
# SSID (via GetManagedObjects) and converts the centi-dBm signal to dBm. Nothing
# else in the suite exercises that method or that ranking against a real daemon -
# the connect tier reads `network list` (unordered, daemon-wide), not this.
#
# The ranking is the point, so the test needs two networks at DIFFERENT signal
# levels. That means the hwsim medium controller (net.connman.hwsim), same as the
# roam tier: two APs on DISTINCT SSIDs, one station, and a rule that fades the
# second AP so it is clearly weaker. Then `station networks` must list the strong
# SSID first, the weak one second, both resolved (not raw paths), with the strong
# signal numerically greater.
#
# Asserts and exits non-zero on the first failure. Needs three radios (RADIOS=3)
# and the hwsim medium controller (HWSIM_MEDIUM=1); run.sh sets both.
set -uo pipefail

SSID_STRONG="${SSID_STRONG:-spiderw-strong}"
SSID_WEAK="${SSID_WEAK:-spiderw-weak}"
PASSPHRASE="${PASSPHRASE:-spiderw-secret}"
# Centi-dBm the weak AP is faded to (-75.00 dBm). The strong AP keeps the medium
# default (-30 dBm), so the two are ~45 dB apart - an unambiguous ranking.
WEAK_CDBM="${WEAK_CDBM:--7500}"
# Minimum dBm gap required between the two, so a fade that only half-applied
# (leaving them near-equal) fails loudly rather than ordering by chance.
MIN_GAP_DBM="${MIN_GAP_DBM:-15}"
SCAN_TRIES="${SCAN_TRIES:-15}"

dump_iwd_log() {
    [ -f /tmp/iwd.log ] || return 0
    echo "--- iwd log (scan/rank/signal lines) ---"
    grep -iE 'scan|rank|signal|bss|network' /tmp/iwd.log | tail -n 30 \
        | sed 's/^/  /'
}
fail() { echo "[ordered] FAIL: $*" >&2; dump_iwd_log >&2; exit 1; }
step() { echo; echo "== $* =="; }

command -v hwsim >/dev/null || fail "hwsim tool not in image (build with --enable-hwsim)"
dbus-send --system --print-reply --dest=org.freedesktop.DBus /org/freedesktop/DBus \
    org.freedesktop.DBus.ListNames 2>/dev/null | grep -q net.connman.hwsim \
    || fail "net.connman.hwsim not on the bus (run with HWSIM_MEDIUM=1)"

# --- three devices: two APs (distinct SSIDs), one station -------------------
mapfile -t DEVICES < <(spiderw device list | cut -f1 | grep -v '^/' | awk 'NF')
[ "${#DEVICES[@]}" -ge 3 ] || fail "need >=3 devices (RADIOS=3), saw: ${DEVICES[*]:-none}"
AP_STRONG="${DEVICES[0]}"; AP_WEAK="${DEVICES[1]}"; STA="${DEVICES[2]}"

step "start two APs: $SSID_STRONG on $AP_STRONG, $SSID_WEAK on $AP_WEAK"
spiderw device "$AP_STRONG" mode ap || fail "$AP_STRONG -> ap mode"
spiderw access-point "$AP_STRONG" start "$SSID_STRONG" "$PASSPHRASE" \
    || fail "$AP_STRONG AP start"
spiderw device "$AP_WEAK" mode ap || fail "$AP_WEAK -> ap mode"
spiderw access-point "$AP_WEAK" start "$SSID_WEAK" "$PASSPHRASE" \
    || fail "$AP_WEAK AP start"
# The AP's BSSID is its radio address; the fade rule matches frames from it.
BSS_WEAK=$(spiderw device "$AP_WEAK" address) || fail "read $AP_WEAK address"
echo "[ordered] STRONG=$SSID_STRONG  WEAK=$SSID_WEAK ($BSS_WEAK)  STA=$STA"

spiderw device "$STA" mode station || fail "$STA -> station mode"

# --- fade the weak AP via a hwsim SignalStrength rule -----------------------
# AddRule returns /ruleN; rules start disabled and match nothing, so set Source
# (frames FROM the weak AP), SignalStrength, then Enabled=true. --print-reply is
# not optional: without it dbus-send exits before the reply and a rejected Set
# looks like success.
step "fade $SSID_WEAK to ${WEAK_CDBM} cdBm via net.connman.hwsim"
rule=$(dbus-send --system --print-reply --dest=net.connman.hwsim / \
        net.connman.hwsim.RuleManager.AddRule 2>/dev/null \
        | awk '/object path/{print $NF}' | tr -d '"')
[ -n "$rule" ] || fail "AddRule returned no rule path"
set_rule() {   # PROP SIG VALUE
    dbus-send --system --print-reply --dest=net.connman.hwsim "$rule" \
        org.freedesktop.DBus.Properties.Set \
        string:net.connman.hwsim.Rule "string:$1" "variant:$2:$3" >/dev/null \
        || fail "set rule $1=$3 failed"
}
get_rule() {   # PROP
    dbus-send --system --print-reply --dest=net.connman.hwsim "$rule" \
        org.freedesktop.DBus.Properties.Get \
        string:net.connman.hwsim.Rule "string:$1" 2>/dev/null \
        | awk 'NR>1{$1="";print}' | sed 's/^ *//'
}
set_rule Source string "$BSS_WEAK"
set_rule SignalStrength int16 "$WEAK_CDBM"
set_rule Enabled boolean true
sig=$(get_rule SignalStrength); en=$(get_rule Enabled)
echo "[ordered] rule $rule: Source=$BSS_WEAK SignalStrength=$sig Enabled=$en"
case "$en" in *true*) ;; *) fail "rule did not enable (Enabled=$en); fade never applied" ;; esac
case "$sig" in *"$WEAK_CDBM"*) ;; *) fail "SignalStrength did not take ($sig); fade never applied" ;; esac

# --- scan (post-fade) until both SSIDs show, with the expected gap ----------
# GetOrderedNetworks returns the most recent scan, so scan AFTER the fade and
# loop until the reported signals reflect it: both present and >= MIN_GAP_DBM
# apart. An early auto-scan could still hold the weak AP at -30, so do not read
# just once.
signal_of() {   # SSID -> dBm number, empty if absent
    spiderw station "$STA" networks 2>/dev/null \
        | awk -F'\t' -v s="$1" '$1==s {split($2,a," "); print a[1]; exit}'
}
strong_dbm=""; weak_dbm=""
for try in $(seq 1 "$SCAN_TRIES"); do
    step "station $STA scan (try $try/$SCAN_TRIES)"
    spiderw station "$STA" scan || true
    strong_dbm=$(signal_of "$SSID_STRONG")
    weak_dbm=$(signal_of "$SSID_WEAK")
    if [ -n "$strong_dbm" ] && [ -n "$weak_dbm" ] \
       && awk -v a="$strong_dbm" -v b="$weak_dbm" -v g="$MIN_GAP_DBM" \
              'BEGIN{exit !(a - b >= g)}'; then
        break
    fi
    sleep 1
done

echo
echo "== station $STA networks =="
spiderw station "$STA" networks | sed 's/^/  /'

# --- assertions -------------------------------------------------------------
[ -n "$strong_dbm" ] || fail "$SSID_STRONG never appeared in ordered networks"
[ -n "$weak_dbm" ]   || fail "$SSID_WEAK never appeared in ordered networks"

# Both SSIDs resolved (matched by exact name above, so a raw path would not have
# matched). Now the ranking: the strong SSID must be listed before the weak one.
names=$(spiderw station "$STA" networks | cut -f1)
i_strong=$(grep -nxF "$SSID_STRONG" <<<"$names" | head -n1 | cut -d: -f1)
i_weak=$(grep -nxF "$SSID_WEAK" <<<"$names" | head -n1 | cut -d: -f1)
[ -n "$i_strong" ] && [ -n "$i_weak" ] || fail "could not locate both SSIDs in the list"
[ "$i_strong" -lt "$i_weak" ] \
    || fail "ranking wrong: $SSID_STRONG (line $i_strong) not before $SSID_WEAK (line $i_weak)"

# And the signal spiderw decoded must agree with the ranking: strong > weak.
awk -v a="$strong_dbm" -v b="$weak_dbm" -v g="$MIN_GAP_DBM" 'BEGIN{exit !(a - b >= g)}' \
    || fail "signal gap too small: $SSID_STRONG=$strong_dbm dBm vs $SSID_WEAK=$weak_dbm dBm (< $g)"

echo
echo "[ordered] PASS: $SSID_STRONG ($strong_dbm dBm) ranked above" \
     "$SSID_WEAK ($weak_dbm dBm); both SSIDs resolved and signal-ordered"
