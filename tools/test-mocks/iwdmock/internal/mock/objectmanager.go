package mock

import (
	"github.com/godbus/dbus/v5"

	"github.com/chrispypip/spiderw/internal/iwdbus"
)

const objectManagerPath = dbus.ObjectPath("/")

// ObjectManager implements the mock D-Bus ObjectManager interface.
type ObjectManager struct {
	adapter *Adapter
	device  *Device
	bsses   []*BasicServiceSet
	daemon  *Daemon
}

// ExportObjectManager exports the mock ObjectManager on the D-Bus connection.
func ExportObjectManager(conn *dbus.Conn) error {
	om := &ObjectManager{}
	if !*omitDaemonFlag {
		om.daemon = &Daemon{}
	}
	if exportedAdapter != nil {
		om.adapter = exportedAdapter
	}
	if exportedDevice != nil {
		om.device = exportedDevice
	}
	if exportedBSSes != nil {
		om.bsses = exportedBSSes
	}
	return conn.Export(om, objectManagerPath, iwdbus.DBusObjectManagerIface)
}

// GetManagedObjects returns the mock object tree in ObjectManager format.
func (o *ObjectManager) GetManagedObjects() (map[dbus.ObjectPath]map[string]map[string]dbus.Variant, *dbus.Error) {
	objects := map[dbus.ObjectPath]map[string]map[string]dbus.Variant{}

	if o.daemon != nil {
		props, _ := o.daemon.GetInfo()
		objects[daemonPath] = map[string]map[string]dbus.Variant{
			iwdbus.IwdDaemonIface: props,
		}
	}

	if o.adapter != nil {
		objects[adapterPath] = map[string]map[string]dbus.Variant{
			iwdbus.IwdAdapterIface: o.adapter.buildPropertyMap(),
		}
	}

	if o.device != nil {
		objects[devicePath] = map[string]map[string]dbus.Variant{
			iwdbus.IwdDeviceIface: o.device.buildPropertyMap(),
		}
	}

	for _, bss := range o.bsses {
		objects[bss.Path] = map[string]map[string]dbus.Variant{
			iwdbus.IwdBasicServiceSetIface: bss.buildPropertyMap(),
		}
	}

	return objects, nil
}
