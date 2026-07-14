package spiderw

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/chrispypip/spiderw/internal/connect"
	"github.com/chrispypip/spiderw/internal/core"
	"github.com/chrispypip/spiderw/internal/iwdvalue"
	"github.com/chrispypip/spiderw/internal/logging"
)

// NetworkType identifies the network type of an iwd network.
type NetworkType string

// NetworkType constants identify the supported iwd network types.
// NetworkTypeUnknown is reserved for invalid or unrecognized values.
const (
	// NetworkTypeUnknown represents an invalid or unrecognized network type.
	NetworkTypeUnknown NetworkType = NetworkType(iwdvalue.NetworkTypeUnknown)

	// NetworkTypeOpen is an open (unsecured) network.
	NetworkTypeOpen NetworkType = NetworkType(iwdvalue.NetworkTypeOpen)

	// NetworkTypeWEP is a WEP network.
	NetworkTypeWEP NetworkType = NetworkType(iwdvalue.NetworkTypeWEP)

	// NetworkTypePSK is a pre-shared-key (WPA-Personal) network.
	NetworkTypePSK NetworkType = NetworkType(iwdvalue.NetworkTypePSK)

	// NetworkType8021x is an 802.1x (EAP / WPA-Enterprise) network.
	NetworkType8021x NetworkType = NetworkType(iwdvalue.NetworkType8021x)

	// NetworkTypeHotspot is a hotspot network (reported only for a KnownNetwork).
	NetworkTypeHotspot NetworkType = NetworkType(iwdvalue.NetworkTypeHotspot)
)

// String returns the canonical iwd string for the network type.
func (s NetworkType) String() string {
	return iwdvalue.NetworkType(s).String()
}

// NetworkPropertiesChanged describes network properties reported by a D-Bus
// PropertiesChanged signal. Changed contains the new values by property name;
// Invalidated contains property names whose values should be re-read if needed.
type NetworkPropertiesChanged struct {
	// Changed contains new property values keyed by property name.
	Changed map[string]any

	// Invalidated contains property names whose values should be re-read if needed.
	Invalidated []string
}

// NetworkProperties is a snapshot of all network properties read in a single
// D-Bus call. KnownNetwork is nil when the network has no known-network record.
type NetworkProperties struct {
	// Name is the network's SSID.
	Name string

	// Connected reports whether the network is currently connected or connecting.
	Connected bool

	// Device references the device/station the network belongs to (Path + resolved
	// Name).
	Device DeviceRef

	// Type is the network's network type.
	Type NetworkType

	// KnownNetwork is the object path of the network's known-network record, or
	// nil when the network is not known/provisioned. Unlike the other bundle
	// cross-references this stays a bare path: a known network's Name is always
	// this network's own SSID, so resolving it would be redundant. Resolve it to a
	// handle with Client.KnownNetwork.
	KnownNetwork *string

	// ExtendedServiceSet references the basic service sets (BSSes) that make up
	// this network (Path + resolved Address).
	ExtendedServiceSet []BasicServiceSetRef
}

// Network provides high-level operations for a specific iwd network object.
//
// A network represents an SSID the owning device can see. Its
// ExtendedServiceSet lists the basic service sets (access points) that serve it.
type Network struct {
	core     core.NetworkIface
	path     string
	resolver connect.Resolver
}

func newNetwork(c core.NetworkIface, path string) *Network {
	if c == nil {
		return nil
	}
	return &Network{core: c, path: path}
}

// withResolver attaches a resolver for enriching Properties path fields with
// friendly identifiers. The Client sets it at construction; a nil resolver
// leaves bundle refs path-only.
func (n *Network) withResolver(r connect.Resolver) *Network {
	if n != nil {
		n.resolver = r
	}
	return n
}

// Path returns the D-Bus object path the network was constructed from.
//
// Path is static object identity, not an iwd property: it requires no D-Bus
// round-trip and never fails. Path returns "" for a nil receiver.
func (n *Network) Path() string {
	if n == nil {
		return ""
	}
	return n.path
}

func (n *Network) coreNetwork(ctx context.Context, op string) (core.NetworkIface, error) {
	if n == nil || n.core == nil {
		logging.FromContext(ctx).Error(ctx, "network wrapper uninitialized", "op", op)
		return nil, wrapPublicError(op, ErrInternal)
	}
	return n.core, nil
}

// Name returns the network SSID.
func (n *Network) Name(ctx context.Context) (string, error) {
	return delegate(ctx, "Network.Name", n.coreNetwork, func(ctx context.Context, c core.NetworkIface) (string, error) {
		return c.Name(ctx)
	})
}

// Connected reports whether the network is currently connected or connecting.
func (n *Network) Connected(ctx context.Context) (bool, error) {
	return delegate(ctx, "Network.Connected", n.coreNetwork, func(ctx context.Context, c core.NetworkIface) (bool, error) {
		return c.Connected(ctx)
	})
}

// Device returns the object path of the device/station the network belongs to.
//
// Resolve it to a handle with Client.Device.
func (n *Network) Device(ctx context.Context) (string, error) {
	return delegate(ctx, "Network.Device", n.coreNetwork, func(ctx context.Context, c core.NetworkIface) (string, error) {
		return c.Device(ctx)
	})
}

// Type returns the network's network type.
func (n *Network) Type(ctx context.Context) (NetworkType, error) {
	return delegate(ctx, "Network.Type", n.coreNetwork, func(ctx context.Context, c core.NetworkIface) (NetworkType, error) {
		ct, err := c.Type(ctx)
		if err != nil {
			return NetworkTypeUnknown, err
		}
		return convertNetworkType(ct)
	})
}

// KnownNetwork returns the object path of the network's known-network record, or
// nil when the network is not known/provisioned.
//
// Resolve it to a handle with Client.KnownNetwork.
func (n *Network) KnownNetwork(ctx context.Context) (*string, error) {
	return delegate(ctx, "Network.KnownNetwork", n.coreNetwork, func(ctx context.Context, c core.NetworkIface) (*string, error) {
		return c.KnownNetwork(ctx)
	})
}

// ExtendedServiceSet returns the object paths of the basic service sets (BSSes)
// that make up this network. Resolve each with Client.BasicServiceSet.
func (n *Network) ExtendedServiceSet(ctx context.Context) ([]string, error) {
	return delegate(ctx, "Network.ExtendedServiceSet", n.coreNetwork, func(ctx context.Context, c core.NetworkIface) ([]string, error) {
		return c.ExtendedServiceSet(ctx)
	})
}

// Connect requests that the owning device connect to this network.
//
// Open networks and networks iwd already knows connect without a credentials
// agent. Connecting to a secured network that is not already known fails with an
// error matching ErrNoAgent until an agent is registered to supply credentials.
func (n *Network) Connect(ctx context.Context) error {
	return do(ctx, "Network.Connect", n.coreNetwork, func(ctx context.Context, c core.NetworkIface) error {
		return c.Connect(ctx)
	})
}

// Properties reads every network property in a single D-Bus call
// (Properties.GetAll) instead of one call per property.
func (n *Network) Properties(ctx context.Context) (*NetworkProperties, error) {
	return delegate(ctx, "Network.Properties", n.coreNetwork, func(ctx context.Context, c core.NetworkIface) (*NetworkProperties, error) {
		cp, err := c.Properties(ctx)
		if err != nil {
			return nil, err
		}

		secType, err := convertNetworkType(cp.Type)
		if err != nil {
			return nil, err
		}

		tree, err := resolveTree(ctx, n.resolver)
		if err != nil {
			return nil, err
		}

		out := &NetworkProperties{
			Name:         cp.Name,
			Connected:    cp.Connected,
			Device:       deviceRefOf(tree, cp.Device),
			Type:         secType,
			KnownNetwork: cp.KnownNetwork, // bare path (see field doc)
		}
		if len(cp.ExtendedServiceSet) > 0 {
			out.ExtendedServiceSet = make([]BasicServiceSetRef, 0, len(cp.ExtendedServiceSet))
			for _, p := range cp.ExtendedServiceSet {
				out.ExtendedServiceSet = append(out.ExtendedServiceSet, bssRefOf(tree, p))
			}
		}
		return out, nil
	})
}

// SubscribePropertiesChanged registers fn for network property-change signals and
// returns a handle that unregisters the callback.
func (n *Network) SubscribePropertiesChanged(ctx context.Context, fn func(NetworkPropertiesChanged)) (UnsubscribeFunc, error) {
	const op = "Network.SubscribePropertiesChanged"

	coreNetwork, err := n.coreNetwork(ctx, op)
	if err != nil {
		return nil, err
	}
	if fn == nil {
		return nil, &Error{Kind: KindInvalidArgument, Resource: ResourceNetwork, Op: op, Details: "callback cannot be nil", Err: ErrInvalidArgument}
	}

	unsubscribe, err := coreNetwork.SubscribePropertiesChanged(ctx, func(core core.NetworkPropertiesChanged) {
		changed := make(map[string]any, len(core.Changed))
		maps.Copy(changed, core.Changed)

		// Copy invalidated to avoid aliasing/mutation across layers.
		var invalidated []string
		if core.Invalidated != nil {
			invalidated = slices.Clone(core.Invalidated)
		}

		fn(NetworkPropertiesChanged{
			Changed:     changed,
			Invalidated: invalidated,
		})
	})
	if err != nil {
		return nil, wrapPublicError(op, err)
	}
	return UnsubscribeFunc(unsubscribe), nil
}

// SubscribeConnectedChanged registers fn for network connected-state changes and
// returns a handle that unregisters the callback.
func (n *Network) SubscribeConnectedChanged(ctx context.Context, fn func(bool)) (UnsubscribeFunc, error) {
	const op = "Network.SubscribeConnectedChanged"

	coreNetwork, err := n.coreNetwork(ctx, op)
	if err != nil {
		return nil, err
	}
	if fn == nil {
		return nil, &Error{Kind: KindInvalidArgument, Resource: ResourceNetwork, Op: op, Details: "callback cannot be nil", Err: ErrInvalidArgument}
	}

	unsubscribe, err := coreNetwork.SubscribeConnectedChanged(ctx, fn)
	if err != nil {
		return nil, wrapPublicError(op, err)
	}
	return UnsubscribeFunc(unsubscribe), nil
}

func convertNetworkType(t core.NetworkType) (NetworkType, error) {
	if !iwdvalue.ValidNetworkType(t) {
		details := fmt.Sprintf("invalid network type %q", t)
		return NetworkTypeUnknown, &Error{Kind: KindInvalidArgument, Resource: ResourceNetwork, Op: "Network.convertType", Details: details, Err: ErrInvalidArgument}
	}
	return NetworkType(t), nil
}

// SubscribeKnownNetworkChanged registers fn for changes to the network's
// known-network association. fn receives the known-network object path, or nil
// when the network is not known.
//
// This is how a network being saved or forgotten is observed: provisioning gives
// the network a known-network record, and forgetting it takes it away.
func (n *Network) SubscribeKnownNetworkChanged(ctx context.Context, fn func(*string)) (UnsubscribeFunc, error) {
	const op = "Network.SubscribeKnownNetworkChanged"

	coreNetwork, err := n.coreNetwork(ctx, op)
	if err != nil {
		return nil, err
	}
	if fn == nil {
		return nil, &Error{Kind: KindInvalidArgument, Resource: ResourceNetwork, Op: op, Details: "callback cannot be nil", Err: ErrInvalidArgument}
	}

	unsubscribe, err := coreNetwork.SubscribeKnownNetworkChanged(ctx, fn)
	if err != nil {
		return nil, wrapPublicError(op, err)
	}
	return UnsubscribeFunc(unsubscribe), nil
}

// SubscribeExtendedServiceSetChanged registers fn for changes to the network's
// BSS list. fn receives the BSS object paths, which change as access points for
// the network come and go across scans.
func (n *Network) SubscribeExtendedServiceSetChanged(ctx context.Context, fn func([]string)) (UnsubscribeFunc, error) {
	const op = "Network.SubscribeExtendedServiceSetChanged"

	coreNetwork, err := n.coreNetwork(ctx, op)
	if err != nil {
		return nil, err
	}
	if fn == nil {
		return nil, &Error{Kind: KindInvalidArgument, Resource: ResourceNetwork, Op: op, Details: "callback cannot be nil", Err: ErrInvalidArgument}
	}

	unsubscribe, err := coreNetwork.SubscribeExtendedServiceSetChanged(ctx, fn)
	if err != nil {
		return nil, wrapPublicError(op, err)
	}
	return UnsubscribeFunc(unsubscribe), nil
}
