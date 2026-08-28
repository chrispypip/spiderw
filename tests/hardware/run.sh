#!/usr/bin/env bash
# Host-side driver for the REAL-HARDWARE test tier. Runs on a self-hosted runner
# whose host has a real Wi-Fi radio - the DUT (device under test). It is normally
# invoked by an out-of-tree CI harness, but is self-contained: given the host
# prerequisites below, you can run it directly on any suitably configured host.
#
#   tests/hardware/run.sh                    # build + read-only smoke
#   tests/hardware/run.sh smoke.sh           # a shared hwsim tier (radio-agnostic)
#   tests/hardware/run.sh connect            # a hardware-only tier (tiers/*.sh)
#   tests/hardware/run.sh spiderw device list   # override the command
#
#   # the connect tier needs the external AP's details (no AP is created):
#   SSID=lab-ap PASSPHRASE=secret tests/hardware/run.sh connect
#
# Hardware-only tiers live in tests/hardware/tiers/ and are baked into the image
# at /usr/local/lib/spiderw-hardware/ (a separate dir so their names cannot
# collide with the hwsim tiers on PATH). A bare tier name here resolves to that
# path; smoke.sh (shared, on PATH) and raw commands pass through unchanged.
#
# It reuses the hwsim container image (tests/hwsim/Dockerfile: spiderw + a pinned
# iwd), but drives it against the host's REAL radio (brcmfmac, iwlwifi, ...), NOT
# virtual mac80211_hwsim ones. So there is deliberately NO `modprobe` here: the
# container's iwd manages the host's real wlan0 through the shared network
# namespace (`--network host` + NET_ADMIN + /dev/rfkill). Because the container
# shares the host kernel, the real driver/firmware/radio are all exercised; only
# iwd is containerized (ephemeral, version-pinned).
#
# Host prerequisites (whatever provisions the runner must ensure these; the tier
# assumes them): Docker; a radio at wlan0 released by NetworkManager; the WiFi
# regdomain set so rfkill is unblocked (where the driver honors it); and the
# AF_ALG crypto modules pre-loaded on the host (a container cannot auto-load
# kernel modules). Docker's default seccomp profile blocks AF_ALG socket creation
# on these arm64 hosts, so `--security-opt seccomp=unconfined` is required; that
# is a safe relaxation here since the container already runs with NET_ADMIN +
# --network host + --device (it can reach host root regardless).
set -euo pipefail

IMAGE="${IMAGE:-spiderw-hardware}"

# SPIDERW_VERSION (optional): a released tag like v0.14.0 makes the image
# download that published binary instead of building from source, so the tier
# verifies the exact release artifact. Unset builds from the checked-out source.
build_args=()
if [ -n "${SPIDERW_VERSION:-}" ]; then
    build_args+=(--build-arg "SPIDERW_VERSION=$SPIDERW_VERSION")
    echo "[run] building $IMAGE (published spiderw $SPIDERW_VERSION)"
else
    echo "[run] building $IMAGE (spiderw from source)"
fi
docker build -f tests/hwsim/Dockerfile "${build_args[@]}" -t "$IMAGE" .

# iwd's rfkill support needs /dev/rfkill; --network host shares the net namespace
# (where the real wlan0 lives) but not device nodes, so pass it through.
device_args=()
if [ -e /dev/rfkill ]; then
    device_args+=(--device /dev/rfkill)
else
    echo "[run] WARNING: /dev/rfkill missing on host; iwd needs it and will not start"
fi

# Resolve the command. A bare hardware-tier name (with or without .sh) that
# exists under tests/hardware/tiers/ maps to its in-image path; anything else -
# a shared hwsim tier like smoke.sh (found on PATH), or a raw command - passes
# through untouched.
HW_TIERS_LOCAL="$(cd "$(dirname "${BASH_SOURCE[0]}")/tiers" 2>/dev/null && pwd)"
HW_TIERS_IMAGE="/usr/local/lib/spiderw-hardware"
tier_name="${1:-}"                    # the tier name before any shift below
cmd=("$@")
if [ $# -ge 1 ] && [ -n "$HW_TIERS_LOCAL" ]; then
    name="${1%.sh}"
    if [ -f "$HW_TIERS_LOCAL/$name.sh" ]; then
        shift
        cmd=("$HW_TIERS_IMAGE/$name.sh" "$@")
    fi
fi

# Forward the tiers' env into the container (only the vars that are set), so a
# tier gets its SSID/PASSPHRASE and tunables. `-e VAR` passes the current value
# through (spaces and all, e.g. THRESHOLDS) without echoing a secret onto the
# command line.
env_args=()
for var in SSID PASSPHRASE SECURITY SCAN_TRIES \
           BAD_PASSPHRASE SETTLE_TRIES MIN_NETWORKS THRESHOLDS REG_TIMEOUT \
           DEVICE_TRIES TRACK_WINDOW MIN_DISTINCT_BANDS; do
    [ -n "${!var:-}" ] && env_args+=(-e "$var")
done

# The hardware tier drives ONE real physical radio, so iwd must never remove and
# recreate its interface (the entrypoint turns this into iwd's UseDefaultInterface
# setting). Without it a softmac DUT like iwlwifi is left with no netdev once this
# container's iwd exits, failing the NEXT run's preflight; it is a no-op on a
# fullmac DUT (e.g. brcmfmac), which keeps its interface regardless.
env_args+=(-e IWD_USE_DEFAULT_INTERFACE=1)

# Per-tier iwd flags. Station.Affinities is [experimental], hidden unless iwd
# starts with -E; the entrypoint turns that on for IWD_EXPERIMENTAL=1. IWD_DEBUG=1
# (-d) surfaces the detail those tiers read from /tmp/iwd.log (the affinity
# accept/drop; the SAE handshake lines the wpa3 tier asserts on).
case "$tier_name" in
    affinities | affinities.sh)
        env_args+=(-e IWD_EXPERIMENTAL=1 -e IWD_DEBUG=1)
        ;;
    wpa3 | wpa3.sh)
        env_args+=(-e IWD_DEBUG=1)
        ;;
esac

echo "[run] running against the real wlan0 (--network host, seccomp=unconfined)"
docker run --rm --network host --cap-add NET_ADMIN \
    --security-opt seccomp=unconfined \
    "${device_args[@]}" "${env_args[@]}" "$IMAGE" "${cmd[@]}"
