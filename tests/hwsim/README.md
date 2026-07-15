# hwsim test tier

Runs spiderw against a **real iwd** on virtual radios (`mac80211_hwsim`), instead
of the pure-Go mock. Every bug this project has shipped came from the mock being
more forgiving than iwd, so this tier exists to catch where mock and reality
diverge.

## Requirements

A host with a hwsim-enabled kernel (`CONFIG_MAC80211_HWSIM`) and Docker. This is
**not** most workstations - a stock desktop kernel usually lacks the option, and a
container cannot load a module the host kernel does not have. A cloud VM with the
module enabled, or a GitHub-hosted Ubuntu runner, works.

## What it does

- The virtual radios are created on the **host** (`modprobe mac80211_hwsim`).
- The container runs its **own** system D-Bus and iwd (see `entrypoint.sh`), so
  iwd owns `net.connman.iwd` on the container's bus, isolated from the host. It
  borrows the host's radios through the shared network namespace
  (`--network host`) and manages them with `NET_ADMIN`.
- iwd is built from source at a **pinned version** (3.12), not installed from
  apt: Ubuntu ships iwd 2.14, and the point of this tier is to meet the same
  daemon the hardware runs. The iwd tarball bundles the exact ell it was released
  with, so bumping `IWD_VERSION` in the Dockerfile is all it takes to track a new
  target.
- spiderw talks to that iwd over the system bus (its default; the mock is the
  odd one out on the session bus).

## Run it (from the repo root, on the VM)

```bash
tests/hwsim/run.sh                       # build + read-only smoke
tests/hwsim/run.sh connect.sh            # build + AP/station/connect flow
tests/hwsim/run.sh roam.sh               # build + roam flow; needs 3 radios
tests/hwsim/run.sh spiderw device list   # override the command
RADIOS=2 tests/hwsim/run.sh               # choose radio count
```

`connect.sh` takes optional `SSID`, `PASSPHRASE`, `SCAN_TRIES`; `roam.sh` adds
`WEAK_CDBM` (how far the connected AP is faded, in centi-dBm) and `ROAM_TIMEOUT`.
`run.sh roam.sh` auto-selects 3 radios and starts the hwsim medium controller
(`HWSIM_MEDIUM=1`); if `mac80211_hwsim` is already loaded with fewer radios,
reset it first (`sudo modprobe -r mac80211_hwsim`).

## Tiers

- **`smoke.sh` (read-only):** spiderw *reads* real iwd - `daemon info`,
  `adapter/device list` and `status`. Read-only, safe. The first mock-vs-reality
  check.
- **`connect.sh` (connect):** spiderw *drives* the AP + station + connect flow
  through the CLI - mode switch, AP start, scan, connect, disconnect - and asserts
  the station reaches (and leaves) the connected state. The first exercise of the
  *write* paths against the real daemon.
- **`roam.sh` (roam):** spiderw *observes a roam*. Two APs share one SSID; the
  station connects to one, then iwd's `hwsim` medium tool (`net.connman.hwsim`,
  `Rule.SignalStrength`) fades that AP so iwd roams to the other. The test asserts
  the roam *signature* through `station monitor access-point`: the associated BSS
  changes with no `access-point=none` between (a reconnect would show one; a true
  roam does not). This is the behaviour a single real radio could never test.
## Automating it

This tier is deliberately **not** wired into this repo's CI. It needs a real iwd
on a hwsim-enabled kernel, which GitHub-hosted runners do not provide, so the
only option would be a self-hosted runner - and self-hosted runners on a
**public** repo are a standing risk, because a fork's pull request could run code
on the runner.

So run it one of two ways:

- **Manually**, on a hwsim-enabled machine, as a pre-release check: `git pull`
  and run the three tiers above.
- **From a separate private repo** that registers the self-hosted runner, clones
  this (public) repo, and runs the tiers. A private repo has no anonymous forks,
  which removes the exposure entirely; it is also the right home for a
  real-hardware runner later. Point that runner at a **dedicated, disposable** VM
  (nothing sensitive on it, no reused keys or broad credentials), since the
  workload is privileged: Docker plus a `NET_ADMIN` / `--network host` container
  can reach host root.
