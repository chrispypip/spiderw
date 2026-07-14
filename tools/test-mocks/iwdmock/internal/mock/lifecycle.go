package mock

import (
	"sync"

	"github.com/godbus/dbus/v5"

	"github.com/chrispypip/spiderw/internal/iwdbus"
)

// iwd's object tree is not static. Forgetting a network destroys its KnownNetwork
// object; connecting to a secured network provisions one; switching a device
// between station and AP mode swaps which of those interfaces the device object
// carries. The mock used to export everything once at startup and never move it
// again, so none of that was reachable - the mode switch in particular was faked
// by shipping two fixed devices, one station and one AP, which is not a shape a
// real host ever has.
//
// This file gives the mock a real lifecycle: a registry the ObjectManager reads
// live, plus the InterfacesAdded/InterfacesRemoved signals iwd emits when objects
// come and go.

// registryMu guards the exported* registries, which the ObjectManager now reads on
// every call rather than snapshotting at export time.
var registryMu sync.RWMutex

// mockConn is the connection the mock exported its objects on. Adding or removing
// an object after startup needs it.
var mockConn *dbus.Conn

// setMockConn records the connection used for dynamic exports.
func setMockConn(conn *dbus.Conn) { mockConn = conn }

// emitInterfacesAdded announces a new object (or new interfaces on an existing
// object) the way iwd's ObjectManager does.
func emitInterfacesAdded(path dbus.ObjectPath, ifaces map[string]map[string]dbus.Variant) {
	if mockConn == nil {
		return
	}
	_ = mockConn.Emit(objectManagerPath, iwdbus.DBusObjectManagerIface+".InterfacesAdded", path, ifaces)
}

// emitInterfacesRemoved announces that an object lost interfaces (or went away
// entirely, when every interface it had is listed).
func emitInterfacesRemoved(path dbus.ObjectPath, ifaces []string) {
	if mockConn == nil {
		return
	}
	_ = mockConn.Emit(objectManagerPath, iwdbus.DBusObjectManagerIface+".InterfacesRemoved", path, ifaces)
}

// unexportObject tears down every export at a path, so a later method call to it
// fails the way a call to a destroyed iwd object does.
func unexportObject(path dbus.ObjectPath, ifaces ...string) {
	if mockConn == nil {
		return
	}
	for _, iface := range ifaces {
		_ = mockConn.Export(nil, path, iface)
	}
	_ = mockConn.Export(nil, path, "org.freedesktop.DBus.Properties")
	_ = mockConn.Export(nil, path, "org.freedesktop.DBus.Introspectable")
}

// removeKnownNetwork destroys a KnownNetwork object: it drops out of the registry
// (so it stops appearing in enumeration), its exports are torn down, and iwd's
// InterfacesRemoved is emitted. Every Network that referenced it loses the link.
func removeKnownNetwork(k *KnownNetwork) {
	registryMu.Lock()
	kept := make([]*KnownNetwork, 0, len(exportedKnownNetworks))
	for _, candidate := range exportedKnownNetworks {
		if candidate != k {
			kept = append(kept, candidate)
		}
	}
	exportedKnownNetworks = kept

	// Networks pointing at it lose the association. iwd reports that loss by
	// invalidating the property, not by sending the null path.
	var unlinked []*Network
	for _, n := range exportedNetworks {
		if n.KnownNetwork == k.Path {
			n.KnownNetwork = ""
			unlinked = append(unlinked, n)
		}
	}
	registryMu.Unlock()

	for _, n := range unlinked {
		emitPropertiesChanged(n.Path, iwdbus.IwdNetworkIface,
			map[string]dbus.Variant{}, []string{"KnownNetwork"})
	}

	unexportObject(k.Path, iwdbus.IwdKnownNetworkIface)
	emitInterfacesRemoved(k.Path, []string{iwdbus.IwdKnownNetworkIface})
}

// provisionKnownNetwork creates the KnownNetwork object iwd writes when a secured
// network is connected to for the first time, links the Network to it, and
// announces it. It is a no-op if the network is already known.
func provisionKnownNetwork(n *Network) {
	registryMu.RLock()
	already := n.KnownNetwork != ""
	registryMu.RUnlock()
	if already {
		return
	}

	k := &KnownNetwork{
		Path:              knownNetworkPathFor(n),
		Name:              n.Name,
		Type:              n.Type,
		LastConnectedTime: provisionedLastConnectedTime,
		AutoConnect:       true,
	}

	if mockConn != nil {
		if err := mockConn.Export(k, k.Path, iwdbus.IwdKnownNetworkIface); err != nil {
			return
		}
		if err := mockConn.Export(k, k.Path, "org.freedesktop.DBus.Properties"); err != nil {
			return
		}
		if err := exportKnownNetworkIntrospection(mockConn, k.Path); err != nil {
			return
		}
	}

	registryMu.Lock()
	exportedKnownNetworks = append(exportedKnownNetworks, k)
	n.KnownNetwork = k.Path
	registryMu.Unlock()

	emitInterfacesAdded(k.Path, map[string]map[string]dbus.Variant{
		iwdbus.IwdKnownNetworkIface: k.buildPropertyMap(),
	})
	emitPropertiesChanged(n.Path, iwdbus.IwdNetworkIface,
		map[string]dbus.Variant{"KnownNetwork": dbus.MakeVariant(k.Path)}, []string{})
}

// provisionedLastConnectedTime is the timestamp a freshly provisioned known
// network reports.
const provisionedLastConnectedTime = "2026-01-01T00:00:00Z"

// knownNetworkPathFor mirrors iwd's known-network path layout:
// /net/connman/iwd/<hex-SSID>_<security>.
func knownNetworkPathFor(n *Network) dbus.ObjectPath {
	return dbus.ObjectPath("/net/connman/iwd/" + hexSSID(n.Name) + "_" + n.Type)
}

// hexSSID hex-encodes an SSID the way iwd encodes it into an object path.
func hexSSID(ssid string) string {
	const digits = "0123456789abcdef"
	out := make([]byte, 0, len(ssid)*2)
	for _, b := range []byte(ssid) {
		out = append(out, digits[b>>4], digits[b&0x0f])
	}
	return string(out)
}

// switchDeviceMode models what iwd does when a device's Mode is set: the
// mode-specific interface on the device object is swapped. A device moved to "ap"
// loses Station (and WSC, which lives on the station) and gains AccessPoint;
// moving back to "station" reverses it.
//
// This is the transition a user actually makes (`device wlan0 mode ap`), and the
// mock could not model it: it shipped two fixed devices, one station-only and one
// AP-only, so the interfaces never moved. That also meant a Station handle held
// across a mode switch - which must start failing, because the interface is gone -
// was never exercised.
func switchDeviceMode(d *Device, mode string) {
	registryMu.Lock()
	wasStation, wasAP := d.HasStation, d.HasAccessPoint

	switch mode {
	case "ap":
		d.HasStation = false
		d.HasAccessPoint = accessPointExported()
	case "station":
		d.HasStation = stationExported()
		d.HasAccessPoint = false
	default:
		// Any other mode (e.g. ad-hoc) carries neither interface.
		d.HasStation = false
		d.HasAccessPoint = false
	}
	nowStation, nowAP := d.HasStation, d.HasAccessPoint
	withWSC := nowStation && !*omitWSCFlag
	registryMu.Unlock()

	if wasStation == nowStation && wasAP == nowAP {
		return
	}

	// Tear down the interfaces the device no longer has, so a call to one fails
	// the way it would against iwd.
	var removed []string
	if wasStation && !nowStation {
		removed = append(removed, iwdbus.IwdStationIface)
		if !*omitWSCFlag {
			removed = append(removed, iwdbus.IwdSimpleConfigurationIface)
		}
	}
	if wasAP && !nowAP {
		removed = append(removed, iwdbus.IwdAccessPointIface)
	}

	if mockConn != nil {
		for _, iface := range removed {
			_ = mockConn.Export(nil, d.Path, iface)
		}
		if !wasStation && nowStation {
			_ = mockConn.Export(d, d.Path, iwdbus.IwdStationIface)
			if withWSC {
				_ = mockConn.Export(d, d.Path, iwdbus.IwdSimpleConfigurationIface)
			}
		}
		if !wasAP && nowAP {
			_ = mockConn.Export(&AccessPoint{device: d}, d.Path, iwdbus.IwdAccessPointIface)
		}
		// Introspection must agree with the exports, or the client's HasInterface
		// check disagrees with what the object actually answers.
		_ = exportDeviceIntrospection(mockConn, d.Path, nowStation, withWSC, nowAP)
	}

	if len(removed) > 0 {
		emitInterfacesRemoved(d.Path, removed)
	}

	added := map[string]map[string]dbus.Variant{}
	if !wasStation && nowStation {
		added[iwdbus.IwdStationIface] = d.buildStationPropertyMap()
	}
	if !wasAP && nowAP {
		added[iwdbus.IwdAccessPointIface] = d.buildAccessPointPropertyMap()
	}
	if len(added) > 0 {
		emitInterfacesAdded(d.Path, added)
	}
}
