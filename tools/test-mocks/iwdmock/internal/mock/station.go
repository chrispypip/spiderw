package mock

import "github.com/godbus/dbus/v5"

// The mock station-mode device (wlan0) reports a "connected" Station, wired to
// real mock objects so property reads resolve to paths the client can also
// enumerate: ConnectedNetwork points at a mock network and ConnectedAccessPoint
// (plus the single Affinities entry) points at a mock BSS. Integration tests
// assert against these literals.
const (
	stationConnectedState           = "connected"
	stationConnectedNetworkPath     = dbus.ObjectPath("/net/connman/iwd/phy0/wlan0/known_psk")
	stationConnectedAccessPointPath = dbus.ObjectPath("/net/connman/iwd/phy0/wlan0/aabbccddeeff")
)

// stationExported reports whether the Station interface should be exported on
// the station-mode device and advertised in introspection/ObjectManager.
func stationExported() bool {
	return !*omitStationFlag
}

// buildStationPropertyMap returns the mock Station interface properties. State
// and Scanning are always present; ConnectedNetwork/ConnectedAccessPoint and
// Affinities are the optional (experimental) properties.
func (d *Device) buildStationPropertyMap() map[string]dbus.Variant {
	return map[string]dbus.Variant{
		"State":                dbus.MakeVariant(d.StationState),
		"Scanning":             dbus.MakeVariant(d.StationScanning),
		"ConnectedNetwork":     dbus.MakeVariant(d.StationConnectedNetwork),
		"ConnectedAccessPoint": dbus.MakeVariant(d.StationConnectedAccessPoint),
		"Affinities":           dbus.MakeVariant(d.StationAffinities),
	}
}
