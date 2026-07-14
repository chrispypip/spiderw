package mock

import (
	"fmt"

	"github.com/godbus/dbus/v5"

	"github.com/chrispypip/spiderw/internal/iwdbus"
)

// networkFixtures defines the mock networks exported under the device.
//
// They cover the three Connect outcomes iwd distinguishes:
//   - an open network connects with no agent,
//   - a known (provisioned) secured network connects with no agent,
//   - an unknown secured network is rejected with net.connman.iwd.NoAgent.
//
// Paths are listed in sorted order to match the daemon's path-sorted
// enumeration. The open network's ExtendedServiceSet lists both mock BSSes,
// demonstrating multi-BSS (e.g. 2.4 GHz + 5 GHz) membership.
var networkFixtures = []struct {
	path         dbus.ObjectPath
	name         string
	secType      string
	connected    bool
	knownNetwork dbus.ObjectPath // empty means the network is not known
	ess          []dbus.ObjectPath
}{
	{
		// iwd encodes a network path as <hex-SSID>_<security>; 4b6e6f776e4e6574 is
		// "KnownNet". BSSes are nested under their network with the MAC as the tail.
		path:    "/net/connman/iwd/0/3/4b6e6f776e4e6574_psk",
		name:    "KnownNet",
		secType: "psk",
		// Provisioned: iwd has stored credentials, so Connect needs no agent.
		knownNetwork: "/net/connman/iwd/4b6e6f776e4e6574_psk",
		ess: []dbus.ObjectPath{
			"/net/connman/iwd/0/3/4b6e6f776e4e6574_psk/deadbeefcafe",
		},
	},
	{
		path:    "/net/connman/iwd/0/3/4f70656e4e6574_open",
		name:    "OpenNet",
		secType: "open",
		ess: []dbus.ObjectPath{
			"/net/connman/iwd/0/3/4f70656e4e6574_open/112233445566",
			"/net/connman/iwd/0/3/4f70656e4e6574_open/778899aabbcc",
		},
	},
	{
		path:    "/net/connman/iwd/0/3/536563757265644e6574_psk",
		name:    "SecuredNet",
		secType: "psk",
		// Not known and secured: Connect fails with NoAgent until an agent exists.
		ess: nil,
	},
}

var exportedNetworks []*Network

// Network represents the mock iwd Network interface exported on D-Bus.
type Network struct {
	// Path is the D-Bus object path this network is exported at.
	Path dbus.ObjectPath

	// Name is the mock Name (SSID) property.
	Name string

	// Connected is the mock Connected property.
	Connected bool

	// Device is the mock Device property (owning device object path).
	Device dbus.ObjectPath

	// Type is the mock Type (security) property.
	Type string

	// KnownNetwork is the mock KnownNetwork property. An empty path means the
	// property is omitted (the network is not known).
	KnownNetwork dbus.ObjectPath

	// ExtendedServiceSet is the mock ExtendedServiceSet property (BSS paths).
	ExtendedServiceSet []dbus.ObjectPath
}

// ExportNetwork exports the mock network objects on the D-Bus connection.
//
// When --omit-network is set, no network objects are exported (and the
// ObjectManager will report no networks), which exercises empty enumeration.
func ExportNetwork(conn *dbus.Conn) error {
	if *omitNetworkFlag {
		return nil
	}

	exportedNetworks = nil
	for _, fixture := range networkFixtures {
		n := &Network{
			Path:               fixture.path,
			Name:               fixture.name,
			Connected:          fixture.connected,
			Device:             devicePath,
			Type:               fixture.secType,
			KnownNetwork:       fixture.knownNetwork,
			ExtendedServiceSet: fixture.ess,
		}

		// Export methods (Connect) and the Properties interface.
		if err := conn.Export(n, n.Path, iwdbus.IwdNetworkIface); err != nil {
			return err
		}
		if err := conn.Export(n, n.Path, "org.freedesktop.DBus.Properties"); err != nil {
			return err
		}
		if err := exportNetworkIntrospection(conn, n.Path); err != nil {
			return err
		}

		exportedNetworks = append(exportedNetworks, n)
	}

	return nil
}

func (n *Network) buildPropertyMap() map[string]dbus.Variant {
	props := map[string]dbus.Variant{
		"Name":               dbus.MakeVariant(n.Name),
		"Connected":          dbus.MakeVariant(n.Connected),
		"Device":             dbus.MakeVariant(n.Device),
		"Type":               dbus.MakeVariant(n.Type),
		"ExtendedServiceSet": dbus.MakeVariant(n.extendedServiceSet()),
	}
	// KnownNetwork is optional: omit it entirely when the network is not known.
	if n.KnownNetwork != "" {
		props["KnownNetwork"] = dbus.MakeVariant(n.KnownNetwork)
	}
	return props
}

// extendedServiceSet returns a non-nil slice so the property always marshals as
// an empty array rather than a nil value.
func (n *Network) extendedServiceSet() []dbus.ObjectPath {
	if n.ExtendedServiceSet == nil {
		return []dbus.ObjectPath{}
	}
	return n.ExtendedServiceSet
}

// Connect implements the mock Network.Connect method.
//
// Open and known (provisioned) networks connect with no agent, mirroring iwd. An
// unknown secured network needs a credentials agent: the mock calls the
// registered agent's RequestPassphrase and connects only if it returns the
// expected passphrase. With no agent registered it is rejected with
// net.connman.iwd.NoAgent; a wrong or declined passphrase yields
// net.connman.iwd.Failed.
func (n *Network) Connect() *dbus.Error {
	if n.Type != "open" && n.KnownNetwork == "" {
		passphrase, ok, err := agents.requestPassphrase(n.Path)
		if !ok {
			return dbus.NewError(iwdbus.IwdErrorNoAgent, []interface{}{"No agent registered"})
		}
		if err != nil {
			// The agent declined (Canceled) or the callback failed; iwd surfaces
			// this as an association failure.
			return dbus.NewError(iwdbus.IwdErrorFailed, []interface{}{err.Error()})
		}
		if passphrase != securedNetworkPassphrase {
			return invalidPassphraseError()
		}
	}

	n.Connected = true
	// iwd writes a profile the first time a secured network is connected to, so a
	// KnownNetwork object appears and the Network gains a link to it.
	if n.Type != "open" {
		provisionKnownNetwork(n)
	}

	emitPropertiesChanged(n.Path, iwdbus.IwdNetworkIface, map[string]dbus.Variant{"Connected": dbus.MakeVariant(true)}, []string{})
	return nil
}

// GetAll returns all mock network properties for the requested interface.
func (n *Network) GetAll(iface string) (map[string]dbus.Variant, *dbus.Error) {
	if iface != iwdbus.IwdNetworkIface {
		return nil, dbus.MakeFailedError(fmt.Errorf("unknown property %q", iface))
	}
	return n.buildPropertyMap(), nil
}

// Get returns a single mock network property for the requested interface.
func (n *Network) Get(iface, p string) (dbus.Variant, *dbus.Error) {
	if iface != iwdbus.IwdNetworkIface {
		return dbus.Variant{}, dbus.MakeFailedError(fmt.Errorf("unknown property %q", iface))
	}

	props := n.buildPropertyMap()
	v, ok := props[p]
	if !ok {
		// An absent optional property is reported the way iwd words it - the
		// client's "is this just absent?" matcher keys off this text, and a
		// different wording turns a tolerated absence into a hard error.
		return dbus.Variant{}, dbus.MakeFailedError(fmt.Errorf("getting property value failed"))
	}
	return v, nil
}
