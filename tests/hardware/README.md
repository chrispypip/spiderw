# real-hardware test tier

Runs spiderw against a **real iwd** on a DUT's **real radio** (brcmfmac,
iwlwifi, ...) - the genuine driver + firmware + hardware, not the virtual
`mac80211_hwsim` radios of the [hwsim tier](../hwsim/README.md). Every bug this
project has shipped came from hardware, so this is the highest-value tier.

## How it relates to the hwsim tier

It reuses the hwsim **container image** (`../hwsim/Dockerfile`: spiderw + pinned
iwd) but a different **driver**: `run.sh` here runs that container against the
DUT's real `wlan0` via `--network host` (no `modprobe`), so the container's iwd
manages the physical radio. The container shares the host kernel, so the real
driver/firmware/radio are exercised; only iwd is containerized (ephemeral,
version-pinned).

A **fullmac** DUT (e.g. `brcmfmac`, firmware-driven roaming) makes iwd behave
differently from hwsim's softmac - exactly the divergence worth testing. A
**softmac** DUT (e.g. `iwlwifi`) is closer to hwsim, but a real driver still
exercises paths the virtual radio cannot.

## Constraints

- **One radio**, so tiers are **station-only against EXTERNAL APs** (a
  single-radio DUT can't self-host an AP like the hwsim connect tier does).
- The **control plane must be Ethernet**: a test reconfigures `wlan0`, which
  would drop a Wi-Fi-connected runner.

## Tiers

- **`smoke.sh` (shared, read-only):** reused from the hwsim tier - it only
  *reads* iwd, so it is radio-agnostic. The first mock-vs-reality check on the
  real driver.
- **`connect` (hardware-only, `tiers/connect.sh`):** drives the single real
  station through mode-station -> scan -> `Network.Connect` (with a passphrase)
  -> disconnect against an **external** AP, asserting it reaches and leaves the
  connected state. The write paths, on the real driver. No AP is created (one
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
- **`wrong-passphrase` (hardware-only, `tiers/wrong-passphrase.sh`):** asserts
  the FAILURE path against the real AP's 4-way handshake - a wrong (valid-length)
  passphrase must fail with a non-empty error and no phantom connect, leave the
  station disconnected, then RECOVER with the correct passphrase. `SSID` +
  `PASSPHRASE` (the correct one) required; `BAD_PASSPHRASE` optional.
- **`power-toggle` (hardware-only, `tiers/power-toggle.sh`):** `SetPowered`
  off/on on the device *and* adapter, asserting the property round-trips and the
  radio is usable again (a scan works) - real rfkill + driver firmware
  down/up. No AP or credentials needed.
- **`ordered-networks` (hardware-only, `tiers/ordered-networks.sh`):** uses the
  genuine ambient RF - `station networks` (GetOrderedNetworks) over the real APs
  in range, asserting the list is monotonically descending by signal, signals
  decode to dBm, and SSIDs resolve. `MIN_NETWORKS` (default 2); optional `SSID`
  must appear if set. No connect/credentials needed.
- **`resolved-refs` (hardware-only, `tiers/resolved-refs.sh`):** connects, then
  asserts every cross-object ref in the status bundles resolves to a friendly
  identifier (SSID, connected BSSID/MAC, station name, adapter name) and is
  mutually consistent (the connected BSSID appears in the network's advertised
  BSSes). Same external-AP env as `connect`.
- **`affinities` (hardware-only, `tiers/affinities.sh`):** connects, then sets
  `Station.Affinities` to the connected BSS and asserts the write lifecycle - iwd
  accepts the set and drops it when the one-shot setting client exits
  (client-scoped by design). Affinities is `[experimental]`, so `run.sh` starts
  iwd with `-E` for this tier. On fullmac brcmfmac iwd may *reject* the pin; that
  rejection is reported as a feasibility wall (a finding), not a pass. Same
  external-AP env as `connect`.
- **`wpa3` (hardware-only, `tiers/wpa3.sh`):** connects to a **WPA3-only** (SAE)
  AP and asserts iwd genuinely negotiated **SAE** (from the iwd log) - the plain
  `connect` tier can't tell WPA3-SAE from WPA2-PSK because iwd reports
  `Type: psk` for both. `run.sh` starts iwd with `-d` for this tier so the SAE
  lines are captured. Point it at a **WPA3-only** SSID (a WPA2/WPA3-*transition*
  AP is what fails SAE on brcmfmac; 6GHz is out - brcmfmac has no 6GHz band).
  Same env as `connect` (`PASSPHRASE` = the SAE password).
- **`hidden` (hardware-only, `tiers/hidden.sh`):** drives `ConnectHiddenNetwork`
  (a directed probe) against a truly **hidden** AP - it first confirms the SSID
  is *absent* from a broadcast scan, then connect-hidden connects. Unlike the
  hwsim tier (iwd AP mode can't hide the SSID), the real AP genuinely suppresses
  the beacon. **Requires the AP configured hidden** (hostapd
  `ignore_broadcast_ssid=1`); if the SSID is visible, iwd's rejection is reported
  as a feasibility wall. Same external-AP env as `connect`, plus `CONNECT_TRIES`.

All hardware tiers share `tiers/common.sh` (`resolve_sta`, which waits for iwd
to enumerate the real radio, and `sta_path`), sourced via
`. "$(dirname "${BASH_SOURCE[0]}")/common.sh"`.

Shared hwsim tiers live in `../hwsim/tiers/` and are baked onto `PATH` in the
image; hardware-only tiers live in `tiers/` here and are baked into a separate
image dir, so a hardware tier can share a name with an hwsim one without
colliding. `run.sh` resolves a bare tier name to the right one.

## Running it (on the DUT runner)

```bash
tests/hardware/run.sh              # build + read-only smoke
tests/hardware/run.sh smoke.sh     # the shared read-only smoke, explicitly
SSID=lab-ap PASSPHRASE=secret \
  tests/hardware/run.sh connect    # station connect against your lab AP
```

The `connect` tier connects `wlan0` to a **real external AP**, so run it on a
runner whose control plane is **Ethernet** (per the constraint above), and point
it at your lab router with `SSID`/`PASSPHRASE` in the environment (as shown
above). You can run it by hand, or drive it from CI.

Host setup (Docker, a radio at `wlan0` freed from NetworkManager, the WiFi
regdomain, the pre-loaded AF_ALG crypto modules, and the `seccomp=unconfined`
the run needs) is the job of whatever provisions the runner. This tier depends
only on those prerequisites, not on any particular provisioning - so it is
self-contained here. (Our own lab runs it from a separate private runner repo
that provisions the hosts and schedules the jobs, but that is orchestration, not
a dependency of this tier.)
