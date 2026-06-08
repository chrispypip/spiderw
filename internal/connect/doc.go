// Package connect wires together the layered spiderw implementation.
//
// It is responsible for:
//   - establishing a D-Bus connection (system or session)
//   - constructing the internal iwdbus daemon, wrapping it with internal/core
//   - returning a Wiring bundle that higher layers can use
//
// The connect package intentionally contains minimal logic: it should not
// introduce new parsing rules or domain invariants. Those belong in
// internal/core, while raw D-Bus behavior belongs in internal/iwdbus.
package connect
