# real-hardware test tier

Runs spiderw against a **real iwd** on the Raspberry Pi's **real brcmfmac
radio** - the genuine driver + firmware + hardware, not the virtual
`mac80211_hwsim` radios of the [hwsim tier](../hwsim/README.md). Every bug this
project has shipped came from hardware, so this is the highest-value tier.

## How it relates to the hwsim tier

It reuses the hwsim **container image** (`../hwsim/Dockerfile`: spiderw + pinned
iwd) but a different **driver**: `run.sh` here runs that container against the
Pi's real `wlan0` via `--network host` (no `modprobe`), so the container's iwd
manages the physical radio. The container shares the host kernel, so the real
driver/firmware/radio are exercised; only iwd is containerized (ephemeral,
version-pinned).

`brcmfmac` is **fullmac** (firmware-driven roaming) versus hwsim's softmac - so
iwd behaves differently here, which is exactly the divergence worth testing.

## Constraints

- **One radio**, so tiers are **station-only against EXTERNAL APs** (the Pi
  can't self-host an AP like the hwsim connect tier does).
- The **control plane must be Ethernet**: a test reconfigures `wlan0`, which
  would drop a Wi-Fi-connected runner.

## Running it (on the Pi runner)

```bash
tests/hardware/run.sh              # build + read-only smoke
tests/hardware/run.sh smoke.sh     # a specific tier from the image
```

Host setup (Docker, `wlan0` freed from NetworkManager, the WiFi regdomain, the
pre-loaded AF_ALG crypto modules, and the `seccomp=unconfined` the run needs) is
handled by spiderw-test's `provision-pi-runner.sh` and driven by its
`hardware.yml`. See that repo for provisioning and the runner.
