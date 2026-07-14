package spiderw

import (
	"context"
	"maps"
	"slices"

	"github.com/chrispypip/spiderw/internal/core"
	"github.com/chrispypip/spiderw/internal/logging"
)

// KnownNetworkPropertiesChanged describes known-network properties reported by a
// D-Bus PropertiesChanged signal. Changed contains the new values by property
// name; Invalidated contains property names whose values should be re-read if
// needed.
type KnownNetworkPropertiesChanged struct {
	// Changed contains new property values keyed by property name.
	Changed map[string]any

	// Invalidated contains property names whose values should be re-read if needed.
	Invalidated []string
}

// KnownNetworkProperties is a snapshot of all known-network properties read in a
// single D-Bus call. LastConnectedTime is nil when the network has never been
// successfully connected to.
type KnownNetworkProperties struct {
	// Name is the known network's name (usually the SSID).
	Name string

	// Type is the known network's type.
	Type NetworkType

	// Hidden reports whether the network is hidden.
	Hidden bool

	// LastConnectedTime is the ISO 8601 timestamp of the last successful
	// connection, or nil when the network has never been connected to.
	LastConnectedTime *string

	// AutoConnect reports whether the network is a candidate for automatic
	// connection.
	AutoConnect bool
}

// KnownNetwork provides high-level operations for a specific iwd known-network
// object.
//
// A known network is one iwd has stored configuration for (a previously
// connected or provisioned network).
type KnownNetwork struct {
	core core.KnownNetworkIface
	path string
}

func newKnownNetwork(c core.KnownNetworkIface, path string) *KnownNetwork {
	if c == nil {
		return nil
	}
	return &KnownNetwork{core: c, path: path}
}

// Path returns the D-Bus object path the known network was constructed from.
//
// Path is static object identity, not an iwd property: it requires no D-Bus
// round-trip and never fails. Path returns "" for a nil receiver.
func (k *KnownNetwork) Path() string {
	if k == nil {
		return ""
	}
	return k.path
}

func (k *KnownNetwork) coreKnownNetwork(ctx context.Context, op string) (core.KnownNetworkIface, error) {
	if k == nil || k.core == nil {
		logging.FromContext(ctx).Error(ctx, "known network wrapper uninitialized", "op", op)
		return nil, wrapPublicError(op, ErrInternal)
	}
	return k.core, nil
}

// Name returns the known network's name (usually the SSID).
func (k *KnownNetwork) Name(ctx context.Context) (string, error) {
	return delegate(ctx, "KnownNetwork.Name", k.coreKnownNetwork, func(ctx context.Context, c core.KnownNetworkIface) (string, error) {
		return c.Name(ctx)
	})
}

// Type returns the known network's type.
func (k *KnownNetwork) Type(ctx context.Context) (NetworkType, error) {
	return delegate(ctx, "KnownNetwork.Type", k.coreKnownNetwork, func(ctx context.Context, c core.KnownNetworkIface) (NetworkType, error) {
		ct, err := c.Type(ctx)
		if err != nil {
			return NetworkTypeUnknown, err
		}
		return convertNetworkType(ct)
	})
}

// Hidden reports whether the known network is hidden.
func (k *KnownNetwork) Hidden(ctx context.Context) (bool, error) {
	return delegate(ctx, "KnownNetwork.Hidden", k.coreKnownNetwork, func(ctx context.Context, c core.KnownNetworkIface) (bool, error) {
		return c.Hidden(ctx)
	})
}

// LastConnectedTime returns the ISO 8601 timestamp of the last successful
// connection, or nil when the network has never been connected to.
func (k *KnownNetwork) LastConnectedTime(ctx context.Context) (*string, error) {
	return delegate(ctx, "KnownNetwork.LastConnectedTime", k.coreKnownNetwork, func(ctx context.Context, c core.KnownNetworkIface) (*string, error) {
		return c.LastConnectedTime(ctx)
	})
}

// AutoConnect reports whether the known network is a candidate for automatic
// connection.
func (k *KnownNetwork) AutoConnect(ctx context.Context) (bool, error) {
	return delegate(ctx, "KnownNetwork.AutoConnect", k.coreKnownNetwork, func(ctx context.Context, c core.KnownNetworkIface) (bool, error) {
		return c.AutoConnect(ctx)
	})
}

// SetAutoConnect changes whether the known network is a candidate for automatic
// connection.
func (k *KnownNetwork) SetAutoConnect(ctx context.Context, autoConnect bool) error {
	return do(ctx, "KnownNetwork.SetAutoConnect", k.coreKnownNetwork, func(ctx context.Context, c core.KnownNetworkIface) error {
		return c.SetAutoConnect(ctx, autoConnect)
	})
}

// Forget removes the known network from iwd, disconnecting it first if currently
// connected.
func (k *KnownNetwork) Forget(ctx context.Context) error {
	return do(ctx, "KnownNetwork.Forget", k.coreKnownNetwork, func(ctx context.Context, c core.KnownNetworkIface) error {
		return c.Forget(ctx)
	})
}

// Properties reads every known-network property in a single D-Bus call
// (Properties.GetAll) instead of one call per property.
func (k *KnownNetwork) Properties(ctx context.Context) (*KnownNetworkProperties, error) {
	return delegate(ctx, "KnownNetwork.Properties", k.coreKnownNetwork, func(ctx context.Context, c core.KnownNetworkIface) (*KnownNetworkProperties, error) {
		cp, err := c.Properties(ctx)
		if err != nil {
			return nil, err
		}

		netType, err := convertNetworkType(cp.Type)
		if err != nil {
			return nil, err
		}

		return &KnownNetworkProperties{
			Name:              cp.Name,
			Type:              netType,
			Hidden:            cp.Hidden,
			LastConnectedTime: cp.LastConnectedTime,
			AutoConnect:       cp.AutoConnect,
		}, nil
	})
}

// SubscribePropertiesChanged registers fn for known-network property-change
// signals and returns a handle that unregisters the callback.
func (k *KnownNetwork) SubscribePropertiesChanged(ctx context.Context, fn func(KnownNetworkPropertiesChanged)) (UnsubscribeFunc, error) {
	const op = "KnownNetwork.SubscribePropertiesChanged"

	coreKnownNetwork, err := k.coreKnownNetwork(ctx, op)
	if err != nil {
		return nil, err
	}
	if fn == nil {
		return nil, &Error{Kind: KindInvalidArgument, Resource: ResourceKnownNetwork, Op: op, Details: "callback cannot be nil", Err: ErrInvalidArgument}
	}

	unsubscribe, err := coreKnownNetwork.SubscribePropertiesChanged(ctx, func(core core.KnownNetworkPropertiesChanged) {
		changed := make(map[string]any, len(core.Changed))
		maps.Copy(changed, core.Changed)

		// Copy invalidated to avoid aliasing/mutation across layers.
		var invalidated []string
		if core.Invalidated != nil {
			invalidated = slices.Clone(core.Invalidated)
		}

		fn(KnownNetworkPropertiesChanged{
			Changed:     changed,
			Invalidated: invalidated,
		})
	})
	if err != nil {
		return nil, wrapPublicError(op, err)
	}
	return UnsubscribeFunc(unsubscribe), nil
}

// SubscribeAutoConnectChanged registers fn for known-network auto-connect changes
// and returns a handle that unregisters the callback.
func (k *KnownNetwork) SubscribeAutoConnectChanged(ctx context.Context, fn func(bool)) (UnsubscribeFunc, error) {
	const op = "KnownNetwork.SubscribeAutoConnectChanged"

	coreKnownNetwork, err := k.coreKnownNetwork(ctx, op)
	if err != nil {
		return nil, err
	}
	if fn == nil {
		return nil, &Error{Kind: KindInvalidArgument, Resource: ResourceKnownNetwork, Op: op, Details: "callback cannot be nil", Err: ErrInvalidArgument}
	}

	unsubscribe, err := coreKnownNetwork.SubscribeAutoConnectChanged(ctx, fn)
	if err != nil {
		return nil, wrapPublicError(op, err)
	}
	return UnsubscribeFunc(unsubscribe), nil
}

// SubscribeHiddenChanged registers fn for changes to the Hidden property.
func (k *KnownNetwork) SubscribeHiddenChanged(ctx context.Context, fn func(bool)) (UnsubscribeFunc, error) {
	const op = "KnownNetwork.SubscribeHiddenChanged"

	coreKnownNetwork, err := k.coreKnownNetwork(ctx, op)
	if err != nil {
		return nil, err
	}
	if fn == nil {
		return nil, &Error{Kind: KindInvalidArgument, Resource: ResourceKnownNetwork, Op: op, Details: "callback cannot be nil", Err: ErrInvalidArgument}
	}

	unsubscribe, err := coreKnownNetwork.SubscribeHiddenChanged(ctx, fn)
	if err != nil {
		return nil, wrapPublicError(op, err)
	}
	return UnsubscribeFunc(unsubscribe), nil
}

// SubscribeLastConnectedTimeChanged registers fn for changes to
// LastConnectedTime, an ISO 8601 timestamp. iwd updates it on each successful
// connection, so this fires once per connect to the network.
func (k *KnownNetwork) SubscribeLastConnectedTimeChanged(ctx context.Context, fn func(*string)) (UnsubscribeFunc, error) {
	const op = "KnownNetwork.SubscribeLastConnectedTimeChanged"

	coreKnownNetwork, err := k.coreKnownNetwork(ctx, op)
	if err != nil {
		return nil, err
	}
	if fn == nil {
		return nil, &Error{Kind: KindInvalidArgument, Resource: ResourceKnownNetwork, Op: op, Details: "callback cannot be nil", Err: ErrInvalidArgument}
	}

	unsubscribe, err := coreKnownNetwork.SubscribeLastConnectedTimeChanged(ctx, fn)
	if err != nil {
		return nil, wrapPublicError(op, err)
	}
	return UnsubscribeFunc(unsubscribe), nil
}
