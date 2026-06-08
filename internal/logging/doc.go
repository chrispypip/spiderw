// Package logging defines the internal logging abstractions used throughout
// spiderw.
//
// Goals
//   - Provide a small Logger interface suitable for libraries.
//   - Support context-based logger injection (FromContext / WithLogger).
//   - Offer multiple implementations, including a no-op logger and a
//     structured slog-backed logger.
//   - Keep logging overhead low in hot paths, with benchmarks tracking
//     regressions.
//
// This package is internal so the public API can evolve independently of any
// specific logging backend.
package logging
