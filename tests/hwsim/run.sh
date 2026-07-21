#!/usr/bin/env bash
# Host-side driver for the hwsim smoke. Run from the repo root, on the VM.
#
#   tests/hwsim/run.sh                 # build + run the read-only smoke
#   tests/hwsim/run.sh connect.sh      # build + run the connect flow
#   tests/hwsim/run.sh roam.sh         # build + run the roam flow
#   tests/hwsim/run.sh spiderw device list   # override the command
#   RADIOS=2 tests/hwsim/run.sh        # choose how many virtual radios
#
# The virtual radios are created on the HOST; the container borrows them through
# the shared network namespace (--network host) and manages them with NET_ADMIN.
set -euo pipefail

# The roam tier (roam.sh) needs three radios (two APs + a station) and the hwsim
# medium controller. Detect it and set both, unless the caller overrode RADIOS.
default_radios=2
env_args=()
for arg in "$@"; do
    case "$arg" in
    roam.sh | */roam.sh)
        default_radios=3
        env_args+=(-e HWSIM_MEDIUM=1 -e IWD_DEBUG=1)
        ;;
    esac
done

RADIOS="${RADIOS:-$default_radios}"
IMAGE="${IMAGE:-spiderw-hwsim}"

if ! lsmod | grep -q '^mac80211_hwsim'; then
    echo "[run] loading mac80211_hwsim radios=$RADIOS (needs sudo)"
    sudo modprobe mac80211_hwsim "radios=$RADIOS"
else
    echo "[run] mac80211_hwsim already loaded; leaving it as-is"
    echo "      (reset with: sudo modprobe -r mac80211_hwsim)"
fi

# SPIDERW_VERSION (optional): a released tag like v0.14.0. When set, the image
# downloads that published binary instead of building from source, so the tier
# verifies the exact release artifact. Unset builds from the checked-out source.
build_args=()
if [ -n "${SPIDERW_VERSION:-}" ]; then
    build_args+=(--build-arg "SPIDERW_VERSION=$SPIDERW_VERSION")
    echo "[run] building $IMAGE (published spiderw $SPIDERW_VERSION)"
else
    echo "[run] building $IMAGE (spiderw from source)"
fi
docker build -f tests/hwsim/Dockerfile "${build_args[@]}" -t "$IMAGE" .

# iwd's rfkill module needs /dev/rfkill. --network host shares the net namespace
# but not device nodes, so pass it through. (mac80211_hwsim radios register with
# rfkill, so the host has it whenever the radios exist.)
device_args=()
if [ -e /dev/rfkill ]; then
    device_args+=(--device /dev/rfkill)
else
    echo "[run] WARNING: /dev/rfkill missing on host; iwd needs it and will not start"
fi

echo "[run] running (--network host --cap-add NET_ADMIN ${device_args[*]} ${env_args[*]})"
docker run --rm --network host --cap-add NET_ADMIN \
    "${device_args[@]}" "${env_args[@]}" "$IMAGE" "$@"
