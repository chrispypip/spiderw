#!/usr/bin/env bash
# Host-side driver for the REAL-HARDWARE test tier. Runs on the Raspberry Pi
# self-hosted runner (see spiderw-test's provision-pi-runner.sh + hardware.yml).
#
#   tests/hardware/run.sh                    # build + read-only smoke
#   tests/hardware/run.sh smoke.sh           # a specific tier (in the image)
#   tests/hardware/run.sh spiderw device list   # override the command
#
# It reuses the hwsim container image (tests/hwsim/Dockerfile: spiderw + a pinned
# iwd), but drives it against the Pi's REAL brcmfmac radio, NOT virtual
# mac80211_hwsim ones. So there is deliberately NO `modprobe` here: the
# container's iwd manages the host's real wlan0 through the shared network
# namespace (`--network host` + NET_ADMIN + /dev/rfkill). Because the container
# shares the host kernel, the real driver/firmware/radio are all exercised; only
# iwd is containerized (ephemeral, version-pinned).
#
# Host prerequisites (provision-pi-runner.sh sets these up, hardware.yml checks
# them): Docker; wlan0 released by NetworkManager; the WiFi regdomain set so
# rfkill is unblocked; and the AF_ALG crypto modules pre-loaded on the host (a
# container cannot auto-load kernel modules). Docker's default seccomp profile
# blocks AF_ALG socket creation on the Pi, so `--security-opt seccomp=unconfined`
# is required; that is a safe relaxation here since the container already runs
# with NET_ADMIN + --network host + --device (it can reach host root regardless).
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

echo "[run] running against the real wlan0 (--network host, seccomp=unconfined)"
docker run --rm --network host --cap-add NET_ADMIN \
    --security-opt seccomp=unconfined \
    "${device_args[@]}" "$IMAGE" "$@"
