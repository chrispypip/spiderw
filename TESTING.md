# Testing Strategy for spiderw

This document describes **how and why testing is structured the way it is** in
the spiderw project.

The goal is not simply high coverage, but **high confidence**: deterministic
correctness guarantees where possible, and exploratory hardening where necessary.

---

## Guiding Principles

1. **Correctness > Brevity**
   Tests should favor explicit correctness and safety over minimal code.

2. **Determinism for Promotion**
   Anything that gates a release must be deterministic and repeatable.

3. **Fuzz Untrusted Boundaries**
   Fuzz testing is used only where inputs are weakly typed, externally controlled, or structurally hard to enumerate.

4. **Strong Typing Is a Safety Boundary**
   Once data has been validated and normalized into strong Go types, fuzzing provides little additional signal.

5. **Separate Test *Intent* via Build Tags**
   spiderw intentionally uses build tags to keep fast inner-loop tests quick,
   and to make heavier suites opt-in. Build tags also keep suites organized by intent.

---

## Build Tags

Most tests in spiderw are behind explicit build tags.

| Tag | What it runs | Intended uses |
| --- | --- | --- |
| `unit` | Fast, deterministic unit tests | Default inner-loop suite |
| `regression` | Deterministic tests added after a bug is found | Prevent reintroducing known bugs |
| `race` | Scenario-style concurrency tests | Run with `-race` for maximum signal |
| `stress` | Higher-load deterministic tests | Longer-running, opt-in |
| `fuzz` | Fuzz tests | Developer-invoked, time-bounded |
| `bench` | Benchmarks | Performance exploration/regression detection |
| `integration` | D-Bus + iwd mock integration tests | End-to-end confidence |

> NOTE: `go test ./...` (with no tags) may run few or no tests, depending on what is currently tagged.

### Quick Start Commands

Each example contains two different commands: The first command is to be run
**outside** of the development container, the second runs inside.

```bash
# Fast inner-loop suite
make test-unit
go test ./... -tags=unit

# Regression inner-loop suite
make test-regression
go test ./... -tags=regression

# Race scenarios + Go race detector
make test-race
go test ./... -race -tags=race

# Stress scenarios (deterministic, higher load)
make test-stress
go test ./... -tags=stress

# Stress scenarios under the Go race detector
make test-stress-race
go test ./... -race -tags=stress
```

Tags can be combined (comma-separated), e.g. `-tags=unit,regression`.

---

## Layered Architecture and Test Responsibilities

spiderw is intentionally layered. Each layer has **different failure modes**, and therefore different testing tools.

```text
D-Bus (untrusted, weakly typed)
        |
internal/iwdbus   <- defensive boundary
        |
internal/iwdvalue <- shared canonical iwd value parsing
        |
internal/core     <- validation & normalization
        |
public API        <- strongly typed, safe surface

internal/failure  <- shared error kind/resource taxonomy used by core and public API
```

---

## internal/iwdbus

### Role

* Interfaces directly with D-Bus
* Handles weakly typed `Variant` data
* Dispatches asynchronous signals
* Must be resilient to malformed, unexpected, or hostile input

### Test Types

| Test Type | Used  | Purpose                                       |
| --------- | ----- | --------------------------------------------- |
| Unit      | yes  | Deterministic behavior and invariants         |
| Race      | yes  | Concurrency safety under normal use           |
| Stress    | yes  | Safety under load and fan-out                 |
| Fuzz      | yes  | Adversarial input and malformed signal safety |
| Benchmark | yes  | Dispatcher and handler performance analysis   |

### Stress Testing

Stress tests live behind an explicit build tag:

```go
//go:build stress
```

They may involve high concurrency and longer runtimes. They are deterministic
but not intended for tight inner-loop development.

### Fuzz Testing

Fuzz tests for `internal/iwdbus` are **high value and required**.

They assert:

* No panics
* No deadlocks
* No send-on-closed-channel
* No negative `WaitGroup` usage

Fuzz tests live behind an explicit build tag:

```go
//go:build fuzz
```

They are run intentionally, not automatically.

Example invocation:

```bash
./dev.sh go test ./internal/iwdbus \
  -tags=fuzz \
  -race \
  -fuzz=FuzzIntrospectedObject_DispatchSignal \
  -fuzztime=5m
```

Future CI should run deterministic promotion-gate tests only; fuzz tests are opt-in and developer-invoked.

### Benchmark Testing

Benchmark tests are guarded behind an explicit build tag:

```go
//go:build bench
```

Benchmarks are used to understand performance characteristics and detect
regressions during refactors. They measure relative performance and
allocation trends, not absolute throughput. They are not promotion gates.

See `BENCHMARKS.md` for instructions on running and interpreting benchmarks.

---

## internal/logging

### Role

* Provides structured logging abstractions
* Accepts arbitrary key/value input (`any`)
* Used pervasively throughout the project

### Test Types

| Test Type | Used        | Purpose                                     |
| --------- | ----------- | ------------------------------------------- |
| Unit      | yes        | Correct formatting and behavior             |
| Race      | yes        | Concurrency safety                          |
| Stress    | yes        | Load and chaining behavior                  |
| Fuzz      | yes (narrow) | Panic-freedom under garbage input         |
| Benchmark | yes        | Allocation and logging overhead measurement |

### Fuzz Testing Scope

Logging fuzz tests are **intentionally narrow**.

They assert one invariant only:

> Logging must never panic, regardless of input.

They do **not** test:

* Output correctness
* Ordering guarantees
* Performance characteristics

#### Bounded Fuzz Inputs

Logging fuzz tests **intentionally bound input size** (for example, to 256 bytes).

This bound exists to prevent *pathological input amplification* during fuzzing,
particularly when running with the race detector enabled (`-race`). Without a
bound, adversarial inputs can cause excessive allocation and scheduler pressure
that obscures real defects.

The bound is chosen to:

* Exceed realistic logging usage by a wide margin
* Preserve adversarial structure (odd kv counts, mixed types, nils)
* Avoid fuzz-harness instability (OOMs, worker termination)

This bound does **not** reduce test effectiveness; it improves signal quality.

Like iwdbus fuzz tests, logging fuzz tests are behind the `fuzz` build tag and are not promotion gates.

---

## internal/core

### Role

* Normalizes D-Bus data into strong Go types
* Enforces invariants
* Shields the public API from D-Bus quirks
* Uses `internal/failure` to classify errors by generic kind plus resource

### Test Types

| Test Type | Used  | Purpose               |
| --------- | ----- | --------------------- |
| Unit      | yes  | Invariant enforcement |
| Race      | yes  | Concurrency safety    |
| Stress    | yes  | Safety under load     |
| Fuzz      | no   | Not applicable        |
| Benchmark | no   | Not applicable        |

### Why There Is No Fuzzing

By design, `internal/core`:

* Accepts only validated, structured input
* Rejects malformed data explicitly
* Does not operate on weakly typed inputs

Any fuzz failure in core would indicate:

* A broken invariant within core, or
* A bug in an upstream boundary (iwdbus) that allowed invalid state through

For this reason, fuzz testing is **not appropriate** at this layer.

---

## internal/iwdvalue and internal/failure

### Role

These packages hold small shared value definitions used across layers:

* `internal/iwdvalue` defines canonical iwd strings and parsing/formatting
  helpers, such as adapter modes.
* `internal/failure` defines stable error kinds and resource names shared by
  core and the public API.

### Test Types

| Test Type | Used | Purpose                                  |
| --------- | ---- | ---------------------------------------- |
| Unit      | yes  | Mapping stability and invalid input cases |
| Race      | no   | No mutable shared state                   |
| Stress    | no   | Not applicable                            |
| Fuzz      | no   | Inputs are tiny closed sets               |
| Benchmark | no   | Not applicable                            |

New reusable enum or error taxonomy behavior should include unit tests here
instead of duplicating mapping tests in every layer.

---

## Wiring Layer

This layer (the `connect/` package) wires together D-Bus connections and the
layered iwd API implementation. It does not introduce new parsing logic,
weak typing, or untrusted inputs, and therefore inherits the safety
guarantees of its dependencies.

It is covered by:

* Unit tests
* Race tests (where concurrency exists)
* Regression tests (when a wiring bug is found)

Fuzzing here would provide little signal and add maintenance burden.

---

## Public API

### Role

* Provides a clean, stable, strongly typed interface
* Does not expose D-Bus types
* Intended for direct use by application code

### Test Types

| Test Type | Used  | Purpose                          |
| --------- | ----- | -------------------------------- |
| Unit      | yes  | Correct public behavior          |
| Race      | yes  | Safe concurrent usage            |
| Stress    | yes  | CLI-style and high-frequency use |
| Fuzz      | no   | Not applicable                   |
| Benchmark | no   | Not applicable                   |

### Why There Is No Fuzzing

The public API:

* Accepts only Go types
* Enforces input validation
* Cannot be meaningfully invoked with arbitrary bit patterns

Fuzzing would attempt to construct states that the API does not admit.

---

## Integration Tests (Deterministic, Environment-Dependent)

Integration tests validate the CLI and public API against the project's iwd mock
environment (D-Bus session, mock binary) and are therefore run in the dev
container.

```bash
go test ./... -tags=integration
```

These tests use the Go `iwdmock` service on the session bus. They do not require
real iwd, Wi-Fi hardware, root access, or `mac80211_hwsim`. If you prefer Make
targets or are not running in the dev container, see `make test-mock`.

---

## Promotion Model and Test Gating

spiderw follows a **promotion-based release model** locally today. GitHub
Actions CI is planned, but the dev-container Makefile workflow is currently the
source of truth for release-readiness checks.

### Promotion Gates (Deterministic)

The following must pass before a version is promoted:

```bash
go test ./...
go test ./... -tags=unit
go test ./... -tags=regression
go test ./... -tags=stress
go test ./... -race -tags=race
go test ./... -race -tags=stress
go test ./... -tags=integration
```

The Makefile exposes these through focused targets such as `make test-unit`,
`make test-race`, `make test-stress`, `make test-stress-race`, and
`make test-mock`. The broader `make test-all` target also runs benchmarks, so it
is a comprehensive local check rather than the minimal promotion gate.

These tests are:

* Deterministic
* Repeatable
* Required

Fuzz testing and benchmarks are explicitly excluded from required promotion
gates, although benchmarks may still be run locally with `make test-bench` or as
part of the broader `make test-all` target.

### Fuzz Testing (Advisory)

Fuzz testing is:

* Exploratory
* Non-deterministic
* Time-bounded

It is **not** a promotion gate.

Instead, fuzzing is used:

* During development
* During pre-release hardening

Any defect discovered via fuzzing must be converted into a deterministic regression test before release.

---

## Summary

* Fuzz testing is used **only at untrusted boundaries**
* Strong typing is treated as a safety boundary
* Promotion gates are deterministic by design
* Fuzzing informs release decisions but does not block them

This strategy prioritizes long-term correctness, maintainability, and confidence in released versions.
