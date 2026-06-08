// Package iwdmock provides helpers for integration tests that need a
// deterministic mock of the iwd D-Bus API.
//
// It includes:
//   - helpers to build and start the iwdmock binary inside the test environment
//   - utilities to wait for readiness (bus name acquired, READY marker)
//   - scenario configuration for malformed payloads and simulated D-Bus errors
//
// Integration tests live under tests/integration/... and typically treat the
// spiderw CLI as a black box.
package iwdmock
