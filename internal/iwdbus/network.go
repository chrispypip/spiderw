package iwdbus

import (
	"context"
	"fmt"

	"github.com/godbus/dbus/v5"

	"github.com/chrispypip/spiderw/internal/iwdvalue"
)

// IwdNetworkIface is the fully qualified D-Bus interface name for iwd networks.
const IwdNetworkIface = IwdService + ".Network"

// NetworkType identifies a raw iwd network type.
type NetworkType = iwdvalue.NetworkType

// NetworkType constants identify raw iwd network types.
// NetworkTypeUnknown is reserved for invalid or unrecognized values.
const (
	// NetworkTypeUnknown represents an invalid or unrecognized network type.
	NetworkTypeUnknown = iwdvalue.NetworkTypeUnknown

	// NetworkTypeOpen is an open (unsecured) network.
	NetworkTypeOpen = iwdvalue.NetworkTypeOpen

	// NetworkTypeWEP is a WEP network.
	NetworkTypeWEP = iwdvalue.NetworkTypeWEP

	// NetworkTypePSK is a pre-shared-key (WPA-Personal) network.
	NetworkTypePSK = iwdvalue.NetworkTypePSK

	// NetworkType8021x is an 802.1x (EAP) network.
	NetworkType8021x = iwdvalue.NetworkType8021x

	// NetworkTypeHotspot is a hotspot network (reported only for a KnownNetwork).
	NetworkTypeHotspot = iwdvalue.NetworkTypeHotspot
)

// NetworkPropertiesChanged describes raw D-Bus network property-change data.
type NetworkPropertiesChanged struct {
	// Changed contains raw D-Bus variants keyed by property name.
	Changed map[string]dbus.Variant

	// Invalidated contains property names whose values should be re-read if needed.
	Invalidated []string
}

// Network wraps an iwd Network object using runtime introspection.
type Network struct {
	call    caller
	signals signalSource
}

// NewNetwork creates a Network for the given iwd object path.
func NewNetwork(ctx context.Context, conn *dbus.Conn, path dbus.ObjectPath) (*Network, error) {
	intro, err := NewIntrospectedObject(ctx, conn, IwdService, path)
	if err != nil {
		return nil, WrapIntrospection(string(path), err)
	}
	if !intro.HasInterface(IwdNetworkIface) {
		_ = intro.Close()
		return nil, fmt.Errorf("object %s does not implement %s", path, IwdNetworkIface)
	}
	return &Network{
		call:    caller(intro),
		signals: signalSource(intro),
	}, nil
}

// GetName reads the Name (SSID) property.
func (n *Network) GetName(ctx context.Context) (string, error) {
	if err := n.ensureInitialized(); err != nil {
		return "", WrapConnection("Network.ensureInitialized", err)
	}

	value, err := n.call.GetProperty(ctx, IwdNetworkIface, "Name")
	if err != nil {
		return "", WrapProperty(IwdNetworkIface, "Name", err)
	}

	s, ok := value.(string)
	if !ok {
		return "", WrapVariant("Name", fmt.Errorf("expected string, got %T", value))
	}
	// Empty/whitespace Name is a semantic concern owned by the core layer; the
	// D-Bus layer returns the raw value.
	return s, nil
}

// GetConnected reads the Connected property.
func (n *Network) GetConnected(ctx context.Context) (bool, error) {
	if err := n.ensureInitialized(); err != nil {
		return false, WrapConnection("Network.ensureInitialized", err)
	}

	value, err := n.call.GetProperty(ctx, IwdNetworkIface, "Connected")
	if err != nil {
		return false, WrapProperty(IwdNetworkIface, "Connected", err)
	}

	b, ok := value.(bool)
	if !ok {
		return false, WrapVariant("Connected", fmt.Errorf("expected bool, got %T", value))
	}
	return b, nil
}

// GetDevice reads the Device property, the object path of the station/device
// the network belongs to.
func (n *Network) GetDevice(ctx context.Context) (dbus.ObjectPath, error) {
	if err := n.ensureInitialized(); err != nil {
		return "", WrapConnection("Network.ensureInitialized", err)
	}

	value, err := n.call.GetProperty(ctx, IwdNetworkIface, "Device")
	if err != nil {
		return "", WrapProperty(IwdNetworkIface, "Device", err)
	}
	return parseNetworkObjectPath("Device", value)
}

// GetType reads and parses the Type (network type) property.
func (n *Network) GetType(ctx context.Context) (NetworkType, error) {
	if err := n.ensureInitialized(); err != nil {
		return NetworkTypeUnknown, WrapConnection("Network.ensureInitialized", err)
	}

	value, err := n.call.GetProperty(ctx, IwdNetworkIface, "Type")
	if err != nil {
		return NetworkTypeUnknown, WrapProperty(IwdNetworkIface, "Type", err)
	}
	return parseNetworkType(value)
}

// GetKnownNetwork reads the optional KnownNetwork property, the object path of
// the KnownNetwork object corresponding to this network. It returns nil when the
// network has no known-network record (the property is omitted).
func (n *Network) GetKnownNetwork(ctx context.Context) (*string, error) {
	if err := n.ensureInitialized(); err != nil {
		return nil, WrapConnection("Network.ensureInitialized", err)
	}

	value, err := n.call.GetProperty(ctx, IwdNetworkIface, "KnownNetwork")
	if err != nil {
		if isUnknownPropertyError(err) {
			return nil, nil
		}
		return nil, WrapProperty(IwdNetworkIface, "KnownNetwork", err)
	}
	return parseOptionalObjectPath(value)
}

// GetExtendedServiceSet reads the ExtendedServiceSet property, the object paths
// of the BasicServiceSet (BSS) objects that make up this network.
func (n *Network) GetExtendedServiceSet(ctx context.Context) ([]string, error) {
	if err := n.ensureInitialized(); err != nil {
		return nil, WrapConnection("Network.ensureInitialized", err)
	}

	value, err := n.call.GetProperty(ctx, IwdNetworkIface, "ExtendedServiceSet")
	if err != nil {
		return nil, WrapProperty(IwdNetworkIface, "ExtendedServiceSet", err)
	}
	return parseObjectPathList(value)
}

// Connect calls Network.Connect, requesting that the owning device connect to
// this network.
//
// Connecting to an open network, or a network that iwd already knows, needs no
// credentials agent. Connecting to a secured network that is not already known
// fails with ErrNoAgent until an agent is registered to supply credentials.
func (n *Network) Connect(ctx context.Context) error {
	if err := n.ensureInitialized(); err != nil {
		return WrapConnection("Network.ensureInitialized", err)
	}

	if _, err := n.call.Call(ctx, IwdNetworkIface, "Connect"); err != nil {
		return wrapIwdMethod(IwdNetworkIface, "Connect", err)
	}
	return nil
}

// NetworkProperties holds every network property read in a single
// Properties.GetAll call. KnownNetwork is optional: a nil pointer means the
// network has no known-network record (the property is absent from GetAll).
type NetworkProperties struct {
	Name               string
	Connected          bool
	Device             dbus.ObjectPath
	Type               NetworkType
	KnownNetwork       *string
	ExtendedServiceSet []string
}

// GetProperties reads every network property in a single Properties.GetAll call
// instead of one Get per property.
//
// Name, Connected, Device, Type, and ExtendedServiceSet are required; a missing
// one is an error. KnownNetwork is optional and left nil when absent.
func (n *Network) GetProperties(ctx context.Context) (*NetworkProperties, error) {
	if err := n.ensureInitialized(); err != nil {
		return nil, WrapConnection("Network.ensureInitialized", err)
	}

	raw, err := n.call.GetAll(ctx, IwdNetworkIface)
	if err != nil {
		return nil, WrapProperty(IwdNetworkIface, "GetAll", err)
	}

	props := &NetworkProperties{}

	nameV, ok := raw["Name"]
	if !ok {
		return nil, WrapProperty(IwdNetworkIface, "Name", fmt.Errorf("missing required property"))
	}
	name, ok := nameV.Value().(string)
	if !ok {
		return nil, WrapVariant("Name", fmt.Errorf("expected string, got %T", nameV.Value()))
	}
	props.Name = name

	connectedV, ok := raw["Connected"]
	if !ok {
		return nil, WrapProperty(IwdNetworkIface, "Connected", fmt.Errorf("missing required property"))
	}
	connected, ok := connectedV.Value().(bool)
	if !ok {
		return nil, WrapVariant("Connected", fmt.Errorf("expected bool, got %T", connectedV.Value()))
	}
	props.Connected = connected

	deviceV, ok := raw["Device"]
	if !ok {
		return nil, WrapProperty(IwdNetworkIface, "Device", fmt.Errorf("missing required property"))
	}
	device, err := parseNetworkObjectPath("Device", deviceV.Value())
	if err != nil {
		return nil, err
	}
	props.Device = device

	typeV, ok := raw["Type"]
	if !ok {
		return nil, WrapProperty(IwdNetworkIface, "Type", fmt.Errorf("missing required property"))
	}
	secType, err := parseNetworkType(typeV.Value())
	if err != nil {
		return nil, err
	}
	props.Type = secType

	essV, ok := raw["ExtendedServiceSet"]
	if !ok {
		return nil, WrapProperty(IwdNetworkIface, "ExtendedServiceSet", fmt.Errorf("missing required property"))
	}
	ess, err := parseObjectPathList(essV.Value())
	if err != nil {
		return nil, err
	}
	props.ExtendedServiceSet = ess

	if knownV, ok := raw["KnownNetwork"]; ok {
		known, err := parseOptionalObjectPath(knownV.Value())
		if err != nil {
			return nil, err
		}
		props.KnownNetwork = known
	}

	return props, nil
}

// SubscribePropertiesChanged registers fn for raw network property-change signals.
func (n *Network) SubscribePropertiesChanged(ctx context.Context, fn func(NetworkPropertiesChanged)) (UnsubscribeFunc, error) {
	if fn == nil {
		return nil, fmt.Errorf("SubscribePropertiesChanged: fn cannot be nil")
	}

	return n.signals.RegisterSignalHandlerWithUnsubscribe("org.freedesktop.DBus.Properties", "PropertiesChanged", func(sig *dbus.Signal) {
		if sig == nil || len(sig.Body) < 3 {
			return
		}

		iface, ok := sig.Body[0].(string)
		if !ok || iface != IwdNetworkIface {
			return
		}

		changed, ok := sig.Body[1].(map[string]dbus.Variant)
		if !ok {
			return
		}

		invalid, ok := sig.Body[2].([]string)
		if !ok {
			invalid = nil
		}

		fn(NetworkPropertiesChanged{
			Changed:     changed,
			Invalidated: invalid,
		})
	})
}

// SubscribeConnectedChanged registers fn for raw connected-state changes.
func (n *Network) SubscribeConnectedChanged(ctx context.Context, fn func(bool)) (UnsubscribeFunc, error) {
	if fn == nil {
		return nil, fmt.Errorf("SubscribeConnectedChanged: fn cannot be nil")
	}

	return n.SubscribePropertiesChanged(ctx, func(ev NetworkPropertiesChanged) {
		variant, ok := ev.Changed["Connected"]
		if !ok {
			return
		}

		b, ok := variant.Value().(bool)
		if ok {
			fn(b)
		}
	})
}

// SubscribeKnownNetworkChanged registers fn for raw changes to the network's
// KnownNetwork association. fn receives the KnownNetwork object path, or nil when
// the network is not known (iwd reports that as the null path "/").
//
// This is how a network being saved or forgotten is observed: provisioning a
// network gives it a KnownNetwork, and forgetting it takes it away.
func (n *Network) SubscribeKnownNetworkChanged(ctx context.Context, fn func(*string)) (UnsubscribeFunc, error) {
	if fn == nil {
		return nil, fmt.Errorf("SubscribeKnownNetworkChanged: fn cannot be nil")
	}

	return n.SubscribePropertiesChanged(ctx, func(ev NetworkPropertiesChanged) {
		variant, ok := ev.Changed["KnownNetwork"]
		if !ok {
			// Forgetting a network clears the record by invalidating the property,
			// not by sending "/", so invalidation is how a forget arrives.
			if propertyCleared(ev.Invalidated, "KnownNetwork") {
				fn(nil)
			}
			return
		}

		path, err := parseOptionalObjectPath(variant.Value())
		if err != nil {
			return
		}
		fn(path)
	})
}

// SubscribeExtendedServiceSetChanged registers fn for raw changes to the
// network's BSS list. fn receives the BSS object paths, which change as access
// points for the network come and go across scans.
func (n *Network) SubscribeExtendedServiceSetChanged(ctx context.Context, fn func([]string)) (UnsubscribeFunc, error) {
	if fn == nil {
		return nil, fmt.Errorf("SubscribeExtendedServiceSetChanged: fn cannot be nil")
	}

	return n.SubscribePropertiesChanged(ctx, func(ev NetworkPropertiesChanged) {
		variant, ok := ev.Changed["ExtendedServiceSet"]
		if !ok {
			return
		}

		paths, err := parseObjectPathList(variant.Value())
		if err != nil {
			return
		}
		fn(paths)
	})
}

// Firehose emits high-frequency network signals for stress and integration tests.
func (n *Network) Firehose(ctx context.Context, fn func(FirehoseSignal)) error {
	if fn == nil {
		return fmt.Errorf("Firehose: fn cannot be nil")
	}

	// Wildcard interface ("*") + wildcard member ("*") gives all signals.
	return n.signals.RegisterSignalHandler("*", "*", func(sig *dbus.Signal) {
		if sig == nil {
			return
		}

		iface, member := splitSignalName(sig.Name)
		fn(FirehoseSignal{
			ObjectPath: sig.Path,
			Interface:  iface,
			Member:     member,
			Body:       sig.Body,
			Raw:        sig,
		})
	})
}

// ensureInitialized verifies that n has been initialized by NewNetwork.
func (n *Network) ensureInitialized() error {
	if n.call == nil {
		return ErrNetworkUninitialized
	}
	return nil
}

// parseNetworkType normalizes the D-Bus Type value into a NetworkType.
func parseNetworkType(v interface{}) (NetworkType, error) {
	s, ok := v.(string)
	if !ok {
		return NetworkTypeUnknown, WrapVariant("Type", fmt.Errorf("expected string, got %T", v))
	}
	secType, ok := iwdvalue.ParseNetworkType(s)
	if !ok {
		return NetworkTypeUnknown, WrapVariant("Type", fmt.Errorf("invalid network type %q", s))
	}
	return secType, nil
}

// parseNetworkObjectPath normalizes a required D-Bus object-path value into a
// dbus.ObjectPath, accepting both the typed dbus.ObjectPath and string forms.
func parseNetworkObjectPath(field string, v interface{}) (dbus.ObjectPath, error) {
	switch p := v.(type) {
	case dbus.ObjectPath:
		if !p.IsValid() {
			return "", WrapVariant(field, fmt.Errorf("invalid object path %q", p))
		}
		return p, nil
	case string:
		path := dbus.ObjectPath(p)
		if !path.IsValid() {
			return "", WrapVariant(field, fmt.Errorf("invalid object path %q", p))
		}
		return path, nil
	default:
		return "", WrapVariant(field, fmt.Errorf("expected object path, got %T", v))
	}
}

// parseOptionalObjectPath normalizes an optional D-Bus object-path value into a
// *string. A nil value, empty path, or the root path "/" (iwd's "no object"
// sentinel) all yield nil.
func parseOptionalObjectPath(v interface{}) (*string, error) {
	var path string
	switch p := v.(type) {
	case nil:
		return nil, nil
	case dbus.ObjectPath:
		path = string(p)
	case string:
		path = p
	case dbus.Variant:
		return parseOptionalObjectPath(p.Value())
	default:
		return nil, WrapVariant("KnownNetwork", fmt.Errorf("expected object path, got %T", v))
	}

	if path == "" || path == "/" {
		return nil, nil
	}
	if !dbus.ObjectPath(path).IsValid() {
		return nil, WrapVariant("KnownNetwork", fmt.Errorf("invalid object path %q", path))
	}
	return &path, nil
}

// parseObjectPathList normalizes a D-Bus array-of-object-path value into a
// []string, accepting both []dbus.ObjectPath and []interface{} forms.
func parseObjectPathList(v interface{}) ([]string, error) {
	switch raw := v.(type) {
	case []dbus.ObjectPath:
		out := make([]string, 0, len(raw))
		for _, p := range raw {
			if !p.IsValid() {
				return nil, WrapVariant("ExtendedServiceSet", fmt.Errorf("invalid object path %q", p))
			}
			out = append(out, string(p))
		}
		return out, nil
	case []interface{}:
		out := make([]string, 0, len(raw))
		for _, elem := range raw {
			path, err := parseNetworkObjectPath("ExtendedServiceSet", elem)
			if err != nil {
				return nil, err
			}
			out = append(out, string(path))
		}
		return out, nil
	default:
		return nil, WrapVariant("ExtendedServiceSet", fmt.Errorf("expected object path array, got %T", v))
	}
}
