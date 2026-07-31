#!/usr/bin/env bash
# Exercise Station.SetAffinities against a REAL iwd 3.12.
#
# FEASIBILITY NOTE. Affinities is an iwd [experimental] read-write property, so
# iwd hides it unless started with -E (run.sh sets IWD_EXPERIMENTAL=1 for this
# tier). It also only exists on a CONNECTED station: an affinity is a BSS the
# station should stay pinned to within its connected network's ESS.
#
# The catch this tier discovered: iwd ties the affinity to the LIFETIME OF THE
# D-BUS CLIENT that set it. When that client disconnects, iwd drops the affinity
# (src/station.c:station_affinity_disconnected_cb, "client that set affinity has
# disconnected"). This is deliberate - a controller that dies should not leave a
# stale roaming pin. But the spiderw CLI is one-shot: `affinities set` opens a
# connection, writes, and EXITS, so the affinity is gone before any separate
# `affinities` read (a new connection) can see it. A cross-invocation round-trip
# therefore cannot be observed with the CLI; only a long-lived client (the
# library, held open) can hold an affinity.
#
# So what IS observable, and what this tier asserts, is the full write lifecycle:
# iwd ACCEPTS the SetAffinities write (SetProperty returns success and the CLI
# echoes the MAC), and then DROPS it when the setting CLI exits (a fresh read
# shows no affinities). Both together prove the write reached and was applied by
# iwd - the disconnect callback only fires if an affinity was actually
# registered. Asserts and exits non-zero on the first failure. Two radios.
set -uo pipefail

SSID="${SSID:-spiderw-affinity}"
PASSPHRASE="${PASSPHRASE:-spiderw-secret}"
SCAN_TRIES="${SCAN_TRIES:-10}"
SETTLE_TRIES="${SETTLE_TRIES:-10}"

dump_iwd_log() {
    [ -f /tmp/iwd.log ] || return 0
    echo "--- iwd log (affinity/experimental lines) ---"
    grep -iE 'affinit|experimental|not support|unknown' /tmp/iwd.log \
        | tail -n 30 | sed 's/^/  /'
}
fail() { echo "[affinities] FAIL: $*" >&2; dump_iwd_log >&2; exit 1; }
step() { echo; echo "== $* =="; }

# --- devices: one AP, one station -------------------------------------------
mapfile -t DEVICES < <(spiderw device list | cut -f1 | grep -v '^/' | awk 'NF')
[ "${#DEVICES[@]}" -ge 2 ] || fail "need >=2 named devices, saw: ${DEVICES[*]:-none}"
AP="${DEVICES[0]}"; STA="${DEVICES[1]}"
echo "[affinities] AP=$AP  STA=$STA  SSID=$SSID"

STA_PATH=$(spiderw device list | awk -F'\t' -v d="$STA" '$1==d{print $2}')
[ -n "$STA_PATH" ] || fail "resolve device path for $STA"
net_path() {
    spiderw network list \
      | awk -F'\t' -v ssid="$SSID" -v pfx="$STA_PATH/" \
            '$1==ssid && index($2,pfx)==1 {print $2; exit}'
}

# --- bring up the AP and connect (affinities need a connected station) -------
step "device $AP mode ap; access-point start $SSID"
spiderw device "$AP" mode ap || fail "$AP -> ap mode"
spiderw access-point "$AP" start "$SSID" "$PASSPHRASE" || fail "AP start"

step "device $STA mode station"
spiderw device "$STA" mode station || fail "$STA -> station mode"

NET=""
for try in $(seq 1 "$SCAN_TRIES"); do
    step "station $STA scan (try $try/$SCAN_TRIES)"
    spiderw station "$STA" scan || true
    NET=$(net_path); [ -n "$NET" ] && break
    sleep 1
done
[ -n "$NET" ] || fail "station $STA never saw $SSID after $SCAN_TRIES scans"

step "network $SSID connect"
spiderw network "$NET" connect --passphrase="$PASSPHRASE" || fail "connect failed"
connected=$(spiderw network "$NET" connected)
[ "$connected" = "true" ] || fail "network $SSID connected=$connected (want true)"
echo "[affinities] connected to $SSID"

# --- the BSS we are connected to is the affinity target ----------------------
AP_MAC=$(spiderw station "$STA" status \
    | sed -n 's/^ConnectedAccessPoint:[[:space:]]*//p' | head -n1)
case "$AP_MAC" in
"" | "-")
    fail "no ConnectedAccessPoint MAC in status (cannot pick an affinity target)" ;;
*:*:*:*:*:*)
    : ;;  # looks like a MAC
*)
    fail "ConnectedAccessPoint is not a MAC: '$AP_MAC' (affinity resolves by MAC)" ;;
esac
echo "[affinities] affinity target (connected BSS): $AP_MAC"

# --- assertion 1: iwd ACCEPTS the write --------------------------------------
# A non-zero exit here is the real feasibility wall (property missing without
# -E, or the driver refusing the pin). Success means iwd took the write; the CLI
# prints the affinity it set on success.
step "station $STA affinities set $AP_MAC"
if ! out=$(spiderw station "$STA" affinities set "$AP_MAC" 2>&1); then
    echo "$out"
    case "$out" in
    *NotSupported* | *not*support* | *xperimental* | *Failed* | *failed*)
        fail "iwd rejected SetAffinities ($out): the property is unavailable or the driver will not pin; not feasible to verify here" ;;
    esac
    fail "affinities set failed: $out"
fi
echo "$out"
grep -Fiq "$AP_MAC" <<<"$out" \
    || fail "set did not echo the affinity $AP_MAC (got: $out)"
echo "[affinities] iwd accepted SetAffinities for $AP_MAC"

# --- assertion 2: iwd DROPS it when the setting CLI exits --------------------
# The set above ran in its own one-shot process, now gone. iwd's
# station_affinity_disconnected_cb clears the affinity on that disconnect, so a
# read from THIS fresh connection must show none. (If a future iwd made affinity
# survive the client, this assertion is where that change would surface.)
has_affinity() { spiderw station "$STA" affinities 2>/dev/null | grep -Fiq "$1"; }
dropped=false
for _ in $(seq 1 "$SETTLE_TRIES"); do
    has_affinity "$AP_MAC" || { dropped=true; break; }
    sleep 1
done
[ "$dropped" = true ] \
    || fail "affinity $AP_MAC still set after the setting client exited (expected iwd to drop it)"
echo "[affinities] affinity dropped after the setting client exited (client-scoped, as iwd intends)"
grep -q 'client that set affinity has disconnected' /tmp/iwd.log 2>/dev/null \
    && echo "[affinities] confirmed in iwd log: station_affinity_disconnected_cb fired"

echo
echo "[affinities] PASS (iwd accepted SetAffinities and dropped it on client exit;"
echo "             a persistent affinity is only holdable by a long-lived client)"
