package iwdbus

import (
	"context"

	"github.com/godbus/dbus/v5"
)

// ObjectTree is a snapshot of iwd's managed objects, used to resolve an object
// path to the human-friendly identifier iwd exposes on that object (a network's
// SSID, a BSS's address, a device/adapter/known-network Name). It backs the
// friendly-reference enrichment of the public Properties bundles.
type ObjectTree struct {
	objects map[dbus.ObjectPath]map[string]map[string]dbus.Variant
}

// FetchObjectTree retrieves the current iwd object tree via a single
// ObjectManager.GetManagedObjects call.
func FetchObjectTree(ctx context.Context, conn *dbus.Conn) (*ObjectTree, error) {
	objects, err := getManagedObjects(ctx, conn, IwdService)
	if err != nil {
		return nil, WrapIntrospection(DBusObjectManagerGetManagedObjects, err)
	}
	return &ObjectTree{objects: objects}, nil
}

// stringProp returns the string value of prop on iface at path, if present.
func (t *ObjectTree) stringProp(path, iface, prop string) (string, bool) {
	if t == nil {
		return "", false
	}
	ifaces, ok := t.objects[dbus.ObjectPath(path)]
	if !ok {
		return "", false
	}
	props, ok := ifaces[iface]
	if !ok {
		return "", false
	}
	v, ok := props[prop]
	if !ok {
		return "", false
	}
	s, ok := v.Value().(string)
	if !ok {
		return "", false
	}
	return s, true
}

// NetworkName returns the SSID (Name) of the network object at path.
func (t *ObjectTree) NetworkName(path string) (string, bool) {
	return t.stringProp(path, IwdNetworkIface, "Name")
}

// DeviceName returns the Name of the device object at path.
func (t *ObjectTree) DeviceName(path string) (string, bool) {
	return t.stringProp(path, IwdDeviceIface, "Name")
}

// AdapterName returns the Name of the adapter object at path.
func (t *ObjectTree) AdapterName(path string) (string, bool) {
	return t.stringProp(path, IwdAdapterIface, "Name")
}

// KnownNetworkName returns the Name of the known-network object at path.
func (t *ObjectTree) KnownNetworkName(path string) (string, bool) {
	return t.stringProp(path, IwdKnownNetworkIface, "Name")
}

// BSSAddress returns the Address (BSSID) of the basic-service-set object at path.
func (t *ObjectTree) BSSAddress(path string) (string, bool) {
	return t.stringProp(path, IwdBasicServiceSetIface, "Address")
}
