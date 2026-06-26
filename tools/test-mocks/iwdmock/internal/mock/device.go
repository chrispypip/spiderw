package mock

import (
	"fmt"

	"github.com/godbus/dbus/v5"

	"github.com/chrispypip/spiderw/internal/iwdbus"
)

// devicePath is the path of the primary mock device. The mock
// networks/BSSes/known networks hang under it, and the firehose emitters target
// it.
const devicePath = dbus.ObjectPath("/net/connman/iwd/phy0/wlan0")

// device1Path is the path of a second mock device, on the second adapter,
// exercising multi-device enumeration.
const device1Path = dbus.ObjectPath("/net/connman/iwd/phy1/wlan1")

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
			Path:    devicePath,
			Name:    "wlan0",
			Address: "aa:bb:cc:dd:ee:ff",
			Powered: true,
			Mode:    "station",
			Adapter: adapterPath,
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
		if err := exportDeviceIntrospection(conn, d.Path); err != nil {
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

// GetAll returns all mock device properties for the requested interface.
func (d *Device) GetAll(iface string) (map[string]dbus.Variant, *dbus.Error) {
	if iface != iwdbus.IwdDeviceIface {
		return nil, dbus.MakeFailedError(fmt.Errorf("unknown property %q", iface))
	}
	return d.buildPropertyMap(), nil
}

// Get returns a single mock device property for the requested interface.
func (d *Device) Get(iface, p string) (dbus.Variant, *dbus.Error) {
	if iface != iwdbus.IwdDeviceIface {
		return dbus.Variant{}, dbus.MakeFailedError(fmt.Errorf("unknown property %q", iface))
	}

	props := d.buildPropertyMap()
	v, ok := props[p]
	if !ok {
		return dbus.Variant{}, dbus.MakeFailedError(fmt.Errorf("unknown property %q", p))
	}
	return v, nil
}

// Set stores supported mock device properties and emits matching change signals.
func (d *Device) Set(iface, p string, v dbus.Variant) *dbus.Error {
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
