# iwdmock

`iwdmock` is a lightweight Go-based mock of the **net.connman.iwd** D-Bus API.
It exists exclusively to support spiderw integration tests.

This mock allows spiderw's integration tests to run:

* **without iwd**
* **without Wi-Fi hardware**
* **without kernel modules such as mac80211_hwsim**.

This mock is intentionally deterministic and deliberately simple: it provides
only what spiderw needs to validate ObjectManager discovery, introspection,
method/property calls, and signal behavior.

Its introspection XML and object shapes are modeled on the **iwd 3.12** D-Bus
API, which is the iwd version spiderw is developed and tested against.

---

## What This Mock *Is*

* A small executable (`package main`) at `tools/test-mocks/iwdmock`
* Registers the D-Bus name:

  ```text
  net.connman.iwd
  ```

* Exposes a small set of objects (currently: ObjectManager, daemon (which also
  hosts the AgentManager interface), adapter, device, basic service sets,
  networks, and known networks)
* Can emit signals, including a "firehose" mode to stress the dispatcher
* Supports session bus only (it connects via `dbus.ConnectSessionBus()`)
---

## What This Mock *Is Not*

* A full iwd implementation
* A simulator for real Wi-Fi scanning
* A source of realistic timing, state transitions, authentication, or networking
* Intended for manual "feature testing" beyond spiderw's integration test suite

If you need behavior closer to real hardware, use:

* A Raspberry Pi running real iwd
* mac80211_hwsim (kernel module)
* or a dedicated "test lab" device

But for CI and local dev, this mock is ideal.

---

## Repository Structure

```text
tools/test-mocks/iwdmock/
|-- main.go          # iwdmock binary entrypoint
|-- doc.go           # package docs
|-- internal/mock    # Mock implementations of the iwd API
|   |-- adapter.go
|   |-- bss.go
|   |-- daemon.go
|   |-- device.go
|   |-- export.go
|   |-- firehose.go
|   |-- knownnetwork.go
|   |-- network.go
|   |-- objectmanager.go
|   |-- utils.go
|   `-- xml          # introspection XML served by Introspectable (go:embed)
|       |-- adapter.xml
|       |-- bss.xml
|       |-- daemon.xml
|       |-- device.xml
|       |-- knownnetwork.xml
|       `-- network.xml
`-- README.md        # This file
```

Each exported mock object registers its path and interface on the D-Bus session bus.

---

## D-Bus Requirements

`iwdmock` requires an active **session** bus.

In the dev container, the entrypoint uses `dbus-run-session`, so integration
tests (and the mock) have `DBUS_SESSION_BUS_ADDRESS` set automatically.

If `DBUS_SESSION_BUS_ADDRESS` is not set, the integration helpers will fail
early.

---

## Running the Mock Manually

Inside the dev container (recommended):

```bash
go run /workspace/tools/test-mocks/iwdmock &
```

If successful, you will see something like:

```bash
[mock-iwd] acquired name: net.connman.iwd
[mock-iwd] READY
[mock-iwd] running. Press Ctrl+C to exit.
```

To inspect available D-Bus names:

```bash
dbus-send --session --dest=org.freedesktop.DBus / \
    org.freedesktop.DBus.ListNames --print-reply
```

You should see `"net.connman.iwd"` in the output.

## Testing With spiderw

Integration tests live under:

```text
tests/integration/iwdbus/
```

These tests treat the spiderw CLI as a black box and run it like:

```bash
go run /workspace/cmd/spiderw ...
```

Most "success path" assertions use the CLI's JSON mode (`--json`) to avoid
brittle string matching.

The tests start the mock using helpers in:

```text
tests/testutil/iwdmock/
```

Those helpers:

* build `iwdmock` into `/workspace/build/iwdmock-bin`
* run it with scenario flags
* wait for `READY`
* wait for bus name registration (`net.connman.iwd`)

To run the integration suite:

```bash
go test ./... -tags=integration
```

Or, from the host using the Makefile wrapper:

```bash
make test-mock
```

See [TESTING.md](../../../TESTING.md) for the full testing matrix and tags.

---

## Scenarios / Flags

`iwdmock` supports scenario flags used by integration tests. These flags are
defined across:

* `tools/test-mocks/iwdmock/main.go` (entrypoint-only flags)
* `tools/test-mocks/iwdmock/internal/mock/` (scenario flags used by the exported mock objects)

### Entrypoint flags

* `--firehose-signals`: Emit rapid `PropertiesChanged` signals for exported mock objects to stress dispatch.
* `--omit-optionals`: Export the adapters with optional fields (`Vendor` and `Model`) omitted.

### Daemon scenario flags

* `--omit-daemon`
  Don't export the daemon interface at all (simulated "service present but API missing")
* `--omit-daemon-version`
  `Version` info in daemon is omitted
* `--omit-daemon-statedir`
  `StateDirectory` info in daemon is omitted
* `--omit-daemon-netconf`
  `NetworkConfigurationEnabled` info in daemon is omitted
> IMPORTANT: Missing boolean fields are indistinguishable from `false` once decoded.
> spiderw treats this as "defaults to false", not an error.
* `--daemon-bad-version`,
  `--daemon-bad-statedir`,
  `--daemon-bad-netconf`
  Return the wrong D-Bus variant type for a specific field
* `--daemon-extra-field`
  Include an extra unrecognized field (should be ignored by spiderw)
* `--daemon-bad-payload`
  Return a malformed payload (still a map, but with multiple wrong/invalid inner variants)
* `--daemon-fail-calls`
  Make daemon calls return a D-Bus error

### Device, basic service set, network, and known-network scenario flags

* `--omit-device`
  Don't export the device objects, exercising empty device enumeration.
* `--omit-bss`
  Don't export the basic service set objects, exercising empty BSS enumeration.
* `--omit-network`
  Don't export the network objects, exercising empty network enumeration.
* `--omit-knownnetwork`
  Don't export the known-network objects, exercising empty known-network
  enumeration.
* `--omit-agent`
  Don't export the `net.connman.iwd.AgentManager` interface, so agent
  registration is unavailable. Exercises the client's "agent manager
  unavailable" path.
* `--omit-station`
  Don't export the `net.connman.iwd.Station` interface on the station-mode
  device, so the device still exists but has no Station. Exercises the client's
  "station unavailable" path and empty station enumeration.

The mock exports multiple basic service sets by default, mirroring iwd reporting
one BSS per access point/radio a device can hear during a scan. It also exports
three networks — an open network, a known (provisioned) secured network, and an
unknown secured network — so `Network.Connect` exercises both the no-agent
success paths and the `net.connman.iwd.NoAgent` rejection. The open network's
`ExtendedServiceSet` lists both mock BSSes, demonstrating multi-BSS membership.

### Station

The station-mode device (`wlan0`) also exports the `net.connman.iwd.Station`
interface on the same object, mirroring iwd (where Station lives on the device
object in station mode). The AP-mode device (`wlan1`) does not, so station
enumeration returns exactly one station. The mock seeds a "connected" station
wired to real mock objects: `ConnectedNetwork` points at the known network,
`ConnectedAccessPoint` and the single `Affinities` entry point at a mock BSS, and
`Scanning` is `false`.

`Scan` models the asynchronous scan: it sets `Scanning` to true and emits a
`PropertiesChanged`, then flips it back to false and emits again a short moment
later — so subscribers observe the true→false transition. `GetOrderedNetworks`
returns the three mock networks with seeded signal strengths (in 100 × dBm),
strongest first. `Affinities` is writable: setting it stores the BSS paths and
emits a change.

`Disconnect` transitions the station to `disconnected` and emits a live `State`
change. `ConnectHiddenNetwork` accepts two seeded hidden SSIDs: `HiddenOpen`
(connects directly) and `HiddenSecured` (drives the same agent callback as a
secured `Network.Connect`, so it needs a registered agent supplying
`mock-secret-passphrase`, else `NoAgent`); a *visible* network name is rejected
`NotHidden` and any other `NotFound`. `GetHiddenAccessPoints` returns two seeded
hidden APs. Use `--omit-station` to drop the interface while keeping the device.

### Credentials agent (AgentManager)

The mock daemon also hosts the `net.connman.iwd.AgentManager` interface
(`RegisterAgent`/`UnregisterAgent`) on the daemon object, recording the single
registered agent the way iwd does — a second `RegisterAgent` is rejected with
`net.connman.iwd.AlreadyExists`, and unregistering an unknown path with
`net.connman.iwd.NotFound`. Connecting the unknown secured network drives the
full callback loop: the mock calls the registered agent's `RequestPassphrase`
back over D-Bus and connects only when it returns the expected passphrase
(`mock-secret-passphrase`); a wrong or declined passphrase yields
`net.connman.iwd.Failed`, and no registered agent yields
`net.connman.iwd.NoAgent`. Use `--omit-agent` to drop the interface entirely.

Two known networks are exported: one (`psk`, with a last-connected time and
auto-connect on) at the path the mock network references via its `KnownNetwork`
property — so that linkage resolves end to end — and one `hotspot` that has never
been connected to (no `LastConnectedTime`) with auto-connect off.

Two adapters (`phy0`, `phy1`) and two devices (`phy0/wlan0`, `phy1/wlan1`) are
exported by default, since real systems can have several radios and a device per
adapter. `phy0`/`wlan0` is the primary pair: the mock networks, BSSes, and known
networks hang under it, and the firehose emitters target it. `phy1` supports a
narrower mode set than `phy0`, and `wlan1` reports `phy1` as its owning adapter.

---

## Extending the Mock

Extend the mock only when spiderw gains new features requiring more D-Bus API
coverage.

Guidelines:

* Keep behavior deterministic
* Avoid adding timing/state complexity
* Match real iwd introspection XML as closely as possible
* Add/adjust integration tests alongside new mock behavior

### Fidelity principle

Be faithful to what spiderw *observes at the boundary*; do not reimplement iwd's
*internal state machine*.

Model with high fidelity:

* Property **shape** — types, and especially presence/absence (e.g. omit
  `KnownNetwork` when a network is not known; omit `LastConnectedTime` when a
  known network has never connected).
* Realistic **fixture values** so normalization and both branches get exercised
  (e.g. one known network with `AutoConnect` true and one false).
* **Method contracts and error names** for a given input (`Connect` →
  `NoAgent`/`Failed`, `RegisterAgent` → `AlreadyExists`, etc.).

Do **not** simulate iwd's stateful side effects — provisioning a `KnownNetwork`
on first connect, computing the `AutoConnect` default, object
creation/destruction, calling `Agent.Release` on a client-initiated
`UnregisterAgent` — unless a specific spiderw code path observes that transition
in the same flow.

Litmus test: *does spiderw read back the result of the side effect in the same
flow?* If yes, simulate it minimally (this is why the mock validates the secured
passphrase against a constant — spiderw's agent + `Connect` flow has both success
and failure branches to exercise). If no, simulating it tests a reimplementation
of iwd rather than spiderw, so leave it to real-hardware testing.
