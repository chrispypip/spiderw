# spiderw

[![CI](https://github.com/chrispypip/spiderw/actions/workflows/ci.yml/badge.svg)](https://github.com/chrispypip/spiderw/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/chrispypip/spiderw.svg)](https://pkg.go.dev/github.com/chrispypip/spiderw)
[![Go Report Card](https://goreportcard.com/badge/github.com/chrispypip/spiderw)](https://goreportcard.com/report/github.com/chrispypip/spiderw)
[![License: Apache 2.0](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)

**spiderw** is a Go-based library and development environment for working with
Wi-Fi interfaces, iwd, and mockable runtime behavior. It provides:

- A clean, strongly typed Go API for interacting with iwd
- A fully containerized and editor-agnostic development workflow
- A Go-based iwd mock for integration testing without real iwd, Wi-Fi hardware, or kernel modules
- Utilities and automation to ensure consistent behavior across environments

> **Project status: early development (pre-v1).** The public API is **unstable**
> and may change without notice until the first tagged release. The implemented
> surface today is intentionally small — `Client`, `Daemon`, and `Adapter`
> (powered state, identity, supported modes, and property subscriptions) — with
> much more of the iwd API planned. Issues are welcome; pull requests for new
> features are not being accepted yet (see [CONTRIBUTING](CONTRIBUTING.md)).

**Pronunciation:** **spider double u**.

**Written:** `spiderw`
**Spoken:** **spider double u**

The name keeps the Wi-Fi/Wireless **w** visible in writing and pronunciation
without changing the Go module path.

---

## License

spiderw is licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE).

---

## Features

- **Strongly typed Go API**
  D-Bus values are validated and converted into concrete Go types.
  Callers never handle `dbus.Variant` or weakly typed maps.

- **Structured errors**
  Public errors expose a stable category, resource, operation, and wrapped
  cause, so callers can use `errors.Is` and `errors.As` without parsing text.

- **Mockable runtime**
  A pure-Go iwd mock enables end-to-end and integration testing without
  requiring system iwd or root access.

- **Containerized development environment**
  Development runs inside an isolated Docker environment so contributors can use
  any editor on any Linux system without host-side dependencies.

- **Makefile-driven workflow**
  Common tasks (`make dev`, `make preflight`, `make lint-check`, etc.) ensure
  a consistent workflow across contributors.

- **Preflight validation**
  Host and container environments are checked for correctness before development
  begins.

- **Optional radio simulation via mac80211_hwsim**
  The current mock integration suite does not require Wi-Fi hardware, real iwd,
  root access, or `mac80211_hwsim`. If you are doing separate radio-level
  experiments against simulated Wi-Fi hardware, enable the Linux kernel module:

  ```bash
  sudo modprobe mac80211_hwsim
  ```

  Without the module, hardware-level Wi-Fi simulation workflows will be
  unavailable.


---

## Current Automation Status

GitHub Actions CI runs on every push to `main` and every pull request targeting
it. The pipeline builds and vets the module, runs `golangci-lint`, executes the
unit, stress, regression, and benchmark suites natively, and runs the race and
mock integration suites under a D-Bus session bus.

The same checks are available locally through the dev-container Makefile
workflow (formatting, linting, and the full test matrix). Before publishing a
release, run the relevant local targets from [TESTING.md](TESTING.md).

---

## Design Philosophy

spiderw prioritizes correctness, safety, and clarity over raw performance.

Key principles:

* All weakly typed D-Bus data is validated and normalized at the boundary
* Concurrency correctness is treated as a first-class concern
* Public APIs expose only strongly typed, stable interfaces
* Performance is monitored, but never optimized at the expense of correctness

---

## User Quick Start

```bash
go get github.com/chrispypip/spiderw
```

Example snippet:

```go
ctx := context.Background()

// SystemBus is the default; pass spiderw.SessionBus for the session bus.
client, err := spiderw.NewClient(ctx, spiderw.SystemBus)
if err != nil {
    log.Fatal(err)
}
defer client.Close()

info, err := client.Daemon().Info(ctx)
if err != nil {
    log.Fatal(err)
}

fmt.Println(info.Version)
```

### Error Handling

spiderw returns structured public errors when it can classify a failure. Use the
generic sentinel for the category and inspect the resource when the caller needs
to distinguish daemon, adapter, network, or client failures.

```go
info, err := client.Daemon().Info(ctx)
if err != nil {
    var swerr *spiderw.Error
    if errors.As(err, &swerr) {
        switch {
        case errors.Is(err, spiderw.ErrUnavailable) &&
            swerr.Resource == spiderw.ResourceDaemon:
            log.Printf("iwd daemon is unavailable: %v", err)
        case errors.Is(err, spiderw.ErrInvalidState):
            log.Printf("spiderw observed invalid daemon state: %v", err)
        default:
            log.Printf("spiderw error in %s: %v", swerr.Op, err)
        }
        return
    }
    log.Fatal(err)
}
```

The public error categories are `KindUnavailable`, `KindInvalidState`,
`KindInvalidArgument`, and `KindInternal`. Resource values include
`ResourceClient`, `ResourceDaemon`, `ResourceAdapter`, `ResourceDevice`,
`ResourceStation`, and `ResourceNetwork`.

---

## Development Quick Start

spiderw contains a development container for easy development and testing. The
development container provides:

* Go toolchain
* D-Bus session environment
* iwd mock tooling
* Testing dependencies

It intentionally does not install any specific editor or IDE tooling. Developers
may use their preferred tools locally or view remote container workflows. Ensure
all guidelines from [CONTRIBUTING.md](CONTRIBUTING.md) are followed.

To set up the development container and prepare for development, follow these
steps:

### 1. Install Dependencies

Ensure you have:

- Docker
- Docker Compose V2
- Make

### 2. Optional: Enable mac80211_hwsim (for radio simulation)

The normal mock integration suite does not need `mac80211_hwsim`. Enable it
only if you plan to run separate radio simulation workflows:

```bash
sudo modprobe mac80211_hwsim
```

### 3. Ensure You Have Everything You Need

This validates:

On the host:

- Docker available without `sudo`
- docker-compose V2 installed

In the container:

- D-Bus session bus availability
- iwd mock functional
- Required system utilities present

```bash
make preflight
```

### 4. Enter the Development Environment

```bash
make dev
```

This opens an isolated shell containing the full toolchain and starts a D-Bus
session.

### 5. Build & Test

Inside the dev shell:

```bash
go build ./...
go test ./... -tags=unit
```

From the host, the equivalent Makefile targets are:

```bash
make test-unit
make lint-check
```

More testing and benchmarking commands are in [TESTING.md](TESTING.md) and
[BENCHMARKS.md](BENCHMARKS.md).

---

## Repository Structure

```text
dev/                     -> Development files
    Dockerfile.dev       -> Dev container runtime definition
    docker-compose.yml   -> Orchestration for development environment
    dev.sh               -> Entry point for dev shell
cmd/                     -> Tooling and CLI utilities
internal/connect         -> D-Bus connection and typed object wiring
internal/core            -> Validation, normalization, and core error wrapping
internal/failure         -> Shared error kind/resource taxonomy
internal/iwdbus          -> Strongly typed D-Bus/iwd bindings
internal/iwdvalue        -> Shared canonical iwd value parsing and formatting
internal/logging         -> Lightweight structured logging helpers
tools/test-mocks/        -> Go-based iwd mock and introspection XML fixtures
tests/                   -> Integration tests and test utilities
```

---

## Strongly Typed API

Although iwd and D-Bus expose weakly typed values (`dbus.Variant,
map[string]interface{}`, etc.), **spiderw intentionally exposes a strongly
typed Go API**.

This ensures:

- Predictable types for all public methods
- Early detection of schema changes or D-Bus inconsistencies
- No Variant handling in user code
- Easier testing and API stability

D-Bus decoding is handled internally; public methods return standard Go types
(`string`, `bool`, `[]int`, etc.).

---

## CLI Quick Start

The `spiderw` command can query the daemon and adapters through the same public
API used by library callers. It uses the system bus by default, which is where
real iwd runs, so the examples below need no bus flag. The Go mock registers on
the session bus, so pass `--session` when testing against `iwdmock`.

Global flags may be placed anywhere in the command:

- `--session` uses the session D-Bus bus instead of the default system bus.
- `--json` emits JSON for commands with structured output.
- `--help` prints command help.

Daemon examples:

```bash
spiderw daemon info
spiderw daemon version
spiderw daemon state-dir
spiderw daemon net-conf
```

List adapters:

```bash
spiderw adapter list
```

Use the adapter name or path from `adapter list` as the adapter reference:

```bash
spiderw adapter phy0 powered
spiderw adapter phy0 powered true
spiderw adapter phy0 name
spiderw adapter phy0 model
spiderw adapter phy0 vendor
spiderw adapter phy0 supported-modes
spiderw adapter phy0 supports-mode station
spiderw adapter phy0 supports-station
spiderw adapter phy0 supports-ap
spiderw adapter phy0 supports-ad-hoc
spiderw adapter phy0 monitor powered
```

To target the Go mock instead of a real daemon, add `--session`:

```bash
spiderw --session daemon info
```

`monitor powered` prints the current value and then streams future powered
changes until interrupted.

---

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for contribution policy and development
instructions. Participation in project spaces is covered by the
[Code of Conduct](CODE_OF_CONDUCT.md).

---

## Further Reading

* [Roadmap](ROADMAP.md)
* [Contributing](CONTRIBUTING.md)
* [Code of Conduct](CODE_OF_CONDUCT.md)
* [Security Policy](SECURITY.md)
* [Testing Strategy](TESTING.md)
* [Benchmarking](BENCHMARKS.md)
