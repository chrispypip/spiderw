// Package cli implements the spiderw command-line interface for interacting
// with iwd through the spiderw public API.
//
// The CLI is designed to be:
//   - stable and script-friendly (including a --json output mode)
//   - testable in-process (Run accepts injected stdout/stderr) as well as via
//     integration tests using the iwd mock
//   - correctness-first: errors are surfaced clearly and Run returns a non-zero
//     exit code on failure
//
// The cmd/spiderw binary is a thin wrapper around Run. See TESTING.md for
// integration test details and tags.
package cli
