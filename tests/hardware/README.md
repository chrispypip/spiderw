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

## Tiers

- **`smoke.sh` (shared, read-only):** reused from the hwsim tier - it only
  *reads* iwd, so it is radio-agnostic. The first mock-vs-reality check on the
  real driver.
- **`connect` (hardware-only, `tiers/connect.sh`):** drives the single real
  station through mode-station -> scan -> `Network.Connect` (with a passphrase)
  -> disconnect against an **external** AP, asserting it reaches and leaves the
  connected state. The write paths, on real brcmfmac. No AP is created (one
  radio), so it needs the target AP supplied by env: `SSID`, `PASSPHRASE` (for a
  PSK network), optional `SECURITY=psk|open` and `SCAN_TRIES`.
- **`known-network` (hardware-only, `tiers/known-network.sh`):** connecting to a
  PSK network makes iwd *save* a profile; this drives its whole lifecycle on
  real hardware - asserts it appears in `known-network list`/`status` with the
  right properties, *survives a disconnect*, that `autoconnect` round-trips and
  its change subscription fires, and that `forget` removes it. Same external-AP
  env as `connect`. (Unlike the hwsim tier it can't stop the AP, so it drops
  `autoconnect` before forgetting to avoid an auto-reconnect race.)
- **`signal` (hardware-only, `tiers/signal.sh`):** registers a
  `SignalLevelAgent` (`station monitor-signal <dBm>...`) on the connected link
  and asserts iwd delivers the initial callback and spiderw decodes it into a
  valid band index + range string. Same external-AP env plus `THRESHOLDS` and
  `REG_TIMEOUT`. *Live* band tracking as the signal moves needs iwd's
  multi-threshold CQM (`CQM_RSSI_LIST`), which fullmac brcmfmac is not expected
  to deliver, so this asserts the register/callback/decode path, not live
  tracking - that would need a softmac (mt76/iwlwifi) station later.

The `connect`, `known-network`, and `signal` tiers share `tiers/common.sh`
(`resolve_sta`, which waits for iwd to enumerate the real radio, and
`sta_path`), sourced via `. "$(dirname "${BASH_SOURCE[0]}")/common.sh"`.

Shared hwsim tiers live in `../hwsim/tiers/` and are baked onto `PATH` in the
image; hardware-only tiers live in `tiers/` here and are baked into a separate
image dir, so a hardware tier can share a name with an hwsim one without
colliding. `run.sh` resolves a bare tier name to the right one.

## Running it (on the Pi runner)

```bash
tests/hardware/run.sh              # build + read-only smoke
tests/hardware/run.sh smoke.sh     # the shared read-only smoke, explicitly
SSID=lab-ap PASSPHRASE=secret \
  tests/hardware/run.sh connect    # station connect against your lab AP
```

The `connect` tier connects `wlan0` to a **real external AP**, so run it on a
runner whose control plane is **Ethernet** (per the constraint above) and point
it at your lab router. Wiring it into `hardware.yml` with the SSID/passphrase
from repo secrets is the CI step (deferred with the rest of the hardware
runner's GitHub Actions setup); run it by hand against the lab AP until then.

Host setup (Docker, `wlan0` freed from NetworkManager, the WiFi regdomain, the
pre-loaded AF_ALG crypto modules, and the `seccomp=unconfined` the run needs) is
handled by spiderw-test's `provision-pi-runner.sh` and driven by its
`hardware.yml`. See that repo for provisioning and the runner.
