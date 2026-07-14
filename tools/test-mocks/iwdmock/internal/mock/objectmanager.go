package mock

import (
	"github.com/godbus/dbus/v5"

	"github.com/chrispypip/spiderw/internal/iwdbus"
)

const objectManagerPath = dbus.ObjectPath("/")

// ObjectManager implements the mock D-Bus ObjectManager interface.
//
// It holds no object slices of its own: iwd's tree changes at runtime (a forgotten
// network's object is destroyed, a provisioned one appears), so the tree is read
// live from the registries on every call. Snapshotting them at export time would
// freeze the tree and make every lifecycle transition invisible.
type ObjectManager struct {
	daemon *Daemon
}

// ExportObjectManager exports the mock ObjectManager on the D-Bus connection.
func ExportObjectManager(conn *dbus.Conn) error {
	om := &ObjectManager{}
	if !*omitDaemonFlag {
		om.daemon = &Daemon{}
	}
	// Objects added or removed after startup need this connection to export or
	// tear themselves down.
	setMockConn(conn)
	return conn.Export(om, objectManagerPath, iwdbus.DBusObjectManagerIface)
}

// GetManagedObjects returns the mock object tree in ObjectManager format.
func (o *ObjectManager) GetManagedObjects() (map[dbus.ObjectPath]map[string]map[string]dbus.Variant, *dbus.Error) {
	objects := map[dbus.ObjectPath]map[string]map[string]dbus.Variant{}

	registryMu.RLock()
	defer registryMu.RUnlock()

	if o.daemon != nil {
		props, _ := o.daemon.GetInfo()
		objects[daemonPath] = map[string]map[string]dbus.Variant{
			iwdbus.IwdDaemonIface: props,
		}
	}

	for _, adapter := range exportedAdapters {
		objects[adapter.Path] = map[string]map[string]dbus.Variant{
			iwdbus.IwdAdapterIface: adapter.buildPropertyMap(),
		}
	}

	for _, device := range exportedDevices {
		ifaces := map[string]map[string]dbus.Variant{
			iwdbus.IwdDeviceIface: device.buildPropertyMap(),
		}
		// A station-mode device advertises the Station interface too, so station
		// enumeration (Daemon.GetStations) finds it.
		if device.HasStation {
			ifaces[iwdbus.IwdStationIface] = device.buildStationPropertyMap()
		}
		// An AP-mode device advertises the AccessPoint interface, so access-point
		// enumeration (Daemon.GetAccessPoints) finds it.
		if device.HasAccessPoint {
			ifaces[iwdbus.IwdAccessPointIface] = device.buildAccessPointPropertyMap()
		}
		objects[device.Path] = ifaces
	}

	for _, bss := range exportedBSSes {
		objects[bss.Path] = map[string]map[string]dbus.Variant{
			iwdbus.IwdBasicServiceSetIface: bss.buildPropertyMap(),
		}
	}

	for _, network := range exportedNetworks {
		objects[network.Path] = map[string]map[string]dbus.Variant{
			iwdbus.IwdNetworkIface: network.buildPropertyMap(),
		}
	}

	for _, known := range exportedKnownNetworks {
		objects[known.Path] = map[string]map[string]dbus.Variant{
			iwdbus.IwdKnownNetworkIface: known.buildPropertyMap(),
		}
	}

	return objects, nil
}
