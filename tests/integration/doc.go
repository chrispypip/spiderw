// Package integration documents the conventions for the spiderw integration
// test suite. It contains no test code itself; the suites live in
// resource-scoped subpackages (e.g. tests/integration/iwdbus).
//
// # What integration tests are for
//
// Integration tests run spiderw end-to-end against a lightweight Go-based iwd
// D-Bus mock (see tools/test-mocks/iwdmock and tests/testutil/iwdmock). Their
// job is to prove the wiring works across a real D-Bus round trip - not to
// re-verify behavior that fast, fake-backed unit tests already cover. They are
// gated behind the "integration" build tag and require a session bus, e.g.:
//
//	dbus-run-session -- go test -tags integration ./tests/...
//
// Each test builds and spawns the mock binary, so the suite is comparatively
// expensive. Keep it lean: prefer representative cases and push exhaustive
// matrices down to unit tests.
//
// # Test layering convention
//
// A spiderw resource (daemon, adapter, device, ...) is exposed at several layers,
// and each layer tests a distinct contract. Cover them in this order of
// priority, and do not duplicate the same assertion across layers:
//
//  1. Public Go API - the baseline. The library is the primary product surface
//     and carries typed return values and structured errors (*spiderw.Error
//     kinds/resources, errors.Is sentinels, nil optionals, subscription
//     callbacks, concurrency). Every public method gets representative
//     integration coverage through *spiderw.Client against the real bus.
//
//  2. CLI - a thin layer on top, covering only CLI-specific behavior: command
//     routing, output rendering (human and --json), argument validation, and
//     error/exit-code mapping. Roughly one happy-path plus one error-path per
//     command; do NOT re-test every API value/error through string matching.
//
//  3. Raw iwdbus - added only where a layer-specific mechanism warrants it,
//     chiefly D-Bus signal plumbing (PropertiesChanged subscriptions, firehose).
//     A resource with no signals or properties does not need this layer: for
//     example the daemon is a single property-less, signal-less GetInfo method,
//     and its raw method-call mechanics are already exercised by the
//     introspection tests (introspectmock_test.go) and its enumeration mechanics
//     by the public-API Adapters/Devices tests.
//
// # What belongs in unit tests instead
//
// Exhaustive value and error matrices - every property getter, every malformed
// or wrong-typed reply, every classification of a wrapped error - belong in the
// unit tests at the iwdbus, core, and public layers, which run with fakes and
// no subprocess. Integration keeps one representative case per behavior (e.g. a
// single malformed-reply test proving normalization fires end-to-end) and
// leaves the rest to those unit tests.
//
// # Constraints
//
// Production code MUST NOT import this package or the integration subpackages;
// they exist solely for testing and development workflows.
package integration
