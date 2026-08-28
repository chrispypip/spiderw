# Shared helpers for the real-hardware tiers (station-only, against an EXTERNAL
# AP). Sourced by each tier via `. "$(dirname "${BASH_SOURCE[0]}")/common.sh"`;
# in the image both live in /usr/local/lib/spiderw-hardware/. Not a tier itself.

# resolve_sta - print the first NAMED station device, WAITING for iwd to
# enumerate it. iwd claims its D-Bus name before the phy is fully up, and a real
# radio (brcmfmac firmware load) can lag a beat, so `device list` may be briefly
# empty right after iwd starts. A device line is "name<TAB>path"; requiring a
# real iwd object path in field 2 means the CLI's "no devices available" message
# (which has no path) can never be mistaken for a device. Returns 1 on timeout.
resolve_sta() {
    local name tries="${DEVICE_TRIES:-20}" k
    for ((k = 0; k < tries; k++)); do
        name=$(spiderw device list 2>/dev/null \
            | awk -F'\t' 'NF>=2 && $2 ~ /^\/net\/connman\/iwd/ {print $1; exit}')
        [ "$name" != "" ] && { printf '%s\n' "$name"; return 0; }
        sleep 1
    done
    return 1
}

# sta_path DEVICE - print the iwd object path for a named device.
sta_path() {
    spiderw device list | awk -F'\t' -v d="$1" '$1 == d {print $2; exit}'
}

# net_path_for STA_PATH SSID - the iwd network object path for SSID, scoped to
# the given station device path so a same-SSID network on ANOTHER device is
# never picked. Empty if SSID is not in that station's last scan.
net_path_for() {
    spiderw network list 2>/dev/null \
        | awk -F'\t' -v s="$2" -v pfx="$1/" \
              '$1 == s && index($2, pfx) == 1 { print $2; exit }'
}

# connect_sta STA STA_PATH - scan for $SSID under this station, connect, and
# confirm it reached the connected state; prints the network object path on
# success and returns 0, or returns 1 on failure. Progress goes to STDERR so
# stdout is just the path:
#   NET=$(connect_sta "$STA" "$STA_PATH") || fail "could not connect to $SSID"
#
# The connect is RETRIED. A real-RF connect can fail transiently even after the
# scan saw the AP - a 4-way handshake timeout, or (for the signal-tracking rig)
# a breathing AP caught in a low-power dip - and the scan already proved the AP
# is reachable, so a fresh attempt usually lands. Tunables from the environment:
#   SSID PASSPHRASE SECURITY  the target (SECURITY = psk | open)
#   SCAN_TRIES    scans to wait for the AP to appear         (default 15)
#   CONNECT_TRIES connect attempts before giving up          (default 3)
#   CONNECT_WAIT  seconds between connect attempts            (default 4)
connect_sta() {
    local sta="$1" sta_path="$2"
    local ssid="${SSID:-}" pass="${PASSPHRASE:-}" sec="${SECURITY:-psk}"
    local scan_tries="${SCAN_TRIES:-15}"
    local connect_tries="${CONNECT_TRIES:-3}" connect_wait="${CONNECT_WAIT:-4}"
    local net="" i

    [ "$ssid" != "" ] || { echo "connect_sta: SSID is empty" >&2; return 1; }

    # Wait for the AP to appear in a scan under this station.
    for ((i = 0; i < scan_tries; i++)); do
        echo "  [connect_sta] scan $((i + 1))/$scan_tries for '$ssid'" >&2
        spiderw station "$sta" scan >/dev/null 2>&1 || true
        net=$(net_path_for "$sta_path" "$ssid")
        [ "$net" != "" ] && break
        sleep 2
    done
    [ "$net" != "" ] || {
        echo "connect_sta: '$ssid' never visible under $sta after" \
             "$scan_tries scans (AP off, out of range, wrong regdomain?)" >&2
        return 1
    }

    # Connect, retrying transient failures; re-resolve the path each pass in case
    # the scan cache churned.
    for ((i = 0; i < connect_tries; i++)); do
        net=$(net_path_for "$sta_path" "$ssid")
        if [ "$net" = "" ]; then
            spiderw station "$sta" scan >/dev/null 2>&1 || true
            sleep "$connect_wait"; continue
        fi
        if [ "$sec" = "open" ]; then
            spiderw network "$net" connect >/dev/null 2>&1 || true
        else
            spiderw network "$net" connect --passphrase="$pass" \
                >/dev/null 2>&1 || true
        fi
        if [ "$(spiderw network "$net" connected 2>/dev/null)" = "true" ]; then
            printf '%s\n' "$net"
            return 0
        fi
        echo "  [connect_sta] connect $((i + 1))/$connect_tries failed;" \
             "retrying" >&2
        sleep "$connect_wait"
    done
    echo "connect_sta: connect to '$ssid' failed after $connect_tries tries" \
         "(wrong passphrase, AP rejected the station, or signal too weak?)" >&2
    return 1
}
