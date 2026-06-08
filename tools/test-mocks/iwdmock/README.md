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

---

## What This Mock *Is*

* A small executable (`package main`) at `tools/test-mocks/iwdmock`
* Registers the D-Bus name:

  ```text
  net.connman.iwd
  ```

* Exposes a small set of objects (currently: ObjectManager, daemon, and adapter)
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
|   |-- daemon.go
|   |-- export.go
|   |-- firehose.go
|   |-- objectmanager.go
|   |-- utils.go
|   `-- xml          # introspection XML served by Introspectable (go:embed)
|       |-- adapter.xml
|       `-- daemon.xml
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
* `--omit-optionals`: Export the adapter with optional fields (e.g. `Vendor` and `Model` from daemon) omitted.

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

---

## Extending the Mock

Extend the mock only when spiderw gains new features requiring more D-Bus API
coverage.

Guidelines:

* Keep behavior deterministic
* Avoid adding timing/state complexity
* Match real iwd introspection XML as closely as possible
* Add/adjust integration tests alongside new mock behavior
