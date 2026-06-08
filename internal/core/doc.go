// Package core normalizes raw D-Bus data from internal/iwdbus into strongly
// typed domain values and enforces invariants.
//
// Responsibilities
//   - Convert weakly typed D-Bus payloads (variants, maps, arrays) into Go
//     types expected by higher layers.
//   - Enforce invariants (non-empty fields, valid object paths, expected value
//     types) so the public API never has to defend against malformed D-Bus
//     inputs.
//   - Provide domain-specific error types that distinguish invalid remote
//     state from transport issues.
//
// core is the "safety boundary" between the raw D-Bus layer and the stable
// spiderw public API.
package core
