// SPDX-License-Identifier: Apache-2.0

// Package spiderw provides a safe, strongly typed Go API for interacting with
// the iwd daemon (net.connman.iwd) over D-Bus.
//
// Public callers start with Client, then reach the daemon, adapters, devices,
// stations, access points, networks, known networks, and basic service sets
// through typed wrappers. The package normalizes raw D-Bus values, avoids
// exposing D-Bus types in the public API, and maps lower-level failures into
// stable public error categories.
//
// State can be observed as events rather than polled: every object with
// properties offers a generic SubscribePropertiesChanged plus typed convenience
// subscriptions. An absent value arrives as nil - iwd signals that an object is
// gone by invalidating the property rather than sending a null path - so a
// disconnected station reports a nil connected network, and a forgotten network a
// nil known network. Watching a station's connected access point is the only way
// to observe a roam: the associated BSS changes while the state stays
// StationStateConnected and the connected network does not change at all.
//
// Credentials are supplied by registering an Agent (Client.RegisterAgent), and
// RSSI thresholds by a signal-level agent (Station.MonitorSignalLevel).
package spiderw
