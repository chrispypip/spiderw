# Benchmark Workflow

This document describes how to run and compare benchmarks in spiderw.

Benchmarks are **diagnostic tools**, not pass/fail tests. They are used to
understand performance characteristics and detect regressions during refactors.

---

## Benchmark Scope

Benchmarks in spiderw are intended to measure **relative performance changes**
and detect **regressions** in hot paths.

They are used to:

* Catch accidental slowdowns or allocation regressions
* Compare alternative implementations during refactors
* Validate that concurrency and isolation mechanisms remain inexpensive

They are **NOT** used to:

* Maximize absolute throughput
* Compare spiderw against other D-Bus libraries
* Drive premature micro-optimizations

Correctness, safety, and clarity always take priority over benchmark results.

---

## Benchmark Areas

Each benchmark exists to protect a specific invariant, such as:

* O(1) dispatch in the single-handler case
* Linear scaling with handler count
* No unbounded goroutine creation
* No hidden allocations in logging hot paths

### internal/iwdbus

Benchmarks cover:

* Signal dispatch fast paths (single handler)
* Fan-out behavior with multiple handlers
* Wildcard vs exact handler resolution
* Dispatcher startup and shutdown
* Registration overhead under contention

These benchmarks ensure that concurrency safety and isolation do not introduce
excessive overhead.

### internal/logging

Benchmarks cover:

* Baseline logging cost with no fields
* Structured logging with small and large field sets
* Allocation behavior
* Deep `With()` chaining

These benchmarks validate that logging remains inexpensive in hot paths.

---

## Running Benchmarks

Benchmarks are guarded by the `bench` build tag and must be run explicitly.

Examples:

```bash
# Run all iwdbus benchmarks
go test ./internal/iwdbus -tags=bench -bench=. -benchmem

# Run all logging benchmarks
go test ./internal/logging -tags=bench -bench=. -benchmem

# Run ALL benchmarks
go test ./... -tags=bench -bench=. -benchmem
```

The `Makefile` contains convenient helpers to run the tests when not in the
development container:

```bash
make test-bench
```

It is recommended to:

* Run benchmarks on an idle system
* Close background applications
* Avoid running inside CI unless explicitly desired

Benchmark results may vary across Go versions; comparisons should be made using
the same toolchain where possible.

Running benchmarks with `-count` can help smooth noise when comparing
results.

---

## Benchmark Stability Expectations

Benchmark results are expected to vary slightly between runs.

On the same machine:

* Variance within ~5-10% is normal
* Allocation counts should be stable
* Larger swings may indicate scheduler noise or system load

A change should be investigated if:

* `alloc/op` increases unexpectedly
* `ns/op` increases by more than ~20% in hot paths
* Multiple related benchmarks regress together

Benchmarks are evaluated as **trends**, not absolutes.

---

## Comparing Results with benchstat

To compare benchmark results before and after a change, use `benchstat`
(from `golang.org/x/perf/cmd/benchstat`).

### Install benchstat

```bash
go install golang.org/x/perf/cmd/benchstat@latest
```

### Capture Baseline Results

```bash
make test-all > before.txt

```
OR if running in the development container:

```bash
go test ./internal/iwdbus -tags=bench -bench=. -benchmem > before.txt
go test ./internal/logging -tags=bench -bench=. -benchmem >> before.txt
```

### Capture Results After Changes

```bash
make test-all > after.txt
```

OR if running in the development container:

```bash
go test ./internal/iwdbus -tags=bench -bench=. -benchmem > after.txt
go test ./internal/logging -tags=bench -bench=. -benchmem >> after.txt
```

### Compare

```bash
benchstat before.txt after.txt
```

### How to Interpret Output

Focus on:

* Large percentage changes
* Changes in `allocs/op`
* Changes in `ns/op`
* Systematic regressions across multiple benchmarks

Small fluctuations are expected and usually not actionable. A result is
suspicious if:

* Allocation counts increase
* Latency doubles without justification
* Regressions appear across multiple benchmarks

`benchstat` output should be interpreted holistically rather than
benchmark-by-benchmark.

---

## Benchmark Philosophy

Benchmarks in spiderw are intentionally limited to:

* Hot paths
* Core abstractions
* Concurrency-sensitive mechanisms

They are not used as promotion gates and should not fail automated builds.

Their purpose is to inform design decisions and catch accidental regressions over time.

---

## When to Add a Benchmark

Add a benchmark when:

* Introducing new concurrency primitives
* Refactoring signal dispatch or handler logic
* Modifying logging internals
* Adding a new hot-path API

Do not add benchmarks for:

* One-time setup code
* Error handling paths
* Mock or test-only utilities
