package mock

import (
	"fmt"

	"github.com/godbus/dbus/v5"

	"github.com/chrispypip/spiderw/internal/iwdbus"
)

// knownNetworkFixtures defines the mock known networks.
//
// The first is exported at the path the mock OpenNet/KnownNet network references
// via its KnownNetwork property, so that linkage resolves end to end. The second
// is a hotspot that has never been connected to (no LastConnectedTime) and has
// auto-connect disabled, exercising the optional/hotspot paths.
var knownNetworkFixtures = []struct {
	path              dbus.ObjectPath
	name              string
	secType           string
	hidden            bool
	lastConnectedTime string // empty means the property is omitted
	autoConnect       bool
}{
	{
		// iwd exports a known network at /net/connman/iwd/<hex-SSID>_<security>.
		path:              "/net/connman/iwd/4b6e6f776e4e6574_psk",
		name:              "KnownNet",
		secType:           "psk",
		lastConnectedTime: "2024-01-02T03:04:05Z",
		autoConnect:       true,
	},
	{
		path:        "/net/connman/iwd/4775657374486f7473706f74_hotspot",
		name:        "GuestHotspot",
		secType:     "hotspot",
		autoConnect: false,
	},
}

var exportedKnownNetworks []*KnownNetwork

// KnownNetwork represents the mock iwd KnownNetwork interface exported on D-Bus.
type KnownNetwork struct {
	// Path is the D-Bus object path this known network is exported at.
	Path dbus.ObjectPath

	// Name is the mock Name property.
	Name string

	// Type is the mock Type (network type) property.
	Type string

	// Hidden is the mock Hidden property.
	Hidden bool

	// LastConnectedTime is the mock LastConnectedTime property. An empty string
	// means the property is omitted (the network was never connected to).
	LastConnectedTime string

	// AutoConnect is the mock AutoConnect property.
	AutoConnect bool
}

// ExportKnownNetwork exports the mock known-network objects on the D-Bus
// connection.
//
// When --omit-knownnetwork is set, no known-network objects are exported (and the
// ObjectManager will report none), which exercises empty enumeration.
func ExportKnownNetwork(conn *dbus.Conn) error {
	if *omitKnownNetworkFlag {
		return nil
	}

	exportedKnownNetworks = nil
	for _, fixture := range knownNetworkFixtures {
		k := &KnownNetwork{
			Path:              fixture.path,
			Name:              fixture.name,
			Type:              fixture.secType,
			Hidden:            fixture.hidden,
			LastConnectedTime: fixture.lastConnectedTime,
			AutoConnect:       fixture.autoConnect,
		}

		if err := conn.Export(k, k.Path, iwdbus.IwdKnownNetworkIface); err != nil {
			return err
		}
		if err := conn.Export(k, k.Path, "org.freedesktop.DBus.Properties"); err != nil {
			return err
		}
		if err := exportKnownNetworkIntrospection(conn, k.Path); err != nil {
			return err
		}

		exportedKnownNetworks = append(exportedKnownNetworks, k)
	}

	return nil
}

func (k *KnownNetwork) buildPropertyMap() map[string]dbus.Variant {
	props := map[string]dbus.Variant{
		"Name":        dbus.MakeVariant(k.Name),
		"Type":        dbus.MakeVariant(k.Type),
		"Hidden":      dbus.MakeVariant(k.Hidden),
		"AutoConnect": dbus.MakeVariant(k.AutoConnect),
	}
	// LastConnectedTime is optional: omit it when the network was never connected.
	if k.LastConnectedTime != "" {
		props["LastConnectedTime"] = dbus.MakeVariant(k.LastConnectedTime)
	}
	return props
}

// Forget implements the mock KnownNetwork.Forget method: the known-network record
// goes away, so every Network that referenced it loses its KnownNetwork property.
//
// iwd signals that loss by *invalidating* the property, not by sending the null
// path "/" (confirmed on hardware: monitoring a network's known-network printed
// nothing on forget). The mock previously did nothing at all here, which is why a
// subscription that only read Changed looked correct.
func (k *KnownNetwork) Forget() *dbus.Error {
	for _, n := range exportedNetworks {
		if n.KnownNetwork != k.Path {
			continue
		}
		n.KnownNetwork = ""
		emitPropertiesChanged(n.Path, iwdbus.IwdNetworkIface,
			map[string]dbus.Variant{}, []string{"KnownNetwork"})
	}
	return nil
}

// GetAll returns all mock known-network properties for the requested interface.
func (k *KnownNetwork) GetAll(iface string) (map[string]dbus.Variant, *dbus.Error) {
	if iface != iwdbus.IwdKnownNetworkIface {
		return nil, dbus.MakeFailedError(fmt.Errorf("unknown property %q", iface))
	}
	return k.buildPropertyMap(), nil
}

// Get returns a single mock known-network property for the requested interface.
func (k *KnownNetwork) Get(iface, p string) (dbus.Variant, *dbus.Error) {
	if iface != iwdbus.IwdKnownNetworkIface {
		return dbus.Variant{}, dbus.MakeFailedError(fmt.Errorf("unknown property %q", iface))
	}

	props := k.buildPropertyMap()
	v, ok := props[p]
	if !ok {
		return dbus.Variant{}, dbus.MakeFailedError(fmt.Errorf("unknown property %q", p))
	}
	return v, nil
}

// Set stores the AutoConnect property and emits a matching change signal. Other
// properties are read-only.
func (k *KnownNetwork) Set(iface, p string, v dbus.Variant) *dbus.Error {
	if iface != iwdbus.IwdKnownNetworkIface {
		return nil
	}

	if p != "AutoConnect" {
		return dbus.MakeFailedError(fmt.Errorf("cannot set property %q", p))
	}

	b, ok := v.Value().(bool)
	if !ok {
		return dbus.MakeFailedError(fmt.Errorf("property AutoConnect must be a bool, got %T", v))
	}
	k.AutoConnect = b
	emitPropertiesChanged(k.Path, iwdbus.IwdKnownNetworkIface, map[string]dbus.Variant{"AutoConnect": dbus.MakeVariant(b)}, []string{})
	return nil
}
