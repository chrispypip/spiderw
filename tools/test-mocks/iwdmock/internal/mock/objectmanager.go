package mock

import (
	"github.com/godbus/dbus/v5"

	"github.com/chrispypip/spiderw/internal/iwdbus"
)

const objectManagerPath = dbus.ObjectPath("/")

// ObjectManager implements the mock D-Bus ObjectManager interface.
type ObjectManager struct {
	adapters      []*Adapter
	devices       []*Device
	bsses         []*BasicServiceSet
	networks      []*Network
	knownNetworks []*KnownNetwork
	daemon        *Daemon
}

// ExportObjectManager exports the mock ObjectManager on the D-Bus connection.
func ExportObjectManager(conn *dbus.Conn) error {
	om := &ObjectManager{}
	if !*omitDaemonFlag {
		om.daemon = &Daemon{}
	}
	if exportedAdapters != nil {
		om.adapters = exportedAdapters
	}
	if exportedDevices != nil {
		om.devices = exportedDevices
	}
	if exportedBSSes != nil {
		om.bsses = exportedBSSes
	}
	if exportedNetworks != nil {
		om.networks = exportedNetworks
	}
	if exportedKnownNetworks != nil {
		om.knownNetworks = exportedKnownNetworks
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

	for _, adapter := range o.adapters {
		objects[adapter.Path] = map[string]map[string]dbus.Variant{
			iwdbus.IwdAdapterIface: adapter.buildPropertyMap(),
		}
	}

	for _, device := range o.devices {
		ifaces := map[string]map[string]dbus.Variant{
			iwdbus.IwdDeviceIface: device.buildPropertyMap(),
		}
		// A station-mode device advertises the Station interface too, so station
		// enumeration (Daemon.GetStations) finds it.
		if device.HasStation {
			ifaces[iwdbus.IwdStationIface] = device.buildStationPropertyMap()
		}
		objects[device.Path] = ifaces
	}

	for _, bss := range o.bsses {
		objects[bss.Path] = map[string]map[string]dbus.Variant{
			iwdbus.IwdBasicServiceSetIface: bss.buildPropertyMap(),
		}
	}

	for _, network := range o.networks {
		objects[network.Path] = map[string]map[string]dbus.Variant{
			iwdbus.IwdNetworkIface: network.buildPropertyMap(),
		}
	}

	for _, known := range o.knownNetworks {
		objects[known.Path] = map[string]map[string]dbus.Variant{
			iwdbus.IwdKnownNetworkIface: known.buildPropertyMap(),
		}
	}

	return objects, nil
}
