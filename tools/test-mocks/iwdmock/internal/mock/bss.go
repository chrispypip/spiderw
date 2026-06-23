package mock

import (
	"fmt"

	"github.com/godbus/dbus/v5"

	"github.com/chrispypip/spiderw/internal/iwdbus"
)

// bssFixtures defines the mock basic service sets exported under the device.
//
// iwd reports one BSS per access point/radio a device can hear during a scan, so
// the mock exports several to exercise multi-BSS enumeration. Paths are listed
// in sorted order to match the daemon's path-sorted enumeration.
var bssFixtures = []struct {
	path    dbus.ObjectPath
	address string
}{
	{path: "/net/connman/iwd/phy0/wlan0/aabbccddeeff", address: "11:22:33:44:55:66"},
	{path: "/net/connman/iwd/phy0/wlan0/bbccddeeff00", address: "77:88:99:aa:bb:cc"},
}

var exportedBSSes []*BasicServiceSet

// BasicServiceSet represents the mock iwd BasicServiceSet interface exported on
// D-Bus. The real interface is read-only with a single Address property.
type BasicServiceSet struct {
	// Path is the D-Bus object path this BSS is exported at.
	Path dbus.ObjectPath

	// Address is the mock Address (BSSID) property.
	Address string
}

// ExportBasicServiceSet exports the mock BSS objects on the D-Bus connection.
//
// When --omit-bss is set, no BSS objects are exported (and the ObjectManager
// will report no basic service sets), which exercises empty enumeration.
func ExportBasicServiceSet(conn *dbus.Conn) error {
	if *omitBSSFlag {
		return nil
	}

	exportedBSSes = nil
	for _, fixture := range bssFixtures {
		b := &BasicServiceSet{
			Path:    fixture.path,
			Address: fixture.address,
		}

		// Export methods.
		if err := conn.Export(b, b.Path, iwdbus.IwdBasicServiceSetIface); err != nil {
			return err
		}
		if err := conn.Export(b, b.Path, "org.freedesktop.DBus.Properties"); err != nil {
			return err
		}
		if err := exportBSSIntrospection(conn, b.Path); err != nil {
			return err
		}

		exportedBSSes = append(exportedBSSes, b)
	}

	return nil
}

func (b *BasicServiceSet) buildPropertyMap() map[string]dbus.Variant {
	return map[string]dbus.Variant{
		"Address": dbus.MakeVariant(b.Address),
	}
}

// GetAll returns all mock BSS properties for the requested interface.
func (b *BasicServiceSet) GetAll(iface string) (map[string]dbus.Variant, *dbus.Error) {
	if iface != iwdbus.IwdBasicServiceSetIface {
		return nil, dbus.MakeFailedError(fmt.Errorf("unknown property %q", iface))
	}
	return b.buildPropertyMap(), nil
}

// Get returns a single mock BSS property for the requested interface.
func (b *BasicServiceSet) Get(iface, p string) (dbus.Variant, *dbus.Error) {
	if iface != iwdbus.IwdBasicServiceSetIface {
		return dbus.Variant{}, dbus.MakeFailedError(fmt.Errorf("unknown property %q", iface))
	}

	props := b.buildPropertyMap()
	v, ok := props[p]
	if !ok {
		return dbus.Variant{}, dbus.MakeFailedError(fmt.Errorf("unknown property %q", p))
	}
	return v, nil
}
