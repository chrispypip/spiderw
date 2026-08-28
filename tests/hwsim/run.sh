#!/usr/bin/env bash
# NATIVE host-side driver for the hwsim tiers - runs them directly on the VM,
# with no Docker (mac80211_hwsim would not work under Docker on the test VMs).
#
#   tests/hwsim/run.sh                 # bring iwd up + run the read-only smoke
#   tests/hwsim/run.sh connect.sh      # a named tier under tiers/
#   tests/hwsim/run.sh roam.sh         # (auto-selects 3 radios + hwsim medium)
#   tests/hwsim/run.sh spiderw device list   # any command, once iwd is up
#   RADIOS=2 tests/hwsim/run.sh        # override the radio count
#   SPIDERW_VERSION=v0.14.0 tests/hwsim/run.sh   # test a published binary
#
# Each call reloads the virtual radios and starts a FRESH iwd, so one call is a
# clean slate for one tier - the same isolation `docker run --rm` gave the
# container path (used for the hardware tier, tests/hardware/run.sh). lib.sh does
# the bring-up/teardown; the tier scripts under tiers/ are unchanged between paths.
#
# Needs root (iwd, the state wipe, and modprobe all do); it re-execs under sudo.
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$HERE/../.." && pwd)"
TIERS_DIR="$HERE/tiers"

# Re-exec under sudo if not already root. --preserve-env carries the knobs a
# caller (or the CI step) may have set; iwd, the /var/lib/iwd wipe, and modprobe
# all require root.
if [ "$(id -u)" -ne 0 ]; then
    echo "[run] re-executing under sudo (iwd + modprobe need root)"
    exec sudo --preserve-env=RADIOS,SPIDERW_VERSION,IWD_DEBUG,IWD_EXPERIMENTAL,HWSIM_MEDIUM \
        bash "${BASH_SOURCE[0]}" "$@"
fi

# shellcheck source=tests/hwsim/lib.sh
. "$HERE/lib.sh"

# Decide the radio count and per-tier env from the first argument (the tier), the
# way the container's run.sh chose `docker run -e` flags. roam and
# ordered-networks need three radios (two APs + a station) and the hwsim medium
# controller for per-link RSSI; affinities needs iwd's experimental API. Other
# tiers run on two radios with no extra flags.
default_radios=2
first="${1:-smoke.sh}"
case "$first" in
roam.sh | */roam.sh | ordered-networks.sh | */ordered-networks.sh)
    default_radios=3
    export HWSIM_MEDIUM=1 IWD_DEBUG=1
    ;;
affinities.sh | */affinities.sh)
    export IWD_EXPERIMENTAL=1 IWD_DEBUG=1
    ;;
esac
RADIOS="${RADIOS:-$default_radios}"

# Make the spiderw binary available on PATH for the tiers (they call it by name).
# Priority: an already-installed spiderw (the CI builds it once up front); else a
# published release when SPIDERW_VERSION is set (verifying the exact artifact);
# else a build from the checked-out source. Mirrors the Dockerfile's logic.
ensure_spiderw() {
    if command -v spiderw >/dev/null 2>&1; then
        log "using spiderw already on PATH: $(command -v spiderw)"
        return 0
    fi

    # Outside the checkout, so a root-owned build cannot dirty the repo or be
    # committed by accident. The CI installs spiderw to a PATH dir up front, so
    # this build branch is a local-dev / fallback convenience.
    local bindir="${TMPDIR:-/tmp}/spiderw-hwsim-bin"
    mkdir -p "$bindir"

    if [ "${SPIDERW_VERSION:-}" != "" ]; then
        local arch url
        arch="$(dpkg --print-architecture 2>/dev/null || uname -m)"
        case "$arch" in
        armhf | armel) arch=arm ;;
        x86_64) arch=amd64 ;;
        aarch64) arch=arm64 ;;
        esac
        url="https://github.com/chrispypip/spiderw/releases/download/${SPIDERW_VERSION}/spiderw_${SPIDERW_VERSION#v}_linux_${arch}.tar.gz"
        log "downloading published spiderw $SPIDERW_VERSION ($url)"
        curl -fsSL "$url" | tar -xz -C "$bindir" spiderw \
            || fail "could not download/extract $url"
    else
        local go
        go="$(command -v go || echo /usr/local/go/bin/go)"
        [ -x "$go" ] || fail "go not found; bake Go into the VM image, or set SPIDERW_VERSION"
        log "building spiderw from source with $go"
        ( cd "$REPO_ROOT" \
            && CGO_ENABLED=0 "$go" build -trimpath -o "$bindir/spiderw" ./cmd/spiderw ) \
            || fail "spiderw build failed"
    fi
    export PATH="$bindir:$PATH"
    log "using spiderw at $bindir/spiderw"
}

[ -e /dev/rfkill ] || log "WARNING: /dev/rfkill missing; iwd needs it and may not start"

trap cleanup EXIT
ensure_spiderw
hwsim_reload "$RADIOS"
iwd_start

# Resolve what to run. A *.sh argument is a tier under tiers/ (by name or path);
# anything else is a literal command to run once iwd is up (e.g. `spiderw device
# list`), matching the container entrypoint's exec-the-args behaviour.
if [ $# -eq 0 ]; then
    set -- bash "$TIERS_DIR/smoke.sh"
elif [ "${1%.sh}" != "$1" ]; then
    tier="$1"; shift
    case "$tier" in
    */*) : ;;                       # an explicit path - use as given
    *) tier="$TIERS_DIR/$tier" ;;   # a bare name - resolve under tiers/
    esac
    [ -f "$tier" ] || fail "tier not found: $tier"
    set -- bash "$tier" "$@"
fi

log "running: $*"
"$@"
