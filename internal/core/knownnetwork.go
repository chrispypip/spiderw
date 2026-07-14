package core

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/chrispypip/spiderw/internal/iwdbus"
)

// KnownNetworkPropertiesChanged describes normalized known-network
// property-change data.
type KnownNetworkPropertiesChanged struct {
	// Changed contains normalized property values keyed by property name.
	Changed map[string]any

	// Invalidated contains property names whose values should be re-read if needed.
	Invalidated []string
}

type knownNetworkRaw interface {
	GetName(ctx context.Context) (string, error)
	GetType(ctx context.Context) (iwdbus.NetworkType, error)
	GetHidden(ctx context.Context) (bool, error)
	GetLastConnectedTime(ctx context.Context) (*string, error)
	GetAutoConnect(ctx context.Context) (bool, error)
	SetAutoConnect(ctx context.Context, autoConnect bool) error
	Forget(ctx context.Context) error
	GetProperties(ctx context.Context) (*iwdbus.KnownNetworkProperties, error)
	SubscribePropertiesChanged(ctx context.Context, fn func(iwdbus.KnownNetworkPropertiesChanged)) (iwdbus.UnsubscribeFunc, error)
	SubscribeAutoConnectChanged(ctx context.Context, fn func(bool)) (iwdbus.UnsubscribeFunc, error)
	SubscribeHiddenChanged(ctx context.Context, fn func(bool)) (iwdbus.UnsubscribeFunc, error)
	SubscribeLastConnectedTimeChanged(ctx context.Context, fn func(*string)) (iwdbus.UnsubscribeFunc, error)
}

// KnownNetworkIface defines the core known-network operations used by the public
// layer.
type KnownNetworkIface interface {
	Name(ctx context.Context) (string, error)
	Type(ctx context.Context) (NetworkType, error)
	Hidden(ctx context.Context) (bool, error)
	LastConnectedTime(ctx context.Context) (*string, error)
	AutoConnect(ctx context.Context) (bool, error)
	SetAutoConnect(ctx context.Context, autoConnect bool) error
	Forget(ctx context.Context) error
	Properties(ctx context.Context) (*KnownNetworkProperties, error)
	SubscribePropertiesChanged(ctx context.Context, fn func(KnownNetworkPropertiesChanged)) (UnsubscribeFunc, error)
	SubscribeAutoConnectChanged(ctx context.Context, fn func(bool)) (UnsubscribeFunc, error)
	SubscribeHiddenChanged(ctx context.Context, fn func(bool)) (UnsubscribeFunc, error)
	SubscribeLastConnectedTimeChanged(ctx context.Context, fn func(*string)) (UnsubscribeFunc, error)
}

// KnownNetworkProperties holds normalized known-network properties read in a
// single backend call. LastConnectedTime is optional: a nil pointer means the
// network has never been successfully connected to.
type KnownNetworkProperties struct {
	Name              string
	Type              NetworkType
	Hidden            bool
	LastConnectedTime *string
	AutoConnect       bool
}

// KnownNetwork is the core-layer facade over a raw iwd known-network backend.
type KnownNetwork struct {
	raw knownNetworkRaw
}

// NewKnownNetwork wraps a raw known-network backend in a core-layer KnownNetwork.
func NewKnownNetwork(raw knownNetworkRaw) *KnownNetwork {
	if raw == nil {
		return nil
	}
	return &KnownNetwork{raw: raw}
}

func (k *KnownNetwork) rawKnownNetwork(op string) (knownNetworkRaw, error) {
	if k == nil || k.raw == nil {
		return nil, WrapInvalidState(ResourceKnownNetwork, op, "known network wrapper was nil", ErrKnownNetworkNotInitialized)
	}
	return k.raw, nil
}

// Name returns the normalized known-network name.
func (k *KnownNetwork) Name(ctx context.Context) (string, error) {
	const op = "KnownNetwork.Name"

	raw, err := k.rawKnownNetwork(op)
	if err != nil {
		return "", err
	}

	value, err := raw.GetName(ctx)
	if err != nil {
		return "", WrapKnownNetworkUnavailable(op, "failed querying iwd KnownNetwork name", err)
	}

	name := strings.TrimSpace(value)
	if name == "" {
		return "", WrapInvalidState(ResourceKnownNetwork, op, "known network returned empty Name", fmt.Errorf("missing or invalid Name field"))
	}

	return name, nil
}

// Type returns the normalized known-network type.
func (k *KnownNetwork) Type(ctx context.Context) (NetworkType, error) {
	const op = "KnownNetwork.Type"

	raw, err := k.rawKnownNetwork(op)
	if err != nil {
		return NetworkTypeUnknown, err
	}

	value, err := raw.GetType(ctx)
	if err != nil {
		return NetworkTypeUnknown, WrapKnownNetworkUnavailable(op, "failed querying iwd KnownNetwork type", err)
	}

	return validateNetworkType(ResourceKnownNetwork, op, value)
}

// Hidden reports whether the known network is hidden.
func (k *KnownNetwork) Hidden(ctx context.Context) (bool, error) {
	const op = "KnownNetwork.Hidden"

	raw, err := k.rawKnownNetwork(op)
	if err != nil {
		return false, err
	}

	value, err := raw.GetHidden(ctx)
	if err != nil {
		return false, WrapKnownNetworkUnavailable(op, "failed querying iwd KnownNetwork hidden", err)
	}

	return value, nil
}

// LastConnectedTime returns the ISO 8601 timestamp of the last successful
// connection, or nil when the network has never been connected to.
func (k *KnownNetwork) LastConnectedTime(ctx context.Context) (*string, error) {
	const op = "KnownNetwork.LastConnectedTime"

	raw, err := k.rawKnownNetwork(op)
	if err != nil {
		return nil, err
	}

	value, err := raw.GetLastConnectedTime(ctx)
	if err != nil {
		return nil, WrapKnownNetworkUnavailable(op, "failed querying iwd KnownNetwork last-connected time", err)
	}

	return normalizeOptionalString(value), nil
}

// AutoConnect reports whether the known network is a candidate for automatic
// connection.
func (k *KnownNetwork) AutoConnect(ctx context.Context) (bool, error) {
	const op = "KnownNetwork.AutoConnect"

	raw, err := k.rawKnownNetwork(op)
	if err != nil {
		return false, err
	}

	value, err := raw.GetAutoConnect(ctx)
	if err != nil {
		return false, WrapKnownNetworkUnavailable(op, "failed querying iwd KnownNetwork auto-connect", err)
	}

	return value, nil
}

// SetAutoConnect sets whether the known network is a candidate for automatic
// connection.
func (k *KnownNetwork) SetAutoConnect(ctx context.Context, autoConnect bool) error {
	const op = "KnownNetwork.SetAutoConnect"

	raw, err := k.rawKnownNetwork(op)
	if err != nil {
		return err
	}

	if err := raw.SetAutoConnect(ctx, autoConnect); err != nil {
		return WrapKnownNetworkUnavailable(op, "failed setting iwd KnownNetwork auto-connect", err)
	}

	return nil
}

// Forget removes the known network from iwd, disconnecting it first if currently
// connected.
func (k *KnownNetwork) Forget(ctx context.Context) error {
	const op = "KnownNetwork.Forget"

	raw, err := k.rawKnownNetwork(op)
	if err != nil {
		return err
	}

	if err := raw.Forget(ctx); err != nil {
		return WrapKnownNetworkUnavailable(op, "failed forgetting iwd KnownNetwork", err)
	}

	return nil
}

// Properties returns all normalized known-network properties read in a single
// backend call (Properties.GetAll), applying the same normalization as the
// per-property getters.
func (k *KnownNetwork) Properties(ctx context.Context) (*KnownNetworkProperties, error) {
	const op = "KnownNetwork.Properties"

	raw, err := k.rawKnownNetwork(op)
	if err != nil {
		return nil, err
	}

	props, err := raw.GetProperties(ctx)
	if err != nil {
		return nil, WrapKnownNetworkUnavailable(op, "failed querying iwd KnownNetwork properties", err)
	}

	name := strings.TrimSpace(props.Name)
	if name == "" {
		return nil, WrapInvalidState(ResourceKnownNetwork, op, "known network returned empty Name", fmt.Errorf("missing or invalid Name field"))
	}

	netType, err := validateNetworkType(ResourceKnownNetwork, op, props.Type)
	if err != nil {
		return nil, err
	}

	return &KnownNetworkProperties{
		Name:              name,
		Type:              netType,
		Hidden:            props.Hidden,
		LastConnectedTime: normalizeOptionalString(props.LastConnectedTime),
		AutoConnect:       props.AutoConnect,
	}, nil
}

// SubscribePropertiesChanged registers fn for normalized property-change events.
func (k *KnownNetwork) SubscribePropertiesChanged(ctx context.Context, fn func(KnownNetworkPropertiesChanged)) (UnsubscribeFunc, error) {
	const op = "KnownNetwork.SubscribePropertiesChanged"

	raw, err := k.rawKnownNetwork(op)
	if err != nil {
		return nil, err
	}
	if fn == nil {
		return nil, WrapInvalidArgument(ResourceKnownNetwork, op, "callback cannot be nil", ErrCore)
	}

	unsub, err := raw.SubscribePropertiesChanged(ctx, func(raw iwdbus.KnownNetworkPropertiesChanged) {
		changed := make(map[string]any, len(raw.Changed))
		for key, v := range raw.Changed {
			changed[key] = v.Value()
		}
		// Copy invalidated to avoid aliasing/mutation across layers.
		var invalidated []string
		if raw.Invalidated != nil {
			invalidated = slices.Clone(raw.Invalidated)
		}

		fn(KnownNetworkPropertiesChanged{
			Changed:     changed,
			Invalidated: invalidated,
		})
	})
	if err != nil {
		return nil, WrapKnownNetworkUnavailable(op, "failed to call iwd KnownNetwork subscribe method", err)
	}
	return UnsubscribeFunc(unsub), nil
}

// SubscribeAutoConnectChanged registers fn for normalized auto-connect events.
func (k *KnownNetwork) SubscribeAutoConnectChanged(ctx context.Context, fn func(bool)) (UnsubscribeFunc, error) {
	const op = "KnownNetwork.SubscribeAutoConnectChanged"

	raw, err := k.rawKnownNetwork(op)
	if err != nil {
		return nil, err
	}
	if fn == nil {
		return nil, WrapInvalidArgument(ResourceKnownNetwork, op, "callback cannot be nil", ErrCore)
	}

	unsub, err := raw.SubscribeAutoConnectChanged(ctx, fn)
	if err != nil {
		return nil, WrapKnownNetworkUnavailable(op, "failed to call iwd KnownNetwork subscribe method", err)
	}
	return UnsubscribeFunc(unsub), nil
}

// SubscribeHiddenChanged registers fn for changes to the Hidden property.
func (k *KnownNetwork) SubscribeHiddenChanged(ctx context.Context, fn func(bool)) (UnsubscribeFunc, error) {
	const op = "KnownNetwork.SubscribeHiddenChanged"

	raw, err := k.rawKnownNetwork(op)
	if err != nil {
		return nil, err
	}
	if fn == nil {
		return nil, WrapInvalidArgument(ResourceKnownNetwork, op, "callback cannot be nil", ErrCore)
	}

	unsub, err := raw.SubscribeHiddenChanged(ctx, fn)
	if err != nil {
		return nil, WrapKnownNetworkUnavailable(op, "failed to call iwd KnownNetwork subscribe method", err)
	}
	return UnsubscribeFunc(unsub), nil
}

// SubscribeLastConnectedTimeChanged registers fn for changes to LastConnectedTime. iwd updates it on
// each successful connection, so this fires once per connect.
func (k *KnownNetwork) SubscribeLastConnectedTimeChanged(ctx context.Context, fn func(*string)) (UnsubscribeFunc, error) {
	const op = "KnownNetwork.SubscribeLastConnectedTimeChanged"

	raw, err := k.rawKnownNetwork(op)
	if err != nil {
		return nil, err
	}
	if fn == nil {
		return nil, WrapInvalidArgument(ResourceKnownNetwork, op, "callback cannot be nil", ErrCore)
	}

	unsub, err := raw.SubscribeLastConnectedTimeChanged(ctx, fn)
	if err != nil {
		return nil, WrapKnownNetworkUnavailable(op, "failed to call iwd KnownNetwork subscribe method", err)
	}
	return UnsubscribeFunc(unsub), nil
}
