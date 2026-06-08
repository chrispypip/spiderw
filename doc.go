// SPDX-License-Identifier: Apache-2.0

// Package spiderw provides a safe, strongly typed Go API for interacting with
// the iwd daemon (net.connman.iwd) over D-Bus.
//
// Public callers start with Client, then access daemon and adapter operations
// through typed wrappers. The package normalizes raw D-Bus values, avoids
// exposing D-Bus types in the public API, and maps lower-level failures into
// stable public error categories.
package spiderw
