# Development

spiderw contains a development container for easy development and testing. The
development container provides:

* Go toolchain
* D-Bus session environment
* iwd mock tooling
* Testing dependencies

It intentionally does not install any specific editor or IDE tooling. Developers
may use their preferred tools locally or view remote container workflows. Ensure
all guidelines from [CONTRIBUTING.md](../CONTRIBUTING.md) are followed.

To set up the development container and prepare for development, follow these
steps:

## 1. Install Dependencies

Ensure you have:

- Docker
- Docker Compose V2
- Make

## 2. Optional: Enable mac80211_hwsim (for radio simulation)

The normal mock integration suite does not need `mac80211_hwsim`. Enable it
only if you plan to run separate radio simulation workflows:

```bash
sudo modprobe mac80211_hwsim
```

## 3. Ensure You Have Everything You Need

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

## 4. Enter the Development Environment

```bash
make dev
```

This opens an isolated shell containing the full toolchain and starts a D-Bus
session.

## 5. Build & Test

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

More testing and benchmarking commands are in [TESTING.md](../TESTING.md) and
[BENCHMARKS.md](../BENCHMARKS.md).
