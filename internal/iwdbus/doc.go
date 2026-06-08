// Package iwdbus is the low-level D-Bus boundary for spiderw.
//
// It provides a small, race-safe wrapper around godbus for interacting with iwd
// over D-Bus. This layer is intentionally weakly typed: it deals in raw D-Bus
// variants, generic method calls, and signals.
//
// Responsibilities:
//   - Runtime introspection (IntrospectedObject)
//   - Safe signal subscription, fan-out, and shutdown semantics
//   - D-Bus method and property calls (org.freedesktop.DBus.Properties)
//
// Design goals:
//   - Correctness-first behavior under concurrency (Close/Emit/Register)
//   - Resilience to malformed signals and unexpected payload types
//   - Handler isolation (slow/blocked handlers do not stall the dispatcher)
//
// Higher layers (internal/core and the public spiderw API) normalize and
// validate D-Bus data into strong types and stable errors. External consumers
// should not import this package.
package iwdbus
