// The iwdmock command is a minimal, Go-based mock of enough of the
// net.connman.iwd D-Bus API to support spiderw's integration tests.
//
// It runs as a standalone executable, connects to the *session* bus, and
// requests the "net.connman.iwd" well-known name. The process exports a
// small set of objects (currently focusing on the Daemon and Adapter
// interfaces) and can be configured via flags to emit malformed payloads,
// wrong types, or D-Bus failures.
//
// The goal is a fast, hermetic, dependency-free environment suitable for CI
// and local development, without requiring a real iwd installation or wireless
// hardware.
//
// This mock intentionally does NOT attempt to emulate iwd behavior accurately.
// Extend it only to cover new spiderw integration scenarios.
package main
