package mock

import (
	"fmt"
	"time"

	"github.com/godbus/dbus/v5"

	"github.com/chrispypip/spiderw/internal/iwdbus"
)

// The mock station-mode device (wlan0) reports a "connected" Station, wired to
// real mock objects so property reads resolve to paths the client can also
// enumerate: ConnectedNetwork points at a mock network and ConnectedAccessPoint
// (plus the single Affinities entry) points at a mock BSS. Integration tests
// assert against these literals.
const (
	stationConnectedState           = "connected"
	stationConnectedNetworkPath     = dbus.ObjectPath("/net/connman/iwd/phy0/wlan0/known_psk")
	stationConnectedAccessPointPath = dbus.ObjectPath("/net/connman/iwd/phy0/wlan0/aabbccddeeff")

	// scanDuration is how long the mock keeps Scanning=true before completing, so
	// subscribers observe the true->false transition without slowing tests.
	scanDuration = 40 * time.Millisecond
)

// mockOrderedNetwork is one (network object path, signal) tuple of
// GetOrderedNetworks; godbus marshals a slice of these to a(on).
type mockOrderedNetwork struct {
	Path   dbus.ObjectPath
	Signal int16
}

// stationOrderedNetworks is the seeded scan result: the mock networks with
// signal strengths in units of 100 * dBm, strongest first.
var stationOrderedNetworks = []mockOrderedNetwork{
	{Path: "/net/connman/iwd/phy0/wlan0/known_psk", Signal: -6000},
	{Path: "/net/connman/iwd/phy0/wlan0/open", Signal: -7200},
	{Path: "/net/connman/iwd/phy0/wlan0/secured_psk", Signal: -8000},
}

// stationExported reports whether the Station interface should be exported on
// the station-mode device and advertised in introspection/ObjectManager.
func stationExported() bool {
	return !*omitStationFlag
}

// buildStationPropertyMap returns the mock Station interface properties. State
// and Scanning are always present; ConnectedNetwork/ConnectedAccessPoint and
// Affinities are the optional (experimental) properties. Reads under the mutex
// because Scan and SetAffinities mutate this state from other goroutines.
func (d *Device) buildStationPropertyMap() map[string]dbus.Variant {
	d.stationMu.Lock()
	defer d.stationMu.Unlock()
	return map[string]dbus.Variant{
		"State":                dbus.MakeVariant(d.StationState),
		"Scanning":             dbus.MakeVariant(d.StationScanning),
		"ConnectedNetwork":     dbus.MakeVariant(d.StationConnectedNetwork),
		"ConnectedAccessPoint": dbus.MakeVariant(d.StationConnectedAccessPoint),
		"Affinities":           dbus.MakeVariant(d.StationAffinities),
	}
}

// Scan implements net.connman.iwd.Station.Scan. It models the async scan: set
// Scanning true and emit, then after scanDuration flip it back to false and emit
// again -- so subscribers observe the transition. A scan already in progress is
// rejected with Busy.
func (d *Device) Scan() *dbus.Error {
	if !d.HasStation {
		return dbus.MakeFailedError(fmt.Errorf("device has no station interface"))
	}

	d.stationMu.Lock()
	if d.StationScanning {
		d.stationMu.Unlock()
		return dbus.NewError(iwdbus.IwdErrorBusy, []interface{}{"scan already in progress"})
	}
	d.StationScanning = true
	d.stationMu.Unlock()
	d.emitStationPropertiesChanged(map[string]dbus.Variant{"Scanning": dbus.MakeVariant(true)})

	go func() {
		time.Sleep(scanDuration)
		d.stationMu.Lock()
		d.StationScanning = false
		d.stationMu.Unlock()
		d.emitStationPropertiesChanged(map[string]dbus.Variant{"Scanning": dbus.MakeVariant(false)})
	}()
	return nil
}

// GetOrderedNetworks implements net.connman.iwd.Station.GetOrderedNetworks,
// returning the seeded scan results as a(on).
func (d *Device) GetOrderedNetworks() ([]mockOrderedNetwork, *dbus.Error) {
	if !d.HasStation {
		return nil, dbus.MakeFailedError(fmt.Errorf("device has no station interface"))
	}
	return stationOrderedNetworks, nil
}

// setStationAffinities stores a new Affinities value and emits a change. Called
// from the Properties.Set handler for the Station interface.
func (d *Device) setStationAffinities(paths []dbus.ObjectPath) {
	d.stationMu.Lock()
	d.StationAffinities = paths
	d.stationMu.Unlock()
	d.emitStationPropertiesChanged(map[string]dbus.Variant{"Affinities": dbus.MakeVariant(paths)})
}

// emitStationPropertiesChanged emits a PropertiesChanged signal on the Station
// interface for the device path, so subscription tests observe changes.
func (d *Device) emitStationPropertiesChanged(changed map[string]dbus.Variant) {
	emitPropertiesChanged(d.Path, iwdbus.IwdStationIface, changed, []string{})
}
