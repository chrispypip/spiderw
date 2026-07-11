package mock

import (
	"fmt"
	"sync"

	"github.com/godbus/dbus/v5"

	"github.com/chrispypip/spiderw/internal/iwdbus"
)

// devicePath is the path of the primary mock device. The mock
// networks/BSSes/known networks hang under it, and the firehose emitters target
// it.
const devicePath = dbus.ObjectPath("/net/connman/iwd/0/3")

// device1Path is the path of a second mock device, on the second adapter,
// exercising multi-device enumeration.
const device1Path = dbus.ObjectPath("/net/connman/iwd/1/4")

var exportedDevices []*Device

// Device represents the mock iwd Device interface exported on D-Bus.
type Device struct {
	// Path is the D-Bus object path this device is exported at.
	Path dbus.ObjectPath

	// Name is the mock Name property.
	Name string

	// Address is the mock Address (MAC) property.
	Address string

	// Powered is the mock Powered property.
	Powered bool

	// Mode is the mock Mode property.
	Mode string

	// Adapter is the mock Adapter property (owning adapter object path).
	Adapter dbus.ObjectPath

	// HasStation reports whether this device also exports the Station interface
	// (true for a station-mode device unless --omit-station is set). The Station
	// property fields below are only meaningful when HasStation is true.
	HasStation bool

	// StationState is the mock Station.State property.
	StationState string

	// StationScanning is the mock Station.Scanning property.
	StationScanning bool

	// StationConnectedNetwork is the mock Station.ConnectedNetwork property.
	StationConnectedNetwork dbus.ObjectPath

	// StationConnectedAccessPoint is the mock Station.ConnectedAccessPoint
	// property (experimental).
	StationConnectedAccessPoint dbus.ObjectPath

	// StationAffinities is the mock Station.Affinities property (experimental).
	StationAffinities []dbus.ObjectPath

	// stationMu guards the mutable Station state (Scanning, Affinities), which
	// Scan and Set mutate from goroutines godbus dispatches concurrently.
	stationMu sync.Mutex
}

// ExportDevice exports the mock device objects on the D-Bus connection. A single
// adapter can host several interfaces and a system can have several adapters, so
// the mock exports two devices (one per adapter).
//
// When --omit-device is set, no device objects are exported (and the
// ObjectManager will report no devices), which exercises empty enumeration.
func ExportDevice(conn *dbus.Conn) error {
	if *omitDeviceFlag {
		return nil
	}

	devices := []*Device{
		{
			Path:                        devicePath,
			Name:                        "wlan0",
			Address:                     "aa:bb:cc:dd:ee:ff",
			Powered:                     true,
			Mode:                        "station",
			Adapter:                     adapterPath,
			HasStation:                  stationExported(),
			StationState:                stationConnectedState,
			StationScanning:             false,
			StationConnectedNetwork:     stationConnectedNetworkPath,
			StationConnectedAccessPoint: stationConnectedAccessPointPath,
			StationAffinities:           []dbus.ObjectPath{stationConnectedAccessPointPath},
		},
		{
			Path:    device1Path,
			Name:    "wlan1",
			Address: "11:22:33:44:55:66",
			Powered: true,
			Mode:    "ap",
			Adapter: adapter1Path,
		},
	}

	exportedDevices = nil
	for _, d := range devices {
		if err := conn.Export(d, d.Path, iwdbus.IwdDeviceIface); err != nil {
			return err
		}
		if err := conn.Export(d, d.Path, "org.freedesktop.DBus.Properties"); err != nil {
			return err
		}
		// A station-mode device also carries the Station interface (gated by
		// --omit-station); the Properties handler above serves its properties.
		withWSC := d.HasStation && !*omitWSCFlag
		if d.HasStation {
			if err := conn.Export(d, d.Path, iwdbus.IwdStationIface); err != nil {
				return err
			}
			// WSC (SimpleConfiguration) is exported on the same station-mode
			// device object, mirroring iwd (gated by --omit-wsc so a station can
			// exist without WSC, like a driver that does not support it).
			if withWSC {
				if err := conn.Export(d, d.Path, iwdbus.IwdSimpleConfigurationIface); err != nil {
					return err
				}
			}
		}
		if err := exportDeviceIntrospection(conn, d.Path, d.HasStation, withWSC); err != nil {
			return err
		}
		exportedDevices = append(exportedDevices, d)
	}

	return nil
}

func (d *Device) buildPropertyMap() map[string]dbus.Variant {
	return map[string]dbus.Variant{
		"Name":    dbus.MakeVariant(d.Name),
		"Address": dbus.MakeVariant(d.Address),
		"Powered": dbus.MakeVariant(d.Powered),
		"Mode":    dbus.MakeVariant(d.Mode),
		"Adapter": dbus.MakeVariant(d.Adapter),
	}
}

// GetAll returns all mock properties for the requested interface. The device
// object serves both its Device interface and, when HasStation is set, the
// Station interface exported on the same path.
func (d *Device) GetAll(iface string) (map[string]dbus.Variant, *dbus.Error) {
	switch iface {
	case iwdbus.IwdDeviceIface:
		return d.buildPropertyMap(), nil
	case iwdbus.IwdStationIface:
		if !d.HasStation {
			return nil, dbus.MakeFailedError(fmt.Errorf("unknown property %q", iface))
		}
		return d.buildStationPropertyMap(), nil
	default:
		return nil, dbus.MakeFailedError(fmt.Errorf("unknown property %q", iface))
	}
}

// Get returns a single mock property for the requested interface (Device or, when
// HasStation is set, Station).
func (d *Device) Get(iface, p string) (dbus.Variant, *dbus.Error) {
	var props map[string]dbus.Variant
	switch iface {
	case iwdbus.IwdDeviceIface:
		props = d.buildPropertyMap()
	case iwdbus.IwdStationIface:
		if !d.HasStation {
			return dbus.Variant{}, dbus.MakeFailedError(fmt.Errorf("unknown property %q", iface))
		}
		props = d.buildStationPropertyMap()
	default:
		return dbus.Variant{}, dbus.MakeFailedError(fmt.Errorf("unknown property %q", iface))
	}

	v, ok := props[p]
	if !ok {
		return dbus.Variant{}, dbus.MakeFailedError(fmt.Errorf("unknown property %q", p))
	}
	return v, nil
}

// Set stores supported mock device properties and emits matching change signals.
func (d *Device) Set(iface, p string, v dbus.Variant) *dbus.Error {
	if iface == iwdbus.IwdStationIface {
		if !d.HasStation {
			return dbus.MakeFailedError(fmt.Errorf("device has no station interface"))
		}
		if p != "Affinities" {
			return dbus.MakeFailedError(fmt.Errorf("cannot set station property %q", p))
		}
		paths, ok := v.Value().([]dbus.ObjectPath)
		if !ok {
			return dbus.MakeFailedError(fmt.Errorf("property Affinities must be an object-path array, got %T", v.Value()))
		}
		d.setStationAffinities(paths)
		return nil
	}
	if iface != iwdbus.IwdDeviceIface {
		return nil
	}

	switch p {
	case "Powered":
		b, ok := v.Value().(bool)
		if !ok {
			return dbus.MakeFailedError(fmt.Errorf("property Powered must be a bool, got %T", v))
		}
		d.Powered = b
		emitPropertiesChanged(d.Path, iwdbus.IwdDeviceIface, map[string]dbus.Variant{"Powered": dbus.MakeVariant(b)}, []string{})
		return nil
	case "Mode":
		s, ok := v.Value().(string)
		if !ok {
			return dbus.MakeFailedError(fmt.Errorf("property Mode must be a string, got %T", v))
		}
		d.Mode = s
		emitPropertiesChanged(d.Path, iwdbus.IwdDeviceIface, map[string]dbus.Variant{"Mode": dbus.MakeVariant(s)}, []string{})
		return nil
	default:
		return dbus.MakeFailedError(fmt.Errorf("cannot set property %q", p))
	}
}
