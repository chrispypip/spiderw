// Command spiderw is a small CLI for interacting with iwd through the spiderw
// public API.
//
// The CLI is designed to be:
//   - stable and script-friendly (including a --json output mode)
//   - testable via black-box integration tests using the iwd mock
//   - correctness-first: errors are surfaced clearly and non-zero exit codes
//     are returned on failure
//
// See TESTING.md for integration test details and tags.
package main
