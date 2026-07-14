package core

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/godbus/dbus/v5"

	"github.com/chrispypip/spiderw/internal/iwdbus"
	"github.com/chrispypip/spiderw/internal/iwdvalue"
)

// NetworkType identifies an iwd network type.
type NetworkType = iwdvalue.NetworkType

// NetworkType constants identify canonical iwd network types.
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

// NetworkPropertiesChanged describes normalized network property-change data.
type NetworkPropertiesChanged struct {
	// Changed contains normalized property values keyed by property name.
	Changed map[string]any

	// Invalidated contains property names whose values should be re-read if needed.
	Invalidated []string
}

type networkRaw interface {
	GetName(ctx context.Context) (string, error)
	GetConnected(ctx context.Context) (bool, error)
	GetDevice(ctx context.Context) (dbus.ObjectPath, error)
	GetType(ctx context.Context) (iwdbus.NetworkType, error)
	GetKnownNetwork(ctx context.Context) (*string, error)
	GetExtendedServiceSet(ctx context.Context) ([]string, error)
	GetProperties(ctx context.Context) (*iwdbus.NetworkProperties, error)
	Connect(ctx context.Context) error
	SubscribePropertiesChanged(ctx context.Context, fn func(iwdbus.NetworkPropertiesChanged)) (iwdbus.UnsubscribeFunc, error)
	SubscribeConnectedChanged(ctx context.Context, fn func(bool)) (iwdbus.UnsubscribeFunc, error)
	SubscribeKnownNetworkChanged(ctx context.Context, fn func(*string)) (iwdbus.UnsubscribeFunc, error)
	SubscribeExtendedServiceSetChanged(ctx context.Context, fn func([]string)) (iwdbus.UnsubscribeFunc, error)
}

// NetworkIface defines the core network operations used by the public layer.
type NetworkIface interface {
	Name(ctx context.Context) (string, error)
	Connected(ctx context.Context) (bool, error)
	Device(ctx context.Context) (string, error)
	Type(ctx context.Context) (NetworkType, error)
	KnownNetwork(ctx context.Context) (*string, error)
	ExtendedServiceSet(ctx context.Context) ([]string, error)
	Properties(ctx context.Context) (*NetworkProperties, error)
	Connect(ctx context.Context) error
	SubscribePropertiesChanged(ctx context.Context, fn func(NetworkPropertiesChanged)) (UnsubscribeFunc, error)
	SubscribeConnectedChanged(ctx context.Context, fn func(bool)) (UnsubscribeFunc, error)
	SubscribeKnownNetworkChanged(ctx context.Context, fn func(*string)) (UnsubscribeFunc, error)
	SubscribeExtendedServiceSetChanged(ctx context.Context, fn func([]string)) (UnsubscribeFunc, error)
}

// NetworkProperties holds normalized network properties read in a single backend
// call. KnownNetwork is optional: a nil pointer means the network has no
// known-network record.
type NetworkProperties struct {
	Name               string
	Connected          bool
	Device             string
	Type               NetworkType
	KnownNetwork       *string
	ExtendedServiceSet []string
}

// Network is the core-layer facade over a raw iwd network backend.
type Network struct {
	raw networkRaw
}

// NewNetwork wraps a raw network backend in a core-layer Network.
func NewNetwork(raw networkRaw) *Network {
	if raw == nil {
		return nil
	}
	return &Network{raw: raw}
}

func (n *Network) rawNetwork(op string) (networkRaw, error) {
	if n == nil || n.raw == nil {
		return nil, WrapInvalidState(ResourceNetwork, op, "network wrapper was nil", ErrNetworkNotInitialized)
	}
	return n.raw, nil
}

// Name returns the normalized network name (SSID).
func (n *Network) Name(ctx context.Context) (string, error) {
	const op = "Network.Name"

	rawNetwork, err := n.rawNetwork(op)
	if err != nil {
		return "", err
	}

	raw, err := rawNetwork.GetName(ctx)
	if err != nil {
		return "", WrapNetworkUnavailable(op, "failed querying iwd Network name", err)
	}

	name := strings.TrimSpace(raw)
	if name == "" {
		return "", WrapInvalidState(ResourceNetwork, op, "network returned empty Name", fmt.Errorf("missing or invalid Name field"))
	}

	return name, nil
}

// Connected reports whether the network is currently connected or connecting.
func (n *Network) Connected(ctx context.Context) (bool, error) {
	const op = "Network.Connected"

	rawNetwork, err := n.rawNetwork(op)
	if err != nil {
		return false, err
	}

	value, err := rawNetwork.GetConnected(ctx)
	if err != nil {
		return false, WrapNetworkUnavailable(op, "failed querying iwd Network connected", err)
	}

	return value, nil
}

// Device returns the object path of the device/station the network belongs to.
func (n *Network) Device(ctx context.Context) (string, error) {
	const op = "Network.Device"

	rawNetwork, err := n.rawNetwork(op)
	if err != nil {
		return "", err
	}

	raw, err := rawNetwork.GetDevice(ctx)
	if err != nil {
		return "", WrapNetworkUnavailable(op, "failed querying iwd Network device", err)
	}

	path := strings.TrimSpace(string(raw))
	if path == "" {
		return "", WrapInvalidState(ResourceNetwork, op, "network returned empty Device", fmt.Errorf("missing or invalid Device field"))
	}

	return path, nil
}

// Type returns the normalized network type.
func (n *Network) Type(ctx context.Context) (NetworkType, error) {
	const op = "Network.Type"

	rawNetwork, err := n.rawNetwork(op)
	if err != nil {
		return NetworkTypeUnknown, err
	}

	raw, err := rawNetwork.GetType(ctx)
	if err != nil {
		return NetworkTypeUnknown, WrapNetworkUnavailable(op, "failed querying iwd Network type", err)
	}

	return validateNetworkType(ResourceNetwork, op, raw)
}

// KnownNetwork returns the object path of the network's known-network record, or
// nil when the network is not known/provisioned.
func (n *Network) KnownNetwork(ctx context.Context) (*string, error) {
	const op = "Network.KnownNetwork"

	rawNetwork, err := n.rawNetwork(op)
	if err != nil {
		return nil, err
	}

	raw, err := rawNetwork.GetKnownNetwork(ctx)
	if err != nil {
		return nil, WrapNetworkUnavailable(op, "failed querying iwd Network known-network", err)
	}

	return normalizeOptionalString(raw), nil
}

// ExtendedServiceSet returns the object paths of the basic service sets (BSSes)
// that make up this network.
func (n *Network) ExtendedServiceSet(ctx context.Context) ([]string, error) {
	const op = "Network.ExtendedServiceSet"

	rawNetwork, err := n.rawNetwork(op)
	if err != nil {
		return nil, err
	}

	raw, err := rawNetwork.GetExtendedServiceSet(ctx)
	if err != nil {
		return nil, WrapNetworkUnavailable(op, "failed querying iwd Network extended service set", err)
	}

	return normalizePathList(op, raw)
}

// Connect requests that the owning device connect to this network.
func (n *Network) Connect(ctx context.Context) error {
	const op = "Network.Connect"

	rawNetwork, err := n.rawNetwork(op)
	if err != nil {
		return err
	}

	if err := rawNetwork.Connect(ctx); err != nil {
		if errors.Is(err, iwdbus.ErrNoAgent) {
			return WrapNetworkUnavailable(op, "no credentials agent registered; connecting to a secured network that is not already known requires an agent", err)
		}
		return WrapNetworkUnavailable(op, "failed connecting to iwd Network", err)
	}

	return nil
}

// Properties returns all normalized network properties read in a single backend
// call (Properties.GetAll), applying the same normalization as the per-property
// getters.
func (n *Network) Properties(ctx context.Context) (*NetworkProperties, error) {
	const op = "Network.Properties"

	rawNetwork, err := n.rawNetwork(op)
	if err != nil {
		return nil, err
	}

	raw, err := rawNetwork.GetProperties(ctx)
	if err != nil {
		return nil, WrapNetworkUnavailable(op, "failed querying iwd Network properties", err)
	}

	name := strings.TrimSpace(raw.Name)
	if name == "" {
		return nil, WrapInvalidState(ResourceNetwork, op, "network returned empty Name", fmt.Errorf("missing or invalid Name field"))
	}

	device := strings.TrimSpace(string(raw.Device))
	if device == "" {
		return nil, WrapInvalidState(ResourceNetwork, op, "network returned empty Device", fmt.Errorf("missing or invalid Device field"))
	}

	secType, err := validateNetworkType(ResourceNetwork, op, raw.Type)
	if err != nil {
		return nil, err
	}

	ess, err := normalizePathList(op, raw.ExtendedServiceSet)
	if err != nil {
		return nil, err
	}

	return &NetworkProperties{
		Name:               name,
		Connected:          raw.Connected,
		Device:             device,
		Type:               secType,
		KnownNetwork:       normalizeOptionalString(raw.KnownNetwork),
		ExtendedServiceSet: ess,
	}, nil
}

// SubscribePropertiesChanged registers fn for normalized property-change events.
func (n *Network) SubscribePropertiesChanged(ctx context.Context, fn func(NetworkPropertiesChanged)) (UnsubscribeFunc, error) {
	const op = "Network.SubscribePropertiesChanged"

	rawNetwork, err := n.rawNetwork(op)
	if err != nil {
		return nil, err
	}
	if fn == nil {
		return nil, WrapInvalidArgument(ResourceNetwork, op, "callback cannot be nil", ErrCore)
	}

	unsub, err := rawNetwork.SubscribePropertiesChanged(ctx, func(raw iwdbus.NetworkPropertiesChanged) {
		changed := make(map[string]any, len(raw.Changed))
		for k, v := range raw.Changed {
			changed[k] = v.Value()
		}
		// Copy invalidated to avoid aliasing/mutation across layers.
		var invalidated []string
		if raw.Invalidated != nil {
			invalidated = slices.Clone(raw.Invalidated)
		}

		fn(NetworkPropertiesChanged{
			Changed:     changed,
			Invalidated: invalidated,
		})
	})
	if err != nil {
		return nil, WrapNetworkUnavailable(op, "failed to call iwd Network subscribe method", err)
	}
	return UnsubscribeFunc(unsub), nil
}

// SubscribeConnectedChanged registers fn for normalized connected-state events.
func (n *Network) SubscribeConnectedChanged(ctx context.Context, fn func(bool)) (UnsubscribeFunc, error) {
	const op = "Network.SubscribeConnectedChanged"

	rawNetwork, err := n.rawNetwork(op)
	if err != nil {
		return nil, err
	}
	if fn == nil {
		return nil, WrapInvalidArgument(ResourceNetwork, op, "callback cannot be nil", ErrCore)
	}

	unsub, err := rawNetwork.SubscribeConnectedChanged(ctx, fn)
	if err != nil {
		return nil, WrapNetworkUnavailable(op, "failed to call iwd Network subscribe method", err)
	}
	return UnsubscribeFunc(unsub), nil
}

// validateNetworkType ensures the backend reported a recognized iwd network
// type, treating an unknown value as invalid state.
func validateNetworkType(resource Resource, op string, t iwdbus.NetworkType) (NetworkType, error) {
	if !iwdvalue.ValidNetworkType(t) {
		details := fmt.Sprintf("network reported unknown type %q", t)
		return NetworkTypeUnknown, WrapInvalidState(resource, op, details, fmt.Errorf("missing or invalid Type field"))
	}
	return t, nil
}

// normalizeOptionalString trims an optional string, returning nil when the
// value is absent or blank after trimming.
func normalizeOptionalString(p *string) *string {
	if p == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*p)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

// normalizePathList trims each object path and rejects any that is empty after
// trimming.
func normalizePathList(op string, paths []string) ([]string, error) {
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		trimmed := strings.TrimSpace(p)
		if trimmed == "" {
			return nil, WrapInvalidState(ResourceNetwork, op, "network returned empty basic service set path", fmt.Errorf("missing or invalid ExtendedServiceSet entry"))
		}
		out = append(out, trimmed)
	}
	return out, nil
}

// SubscribeKnownNetworkChanged registers fn for changes to the KnownNetwork association. fn receives
// the known-network object path, or nil when the network is not known. This is
// how a network being saved or forgotten is observed.
func (n *Network) SubscribeKnownNetworkChanged(ctx context.Context, fn func(*string)) (UnsubscribeFunc, error) {
	const op = "Network.SubscribeKnownNetworkChanged"

	raw, err := n.rawNetwork(op)
	if err != nil {
		return nil, err
	}
	if fn == nil {
		return nil, WrapInvalidArgument(ResourceNetwork, op, "callback cannot be nil", ErrCore)
	}

	unsub, err := raw.SubscribeKnownNetworkChanged(ctx, fn)
	if err != nil {
		return nil, WrapNetworkUnavailable(op, "failed to call iwd Network subscribe method", err)
	}
	return UnsubscribeFunc(unsub), nil
}

// SubscribeExtendedServiceSetChanged registers fn for changes to the network BSS list, which changes as
// access points come and go across scans.
func (n *Network) SubscribeExtendedServiceSetChanged(ctx context.Context, fn func([]string)) (UnsubscribeFunc, error) {
	const op = "Network.SubscribeExtendedServiceSetChanged"

	raw, err := n.rawNetwork(op)
	if err != nil {
		return nil, err
	}
	if fn == nil {
		return nil, WrapInvalidArgument(ResourceNetwork, op, "callback cannot be nil", ErrCore)
	}

	unsub, err := raw.SubscribeExtendedServiceSetChanged(ctx, fn)
	if err != nil {
		return nil, WrapNetworkUnavailable(op, "failed to call iwd Network subscribe method", err)
	}
	return UnsubscribeFunc(unsub), nil
}
