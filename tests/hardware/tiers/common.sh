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
