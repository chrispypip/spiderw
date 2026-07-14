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
	// Nested under their owning network, with the object-path tail the MAC hex
	// (colons stripped) -- mirroring real iwd. The first is KnownNet's connected
	// AP; the other two belong to OpenNet.
	{path: "/net/connman/iwd/0/3/4b6e6f776e4e6574_psk/deadbeefcafe", address: "de:ad:be:ef:ca:fe"},
	{path: "/net/connman/iwd/0/3/4f70656e4e6574_open/112233445566", address: "11:22:33:44:55:66"},
	{path: "/net/connman/iwd/0/3/4f70656e4e6574_open/778899aabbcc", address: "77:88:99:aa:bb:cc"},
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
		// An absent optional property is reported the way iwd words it — the
		// client's "is this just absent?" matcher keys off this text, and a
		// different wording turns a tolerated absence into a hard error.
		return dbus.Variant{}, dbus.MakeFailedError(fmt.Errorf("getting property value failed"))
	}
	return v, nil
}
