# hwsim test tier

Runs spiderw against a **real iwd** on virtual radios (`mac80211_hwsim`), instead
of the pure-Go mock. Every bug this project has shipped came from the mock being
more forgiving than iwd, so this tier exists to catch where mock and reality
diverge.

## Requirements

A host with a hwsim-enabled kernel (`CONFIG_MAC80211_HWSIM`). This is **not**
most workstations - a stock desktop kernel usually lacks the option. A cloud VM
with the module enabled works; the test harness runs on a dedicated, disposable
VM (see `spiderw-test`).

The tiers run **natively** on that VM - no Docker (`mac80211_hwsim` would not
work under Docker on these VMs). The VM image is baked once with what `run.sh`
needs on `PATH`: a **pinned iwd** (3.12, built from source `--enable-hwsim` so
the `hwsim` medium tool is present) and Go (to build spiderw from source). The
runner user has passwordless sudo, because `run.sh` re-execs under it - iwd, the
`/var/lib/iwd` wipe, and `modprobe` all need root. NetworkManager must not
manage the `wlan*` interfaces, and no packaged iwd service should be running
(`run.sh` stops one defensively).

> The **same tier scripts** also run in a container against the Pi's real radio
> (`tests/hardware/`, which reuses `Dockerfile` + `entrypoint.sh`). That path
> keeps Docker; only the VM/hwsim path is native. The tier scripts are identical
> across both.

## What it does

- Each `run.sh` call is one tier and its own clean slate - the isolation
  `docker run --rm` used to give. `lib.sh` reloads `mac80211_hwsim` (fresh phys
  in default station mode), wipes `/var/lib/iwd` (no leftover known-network or
  stored-AP profile), starts a **fresh** iwd (and, for roam/ordered, a fresh
  `hwsim` medium controller so no fade rule leaks), then tears it all down on
  exit. iwd's log lands at `/tmp/iwd.log`, where the tiers already look.
- iwd is **pinned** (3.12), not the distro package: Ubuntu ships 2.14, and the
  point of this tier is to meet the same daemon the hardware runs. The VM image
  builds it from the kernel.org tarball (which bundles the exact ell), the same
  version the `Dockerfile` pins for the Pi path.
- spiderw talks to that iwd over the **system** bus (its default; the mock is
  the odd one out on the session bus).

## Run it (from the repo root, on the VM)

```bash
tests/hwsim/run.sh                       # read-only smoke
tests/hwsim/run.sh connect.sh            # AP/station/connect flow
tests/hwsim/run.sh start-profile.sh      # AP from a stored profile
tests/hwsim/run.sh wrong-passphrase.sh   # failed-connect error path
tests/hwsim/run.sh resolved-refs.sh      # path->SSID/MAC/name resolver
tests/hwsim/run.sh power-toggle.sh       # device/adapter power toggle
tests/hwsim/run.sh ap-scan.sh            # AP-side scan for neighbors
tests/hwsim/run.sh roam.sh               # roam flow; auto-selects 3 radios
tests/hwsim/run.sh signal.sh             # signal-level agent flow
tests/hwsim/run.sh known-network.sh      # known-network lifecycle
tests/hwsim/run.sh hidden.sh             # connect-hidden method
tests/hwsim/run.sh affinities.sh         # SetAffinities round-trip
tests/hwsim/run.sh ordered-networks.sh   # ranked scan; auto-selects 3 radios
tests/hwsim/run.sh wsc.sh                # WSC (WPS) enrollee interface
tests/hwsim/run.sh spiderw device list   # override the command
RADIOS=2 tests/hwsim/run.sh              # override the radio count
SPIDERW_VERSION=v0.14.0 tests/hwsim/run.sh   # test a published binary
```

`run.sh` re-execs under sudo. It uses a `spiderw` already on `PATH` (the CI
installs one up front); otherwise it downloads the `SPIDERW_VERSION` release, or
builds from the checked-out source. `connect.sh` takes optional `SSID`,
`PASSPHRASE`, `SCAN_TRIES`; `roam.sh` adds `WEAK_CDBM` (how far the connected AP
is faded, in centi-dBm) and `ROAM_TIMEOUT`; `signal.sh` adds `THRESHOLDS` (the
dBm bands to register). `run.sh roam.sh` and `run.sh ordered-networks.sh`
auto-select 3 radios and start the hwsim medium controller (`HWSIM_MEDIUM=1`);
`run.sh` reloads the module every call, so a stale radio count fixes itself.

> **Destructive to the host's iwd state.** `run.sh` wipes `/var/lib/iwd` and
> reloads `mac80211_hwsim` - it is built for the disposable VM, not a workstation
> whose saved networks you care about.

## Tiers

Each tier is one script under `tiers/`; adding a tier is just dropping a new
`tiers/<name>.sh` in and running `tests/hwsim/run.sh <name>.sh`. The tier
scripts assume iwd is already up and drive spiderw against it - they do not care
whether that iwd came from the native harness or the container, which is why the
same scripts serve both paths. One level up:

- `run.sh` - the native host-side driver (VM): pick the radio count + flags,
  bring up a fresh iwd via `lib.sh`, run the tier, tear down.
- `lib.sh` - the native bring-up/teardown (reload radios, wipe state, start/stop
  iwd and the hwsim medium).
- `Dockerfile` + `entrypoint.sh` - the **container** path, used against the Pi's
  real radio (`tests/hardware/`). `entrypoint.sh` is the container init that
  brings up dbus + iwd before exec'ing the tier.

- **`smoke.sh` (read-only):** spiderw *reads* real iwd - `daemon info`,
  `adapter/device list` and `status`. Read-only, safe. The first mock-vs-reality
  check.
- **`connect.sh` (connect):** spiderw *drives* the AP + station + connect flow
  through the CLI - mode switch, AP start, scan, connect, disconnect - and asserts
  the station reaches (and leaves) the connected state. The first exercise of the
  *write* paths against the real daemon.
- **`start-profile.sh` (AP from a stored profile):** where `connect.sh` uses the
  inline `access-point start <ssid> <psk>`, this drives the distinct
  `StartProfile` path - iwd reads a stored `.ap` profile file (which can carry
  security config beyond the inline PSK form). The tier writes a minimal PSK
  profile into iwd's AP dir (`/var/lib/iwd/ap`), starts the AP from it, asserts
  it is up broadcasting the profile's SSID, and proves the profile works by
  connecting a station with its passphrase.
- **`wrong-passphrase.sh` (failed-connect error path):** every other connect uses
  the right passphrase and asserts success; this asserts a FAILURE, where the mock
  is most likely to be more forgiving than iwd. With a wrong (but valid-length)
  passphrase the AP's 4-way handshake rejects the station (iwd is both AP and
  station, so it is really enforced). The test asserts spiderw fails the connect
  with a non-empty error rather than a phantom success, leaves the station
  disconnected, and then RECOVERS - a follow-up connect with the correct
  passphrase succeeds, proving the failed attempt did not wedge iwd.
- **`resolved-refs.sh` (friendly-ref resolver):** spiderw's status bundles turn
  iwd's cross-object paths into the names they stand for - a connected network
  into its SSID, an access point into its MAC, a device into its adapter name -
  from one `GetManagedObjects` tree. That resolver is otherwise unit-tested only
  against the mock, whose object tree merely *imitates* iwd's layout. This
  tier connects a station and asserts every such ref in `station`/`network`/
  `device status` is resolved - and, against ground truth (the SSID, the AP's
  BSSID, the station name), resolved to the *right* value - so a real-iwd layout
  that differed from the mock's (leaving raw `/net/connman/iwd/...` paths) is
  caught here.
- **`roam.sh` (roam):** spiderw *observes a roam*. Two APs share one SSID; the
  station connects to one, then iwd's `hwsim` medium tool (`net.connman.hwsim`,
  `Rule.SignalStrength`) fades that AP so iwd roams to the other. The test asserts
  the roam *signature* through `station monitor access-point`: the associated BSS
  changes with no `access-point=none` between (a reconnect would show one; a true
  roam does not). This is the behaviour a single real radio could never test.
- **`ordered-networks.sh` (ranked scan):** spiderw *reads iwd's ordered
  networks*. `station networks` calls `GetOrderedNetworks`, which iwd returns
  ranked best-signal-first; spiderw resolves each object path to its SSID and
  converts the signal to dBm. Two APs on distinct SSIDs, with the medium
  controller fading one, let the test assert the ranking: the strong SSID is
  listed before the weak one, both resolved (not raw paths), with the strong
  signal numerically greater. Unlike `connect.sh`'s `network list` (unordered,
  daemon-wide), this is the ordered per-station method and its SSID resolver.
- **`signal.sh` (signal-level agent):** spiderw *registers a
  `SignalLevelAgent`*. One station connects to one AP, then `station
  monitor-signal <dBm>...` registers the agent; the test asserts iwd delivers the
  initial callback and spiderw decodes it into a valid band index and range
  string - the first exercise of that whole interface against a real daemon.
  Note: the *live* band index tracks RSSI only through iwd's multi-threshold CQM
  monitor (`CQM_RSSI_LIST`), which mac80211/hwsim does not implement (it has only
  the single-threshold monitor the roam tier uses), so band tracking as the
  signal moves is testable only on real hardware, not here.
- **`known-network.sh` (known-network lifecycle):** connecting to a PSK network
  makes iwd *save* a profile; spiderw then drives its whole lifecycle - asserts
  it appears in `known-network list`/`status` with the right properties, that it
  *survives a disconnect*, that `autoconnect` round-trips and its change
  subscription fires, and that `forget` removes it. The first exercise of the
  `KnownNetwork` interface against a real daemon.
- **`hidden.sh` (connect-hidden):** spiderw *drives `ConnectHiddenNetwork`*, a
  distinct path from `Network.Connect` in which iwd runs a *directed* probe scan
  for a named SSID rather than picking from broadcast scan results. iwd's AP mode
  cannot actually suppress the SSID in beacons, so the test stands up a normal
  (visible) AP and, without a broad station scan first, calls
  `station connect-hidden` - a real AP answers the directed probe, so the connect
  succeeds and exercises the method. If a future iwd instead rejected the call as
  not-hidden, the test reports that as the feasibility wall (a true hidden BEACON
  would need hostapd's `ignore_broadcast_ssid`, which this image does not carry).
- **`affinities.sh` (SetAffinities):** spiderw *writes* `Station.Affinities` - a
  BSS the station should stay pinned to within its connected network. Affinities
  is an iwd `[experimental]` property, so `run.sh` starts iwd with `-E`
  (`IWD_EXPERIMENTAL=1`) for this tier only. It connects a station, sets an
  affinity to the connected BSS by MAC, and asserts the *write lifecycle*: iwd
  accepts the set, then drops it when the setting client exits. That drop is by
  design - iwd ties an affinity to the D-Bus connection that set it
  (`station_affinity_disconnected_cb`), so a dead controller leaves no stale
  pin - which means the one-shot CLI cannot hold an affinity across invocations;
  only a long-lived client (the library, held open) can. So the tier proves the
  write reaches and is applied by iwd rather than a cross-invocation round-trip,
  which is not observable by design.
- **`wsc.sh` (WSC / WPS enrollee):** spiderw *drives the station's
  `SimpleConfiguration` interface* - `GeneratePin`, `PushButton`, `Cancel`. A
  completed enrollment needs an AP acting as a WPS registrar, which iwd AP mode
  is not (its `AccessPoint` interface has no WSC method) and this image has no
  hostapd, so a join cannot complete on hwsim. Short of that, the tier asserts
  the enrollee interface works: `GeneratePin` returns a valid 8-digit WPS PIN
  (correct check digit - a real PIN, not eight random digits), `PushButton`
  initiates a session that `Cancel` then aborts on iwd's side (the blocking call
  returns), and a no-op `Cancel` is rejected. The first exercise of that
  interface against a real daemon.
- **`power-toggle.sh` (Powered write path):** the last write path not covered
  elsewhere - `SetPowered`, on both the device and the adapter. It powers a
  device off (asserting the Device object persists with `Powered=false`, since
  off is not removal) and back on (asserting it is usable again - a scan works),
  then round-trips the adapter's `Powered` (a distinct `Adapter.SetPowered`) and
  checks the device returns and scans afterwards.
- **`ap-scan.sh` (AP-side scan):** the station scan paths are covered elsewhere;
  this drives the distinct AP-side scan (`AccessPoint.Scan` +
  `GetOrderedNetworks`, which iwd exports on a device in AP mode so an AP can
  survey its neighbours). A scanner in AP mode scans for a second, started AP,
  and asserts that AP's SSID comes back through `access-point <ap> networks`,
  resolved to its name with a signal and a security type.

## Automating it

This tier is deliberately **not** wired into this repo's CI. It needs a real iwd
on a hwsim-enabled kernel, which GitHub-hosted runners do not provide, so the
only option would be a self-hosted runner - and self-hosted runners on a
**public** repo are a standing risk, because a fork's pull request could run code
on the runner.

So run it one of two ways:

- **Manually**, on a hwsim-enabled disposable machine, as a pre-release check:
  `git pull` and run the tiers above.
- **From a separate private repo** that registers the self-hosted runner, clones
  this (public) repo, and runs the tiers. A private repo has no anonymous forks,
  which removes the exposure entirely; it is also the home for the real-hardware
  Pi runner. Point that runner at a **dedicated, disposable** VM (nothing
  sensitive on it, no reused keys or broad credentials), since the workload is
  privileged: the runner has passwordless sudo and iwd runs as root with
  `NET_ADMIN` over the radios, so it can reach host root regardless.
