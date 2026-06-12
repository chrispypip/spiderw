package mock

import (
	"fmt"

	"github.com/godbus/dbus/v5"

	"github.com/chrispypip/spiderw/internal/iwdbus"
)

const devicePath = dbus.ObjectPath("/net/connman/iwd/phy0/wlan0")

var exportedDevice *Device

// Device represents the mock iwd Device interface exported on D-Bus.
type Device struct {
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

// ExportDevice exports the mock device object on the D-Bus connection.
//
// When --omit-device is set, no device object is exported (and the
// ObjectManager will report no devices), which exercises empty enumeration.
func ExportDevice(conn *dbus.Conn) error {
	if *omitDeviceFlag {
		return nil
	}

	d := &Device{
		Name:    "wlan0",
		Address: "aa:bb:cc:dd:ee:ff",
		Powered: true,
		Mode:    "station",
		Adapter: adapterPath,
	}

	exportedDevice = d

	// Export methods.
	if err := conn.Export(d, devicePath, iwdbus.IwdDeviceIface); err != nil {
		return err
	}
	if err := conn.Export(d, devicePath, "org.freedesktop.DBus.Properties"); err != nil {
		return err
	}
	return exportDeviceIntrospection(conn)
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
		emitPropertiesChanged(devicePath, iwdbus.IwdDeviceIface, map[string]dbus.Variant{"Powered": dbus.MakeVariant(b)}, []string{})
		return nil
	case "Mode":
		s, ok := v.Value().(string)
		if !ok {
			return dbus.MakeFailedError(fmt.Errorf("property Mode must be a string, got %T", v))
		}
		d.Mode = s
		emitPropertiesChanged(devicePath, iwdbus.IwdDeviceIface, map[string]dbus.Variant{"Mode": dbus.MakeVariant(s)}, []string{})
		return nil
	default:
		return dbus.MakeFailedError(fmt.Errorf("cannot set property %q", p))
	}
}
