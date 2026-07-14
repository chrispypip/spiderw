package iwdbus

import (
	"context"
	"fmt"

	"github.com/godbus/dbus/v5"
)

// IwdKnownNetworkIface is the fully qualified D-Bus interface name for iwd known
// networks.
const IwdKnownNetworkIface = IwdService + ".KnownNetwork"

// KnownNetworkPropertiesChanged describes raw D-Bus known-network
// property-change data.
type KnownNetworkPropertiesChanged struct {
	// Changed contains raw D-Bus variants keyed by property name.
	Changed map[string]dbus.Variant

	// Invalidated contains property names whose values should be re-read if needed.
	Invalidated []string
}

// KnownNetwork wraps an iwd KnownNetwork object using runtime introspection.
type KnownNetwork struct {
	call    caller
	signals signalSource
}

// NewKnownNetwork creates a KnownNetwork for the given iwd object path.
func NewKnownNetwork(ctx context.Context, conn *dbus.Conn, path dbus.ObjectPath) (*KnownNetwork, error) {
	intro, err := NewIntrospectedObject(ctx, conn, IwdService, path)
	if err != nil {
		return nil, WrapIntrospection(string(path), err)
	}
	if !intro.HasInterface(IwdKnownNetworkIface) {
		_ = intro.Close()
		return nil, fmt.Errorf("object %s does not implement %s", path, IwdKnownNetworkIface)
	}
	return &KnownNetwork{
		call:    caller(intro),
		signals: signalSource(intro),
	}, nil
}

// GetName reads the Name property. This is usually the SSID, unless the network
// is a hotspot, in which case it comes from the provisioning file.
func (k *KnownNetwork) GetName(ctx context.Context) (string, error) {
	if err := k.ensureInitialized(); err != nil {
		return "", WrapConnection("KnownNetwork.ensureInitialized", err)
	}

	value, err := k.call.GetProperty(ctx, IwdKnownNetworkIface, "Name")
	if err != nil {
		return "", WrapProperty(IwdKnownNetworkIface, "Name", err)
	}

	s, ok := value.(string)
	if !ok {
		return "", WrapVariant("Name", fmt.Errorf("expected string, got %T", value))
	}
	// Empty/whitespace Name is a semantic concern owned by the core layer; the
	// D-Bus layer returns the raw value.
	return s, nil
}

// GetType reads and parses the Type property. For a known network this may be
// "open", "psk", "8021x", or "hotspot".
func (k *KnownNetwork) GetType(ctx context.Context) (NetworkType, error) {
	if err := k.ensureInitialized(); err != nil {
		return NetworkTypeUnknown, WrapConnection("KnownNetwork.ensureInitialized", err)
	}

	value, err := k.call.GetProperty(ctx, IwdKnownNetworkIface, "Type")
	if err != nil {
		return NetworkTypeUnknown, WrapProperty(IwdKnownNetworkIface, "Type", err)
	}
	return parseNetworkType(value)
}

// GetHidden reads the Hidden property.
func (k *KnownNetwork) GetHidden(ctx context.Context) (bool, error) {
	if err := k.ensureInitialized(); err != nil {
		return false, WrapConnection("KnownNetwork.ensureInitialized", err)
	}

	value, err := k.call.GetProperty(ctx, IwdKnownNetworkIface, "Hidden")
	if err != nil {
		return false, WrapProperty(IwdKnownNetworkIface, "Hidden", err)
	}

	b, ok := value.(bool)
	if !ok {
		return false, WrapVariant("Hidden", fmt.Errorf("expected bool, got %T", value))
	}
	return b, nil
}

// GetLastConnectedTime reads the optional LastConnectedTime property, an ISO
// 8601 timestamp. It returns nil when the network has never been successfully
// connected to (the property is omitted).
func (k *KnownNetwork) GetLastConnectedTime(ctx context.Context) (*string, error) {
	if err := k.ensureInitialized(); err != nil {
		return nil, WrapConnection("KnownNetwork.ensureInitialized", err)
	}

	value, err := k.call.GetProperty(ctx, IwdKnownNetworkIface, "LastConnectedTime")
	if err != nil {
		if isUnknownPropertyError(err) {
			return nil, nil
		}
		return nil, WrapProperty(IwdKnownNetworkIface, "LastConnectedTime", err)
	}

	t, err := parseOptionalString(value)
	if err != nil {
		return nil, WrapVariant("LastConnectedTime", err)
	}
	return t, nil
}

// GetAutoConnect reads the AutoConnect property.
func (k *KnownNetwork) GetAutoConnect(ctx context.Context) (bool, error) {
	if err := k.ensureInitialized(); err != nil {
		return false, WrapConnection("KnownNetwork.ensureInitialized", err)
	}

	value, err := k.call.GetProperty(ctx, IwdKnownNetworkIface, "AutoConnect")
	if err != nil {
		return false, WrapProperty(IwdKnownNetworkIface, "AutoConnect", err)
	}

	b, ok := value.(bool)
	if !ok {
		return false, WrapVariant("AutoConnect", fmt.Errorf("expected bool, got %T", value))
	}
	return b, nil
}

// SetAutoConnect sets the AutoConnect property.
func (k *KnownNetwork) SetAutoConnect(ctx context.Context, val bool) error {
	if err := k.ensureInitialized(); err != nil {
		return WrapConnection("KnownNetwork.ensureInitialized", err)
	}

	if err := k.call.SetProperty(ctx, IwdKnownNetworkIface, "AutoConnect", val); err != nil {
		return wrapIwdProperty(IwdKnownNetworkIface, "AutoConnect", err)
	}
	return nil
}

// Forget calls KnownNetwork.Forget, removing the network from the known-networks
// list (disconnecting it first if currently connected).
func (k *KnownNetwork) Forget(ctx context.Context) error {
	if err := k.ensureInitialized(); err != nil {
		return WrapConnection("KnownNetwork.ensureInitialized", err)
	}

	if _, err := k.call.Call(ctx, IwdKnownNetworkIface, "Forget"); err != nil {
		return wrapIwdMethod(IwdKnownNetworkIface, "Forget", err)
	}
	return nil
}

// KnownNetworkProperties holds every known-network property read in a single
// Properties.GetAll call. LastConnectedTime is optional: a nil pointer means the
// network has never been successfully connected to (the property is absent).
type KnownNetworkProperties struct {
	Name              string
	Type              NetworkType
	Hidden            bool
	LastConnectedTime *string
	AutoConnect       bool
}

// GetProperties reads every known-network property in a single Properties.GetAll
// call instead of one Get per property.
//
// Name, Type, Hidden, and AutoConnect are required; a missing one is an error.
// LastConnectedTime is optional and left nil when absent.
func (k *KnownNetwork) GetProperties(ctx context.Context) (*KnownNetworkProperties, error) {
	if err := k.ensureInitialized(); err != nil {
		return nil, WrapConnection("KnownNetwork.ensureInitialized", err)
	}

	raw, err := k.call.GetAll(ctx, IwdKnownNetworkIface)
	if err != nil {
		return nil, WrapProperty(IwdKnownNetworkIface, "GetAll", err)
	}

	props := &KnownNetworkProperties{}

	nameV, ok := raw["Name"]
	if !ok {
		return nil, WrapProperty(IwdKnownNetworkIface, "Name", fmt.Errorf("missing required property"))
	}
	name, ok := nameV.Value().(string)
	if !ok {
		return nil, WrapVariant("Name", fmt.Errorf("expected string, got %T", nameV.Value()))
	}
	props.Name = name

	typeV, ok := raw["Type"]
	if !ok {
		return nil, WrapProperty(IwdKnownNetworkIface, "Type", fmt.Errorf("missing required property"))
	}
	secType, err := parseNetworkType(typeV.Value())
	if err != nil {
		return nil, err
	}
	props.Type = secType

	hiddenV, ok := raw["Hidden"]
	if !ok {
		return nil, WrapProperty(IwdKnownNetworkIface, "Hidden", fmt.Errorf("missing required property"))
	}
	hidden, ok := hiddenV.Value().(bool)
	if !ok {
		return nil, WrapVariant("Hidden", fmt.Errorf("expected bool, got %T", hiddenV.Value()))
	}
	props.Hidden = hidden

	autoV, ok := raw["AutoConnect"]
	if !ok {
		return nil, WrapProperty(IwdKnownNetworkIface, "AutoConnect", fmt.Errorf("missing required property"))
	}
	auto, ok := autoV.Value().(bool)
	if !ok {
		return nil, WrapVariant("AutoConnect", fmt.Errorf("expected bool, got %T", autoV.Value()))
	}
	props.AutoConnect = auto

	if ltV, ok := raw["LastConnectedTime"]; ok {
		lt, err := parseOptionalString(ltV.Value())
		if err != nil {
			return nil, WrapVariant("LastConnectedTime", err)
		}
		props.LastConnectedTime = lt
	}

	return props, nil
}

// SubscribePropertiesChanged registers fn for raw known-network property-change
// signals.
func (k *KnownNetwork) SubscribePropertiesChanged(ctx context.Context, fn func(KnownNetworkPropertiesChanged)) (UnsubscribeFunc, error) {
	if fn == nil {
		return nil, fmt.Errorf("SubscribePropertiesChanged: fn cannot be nil")
	}

	return k.signals.RegisterSignalHandlerWithUnsubscribe("org.freedesktop.DBus.Properties", "PropertiesChanged", func(sig *dbus.Signal) {
		if sig == nil || len(sig.Body) < 3 {
			return
		}

		iface, ok := sig.Body[0].(string)
		if !ok || iface != IwdKnownNetworkIface {
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

		fn(KnownNetworkPropertiesChanged{
			Changed:     changed,
			Invalidated: invalid,
		})
	})
}

// SubscribeAutoConnectChanged registers fn for raw auto-connect changes.
func (k *KnownNetwork) SubscribeAutoConnectChanged(ctx context.Context, fn func(bool)) (UnsubscribeFunc, error) {
	if fn == nil {
		return nil, fmt.Errorf("SubscribeAutoConnectChanged: fn cannot be nil")
	}

	return k.SubscribePropertiesChanged(ctx, func(ev KnownNetworkPropertiesChanged) {
		variant, ok := ev.Changed["AutoConnect"]
		if !ok {
			return
		}

		b, ok := variant.Value().(bool)
		if ok {
			fn(b)
		}
	})
}

// SubscribeHiddenChanged registers fn for raw changes to the Hidden property.
func (k *KnownNetwork) SubscribeHiddenChanged(ctx context.Context, fn func(bool)) (UnsubscribeFunc, error) {
	if fn == nil {
		return nil, fmt.Errorf("SubscribeHiddenChanged: fn cannot be nil")
	}

	return k.SubscribePropertiesChanged(ctx, func(ev KnownNetworkPropertiesChanged) {
		variant, ok := ev.Changed["Hidden"]
		if !ok {
			return
		}

		b, ok := variant.Value().(bool)
		if ok {
			fn(b)
		}
	})
}

// SubscribeLastConnectedTimeChanged registers fn for raw changes to
// LastConnectedTime, an ISO 8601 timestamp. iwd updates it on each successful
// connection, so this fires once per connect to the network. fn receives nil if
// iwd ever reports the property as absent (a network never connected to).
func (k *KnownNetwork) SubscribeLastConnectedTimeChanged(ctx context.Context, fn func(*string)) (UnsubscribeFunc, error) {
	if fn == nil {
		return nil, fmt.Errorf("SubscribeLastConnectedTimeChanged: fn cannot be nil")
	}

	return k.SubscribePropertiesChanged(ctx, func(ev KnownNetworkPropertiesChanged) {
		variant, ok := ev.Changed["LastConnectedTime"]
		if !ok {
			if propertyCleared(ev.Invalidated, "LastConnectedTime") {
				fn(nil)
			}
			return
		}

		t, err := parseOptionalString(variant.Value())
		if err != nil {
			return
		}
		fn(t)
	})
}

// Firehose emits high-frequency known-network signals for stress and integration
// tests.
func (k *KnownNetwork) Firehose(ctx context.Context, fn func(FirehoseSignal)) error {
	if fn == nil {
		return fmt.Errorf("Firehose: fn cannot be nil")
	}

	// Wildcard interface ("*") + wildcard member ("*") gives all signals.
	return k.signals.RegisterSignalHandler("*", "*", func(sig *dbus.Signal) {
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

// ensureInitialized verifies that k has been initialized by NewKnownNetwork.
func (k *KnownNetwork) ensureInitialized() error {
	if k.call == nil {
		return ErrKnownNetworkUninitialized
	}
	return nil
}
