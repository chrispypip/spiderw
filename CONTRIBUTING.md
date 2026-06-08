# Contributing to spiderw

Thank you for your interest in contributing!
spiderw uses a containerized editor-agnostic development workflow so anyone on
any Linux distribution can contribute reliably.

---

## Code of Conduct

Participation in spiderw project spaces is covered by the
[Code of Conduct](CODE_OF_CONDUCT.md). Be respectful, keep technical feedback
focused on the work, and follow maintainer direction.

---

## Contributions

spiderw is currently in an early design phase. The public API and internal
architecture are still being shaped through a small number of vertical slices.

At this stage, I am not generally accepting pull requests for new features,
new public APIs, or major refactors.

Issues are welcome, especially for:

- bug reports
- documentation problems
- unclear API behavior
- CLI usability feedback
- test failures
- ideas for future iwd vertical slices

Small pull requests for typo fixes or documentation corrections may be
considered, but please open an issue first before spending time on a PR.

This policy may change once the core API shape is more stable.

---

## Contribution Philosophy

spiderw is correctness-first.

Contributions are evaluated primarily on:

* Correctness and safety
* Clear ownership of invariants
* Robust behavior under concurrency
* Maintainability over cleverness

Performance improvements are welcome only when they preserve these guarantees.

---

## Commit Messages

spiderw uses a lightweight Conventional Commits style:

```text
<type>(scope): <summary>
```

Examples:

- `feat(adapter): add powered monitor command`
- `fix(iwdbus): discover adapters via ObjectManager`
- `test(adapter): add unsubscribe mock coverage`
- `docs(readme): update CLI examples`

Common types:

- `feat`: user-facing feature
- `fix`: bug fix
- `docs`: documentation-only change
- `test`: tests-only change
- `ci`: CI or GitHub workflow change
- `refactor`: behavior-preserving code cleanup
- `build`: build/dependency/tooling change
- `chore`: maintenance that does not fit another type

Keep the summary in the imperative mood when possible, for example
`fix(adapter): handle nil mode list`, not `fixed adapter nil mode list`.

---

## Development Environment

spiderw contains a development container for easy development and testing. The
development container provides:

* Go toolchain
* D-Bus session environment
* iwd mock tooling
* Testing dependencies

It intentionally does not install any specific editor or IDE tooling. Developers
may use their preferred tools locally or via remote container workflows. Ensure
all guidelines from [CONTRIBUTING.md](CONTRIBUTING.md) are followed.

### 1. Requirements

- Docker
- Docker Compose V2
- Make
- **Optional:** `mac80211_hwsim` kernel module
  - Not required for the normal mock integration suite
  - Required only if you intend to do separate simulated 802.11 radio work
  - Enable with:

  ```bash
  sudo modprobe mac80211_hwsim
  ```

If `mac80211_hwsim` is not enabled, hardware-level radio simulation workflows
will be unavailable, but the Go iwd mock integration tests can still run.

### 2. Enter the Dev Shell

```bash
make dev
```

This gives you:

- A pinned Go toolchain
- A D-Bus session environment
- All project dependencies
- A consistent, reproducible development runtime

### 3. Validate Everything

From the host, run the full preflight check:

```bash
make preflight
```

Inside an already-open dev shell, you can run only the container-side check:

```bash
./dev/scripts/preflight-container.sh
```

The host check verifies Docker and Compose availability. The container check
verifies the D-Bus session environment and iwd mock tooling.

---

## Coding Standards

- Use `go fmt`, `gofumpt`, or editor-integrated formatting
- Write idiomatic, readable Go
- Keep packages focused and cohesive
- All new functionality **must include tests**
- Public APIs must be **strongly typed**
  - D-Bus values must be decoded internally
  - Return concrete Go types (`string`, `bool`, enums, etc.)
  - Never return `dbus.Variant` to callers
- New functionality must be fully mockable using the Go-based iwd mock

---

## Strong Typing Guidelines

spiderw wraps a weakly typed D-Bus/iwd API. To ensure correctness and stability:

- Decode D-Bus properties into Go types immediately
- Validate types in the decoding layer
- Reject unexpected D-Bus types with helpful errors
- Expose only strongly typed public methods
- Convenience accessors must not silently coerce incorrect types

This design helps detect upstream changes early and makes spiderw's API easier
to use and maintain.

---

## Mapping and Error Guidelines

Shared iwd values and error categories should be defined once and translated at
layer boundaries.

- Put canonical iwd enum values and parsing helpers in `internal/iwdvalue`.
- Keep layer-specific public/core enum aliases thin and generated from the
  shared canonical values where practical.
- Put shared failure categories and resource names in `internal/failure`.
- Classify errors with a generic kind plus resource, for example
  `KindUnavailable` with `ResourceAdapter`.
- Do not add new combined kind or sentinel names such as
  `KindDeviceUnavailable` or `ErrStationUnavailable`.
- Public callers should use `errors.Is(err, spiderw.ErrUnavailable)` for the
  category and `errors.As` with `*spiderw.Error` to inspect `Resource`, `Kind`,
  `Op`, and `Details`.
- Resource-specific convenience wrappers are acceptable inside a layer when
  they make call sites clearer, but they should delegate to the shared
  taxonomy.

This keeps daemon, adapter, device, station, and future network APIs from
growing duplicate enum maps or a separate unavailable error for every object
type.

---

## Directory Layout

```text
/internal/iwdbus
    Low-level D-Bus binding, signal dispatch, and iwd object wrappers

/internal/core
    Normalization, validation, and layer-specific error wrapping

/internal/connect
    Client wiring for D-Bus connections and typed object construction

/internal/failure
    Shared error kind/resource taxonomy used by core and the public API

/internal/iwdvalue
    Shared canonical iwd enum parsing and formatting helpers

/internal/logging
    Lightweight structured logging helpers

/tools/test-mocks/iwdmock
    Go-based iwd mock and D-Bus introspection XML fixtures

/tests/integration
    Mock-backed integration tests for CLI and public API behavior

/dev/scripts
    Host and container preflight scripts
```

---

## Testing Expectations

ALL contributions must include appropriate tests.

Depending on the change, this may include:

* Unit tests
* Race and/or stress tests for concurrency changes
* Integration tests
* Regression tests for fixed bugs

Use the dev container for integration tests.

See [TESTING.md](TESTING.md) for details on test types and build tags.

---

## Performance Considerations

If a change affects:

* Signal dispatch
* Handler fan-out
* Logging hot paths
* Concurrency primitives

A benchmark update or comparison may be requested.

See [BENCHMARKS.md](BENCHMARKS.md) for guidance.

---

## Out of Scope

The following are generally out of scope without prior discussion:

* API-breaking changes
* Performance-only refactors without correctness motivation
* Large rewrites without incremental validation
* Style-only changes without functional benefit

---

## Submitting Changes

spiderw is not generally accepting unsolicited pull requests yet. Open an issue
first so the design and scope can be discussed before you spend time on a PR.

If a PR has been discussed and is appropriate:

1. Fork the repository.
2. Create a feature branch.

   ```bash
   git checkout -b feature/my-change
   ```

3. Make changes inside the `make dev` environment.
4. Run the relevant test targets. At minimum, run:

   ```bash
   make fmt-check
   make lint-check
   make test-unit
   ```

5. For behavior changes, also run the relevant regression, race, stress, and
   mock integration targets described in [TESTING.md](TESTING.md).
6. Submit the pull request and include the commands you ran.

Be sure to include relevant tests and ensure preflight checks pass.

---

## Reporting Issues

Please include:

- Host OS and kernel version
- Docker + Compose versions
- Steps to reproduce
- Relevant logs:

  ```bash
  make preflight
  docker-compose logs spiderw-dev
  ```

Thank you for helping improve **spiderw!**
