// Package integration contains the spiderw integration test suite.
//
// These tests are intentionally separated from production packages. They run
// spiderw end-to-end against a lightweight Go-based iwd D-Bus mock and validate
// the behavior of the public surface (including the CLI) under realistic D-Bus
// interaction patterns.
//
// The integration suite typically:
//   - starts the iwd mock (see tests/testutil/iwdmock)
//   - runs the spiderw CLI as a black box (e.g. `go run cmd/spiderw ...`)
//   - prefers `--json` output for success-path assertions
//   - asserts on stable error substrings for failure-path behavior
//
// Production code MUST NOT import this package.
// It exists solely for testing and development workflows.
//
// See tools/test-mocks/iwdmock for the mock executable entrypoint.
package integration
