package mock

import (
	"fmt"

	"github.com/godbus/dbus/v5"

	"github.com/chrispypip/spiderw/internal/iwdbus"
)

const daemonPath = dbus.ObjectPath("/net/connman/iwd")

// Daemon represents the iwd Daemon interface mock.
type Daemon struct{}

// ExportDaemon registers the mock Daemon on the session bus.
func ExportDaemon(conn *dbus.Conn) error {
	if !*omitDaemonFlag {
		d := &Daemon{}
		// Export Daemon interface
		if err := conn.Export(d, daemonPath, iwdbus.IwdDaemonIface); err != nil {
			return err
		}
		// Export Properties interface
		if err := conn.Export(d, daemonPath, "org.freedesktop.DBus.Properties"); err != nil {
			return err
		}
		// Export Introspectable interface
		return exportDaemonIntrospection(conn)
	}
	return nil
}

// GetInfo implements net.connman.iwd.Daemon.GetInfo.
// Returns a{sv}.
func (d *Daemon) GetInfo() (map[string]dbus.Variant, *dbus.Error) {
	if *daemonFailCallsFlag {
		return nil, dbus.MakeFailedError(fmt.Errorf("mock daemon failure"))
	}
	if *daemonBadPayloadFlag {
		// "Malformed" map: required fields missing and wrong inner types.
		return map[string]dbus.Variant{
			"Version":                     dbus.MakeVariant(999),
			"StateDirectory":              dbus.MakeVariant(false),
			"NetworkConfigurationEnabled": dbus.MakeVariant("bad"),
			"ExtraField":                  dbus.MakeVariant([]int{1, 2, 3}),
		}, nil
	}

	props := map[string]dbus.Variant{}
	if !*omitDaemonVersionFlag {
		if *daemonBadVersionFlag {
			props["Version"] = dbus.MakeVariant(12345)
		} else {
			props["Version"] = dbus.MakeVariant("1.0.0")
		}
	}
	if !*omitDaemonStateDirFlag {
		if *daemonBadStateDirFlag {
			props["StateDirectory"] = dbus.MakeVariant(false)
		} else {
			props["StateDirectory"] = dbus.MakeVariant("/test/iwd/state")
		}
	}
	if !*omitDaemonNetConfFlag {
		if *daemonBadNetConfFlag {
			props["NetworkConfigurationEnabled"] = dbus.MakeVariant("yes")
		} else {
			props["NetworkConfigurationEnabled"] = dbus.MakeVariant(true)
		}
	}
	if *daemonExtraFieldFlag {
		props["ExtraField"] = dbus.MakeVariant("ignored")
	}

	return props, nil
}

// GetAll implements org.freedesktop.DBus.Properties.GetAll.
func (d *Daemon) GetAll(iface string) (map[string]dbus.Variant, *dbus.Error) {
	props, _ := d.GetInfo()
	return props, nil
}

// Get implements org.freedesktop.DBus.Properties.Get.
func (d *Daemon) Get(iface, property string) (dbus.Variant, *dbus.Error) {
	props, _ := d.GetInfo()
	if v, ok := props[property]; ok {
		return v, nil
	}
	return dbus.Variant{}, nil
}

// Set implements org.freedesktop.DBus.Properties.Set.
// It accepts the value but doesn't store anything in this mock.
func (d *Daemon) Set(iface, property string, v dbus.Variant) *dbus.Error {
	return nil
}
