package core

import (
	"context"
	"fmt"
	"slices"

	"github.com/chrispypip/spiderw/internal/iwdbus"
)

// accessPointRaw is the iwdbus backend the core AccessPoint wraps. It mirrors the
// iwdbus.AccessPoint methods so the concrete type satisfies it directly and tests
// can substitute a fake.
type accessPointRaw interface {
	GetStarted(ctx context.Context) (bool, error)
	GetScanning(ctx context.Context) (bool, error)
	GetName(ctx context.Context) (*string, error)
	GetFrequency(ctx context.Context) (*uint32, error)
	GetPairwiseCiphers(ctx context.Context) ([]string, error)
	GetGroupCipher(ctx context.Context) (*string, error)
	GetProperties(ctx context.Context) (*iwdbus.AccessPointProperties, error)
	Start(ctx context.Context, ssid, psk string) error
	StartProfile(ctx context.Context, ssid string) error
	Stop(ctx context.Context) error
	Scan(ctx context.Context) error
	GetOrderedNetworks(ctx context.Context) ([]iwdbus.AccessPointOrderedNetwork, error)
	SubscribePropertiesChanged(ctx context.Context, fn func(iwdbus.AccessPointPropertiesChanged)) (iwdbus.UnsubscribeFunc, error)
	SubscribeStartedChanged(ctx context.Context, fn func(bool)) (iwdbus.UnsubscribeFunc, error)
	SubscribeScanningChanged(ctx context.Context, fn func(bool)) (iwdbus.UnsubscribeFunc, error)
}

// AccessPointProperties holds the normalized access-point properties. Only
// Started is always present; the remaining fields are optional and left at their
// zero value (nil, or false for Scanning) when iwd omits them - which it does
// while the AP is not running.
type AccessPointProperties struct {
	Started         bool
	Scanning        bool
	Name            *string
	Frequency       *uint32
	PairwiseCiphers []string
	GroupCipher     *string
}

// AccessPointOrderedNetwork is one entry from OrderedNetworks: a network the AP
// heard while scanning.
type AccessPointOrderedNetwork struct {
	// Name is the network SSID.
	Name string

	// SignalStrength is in units of 100 * dBm (iwd's native unit).
	SignalStrength int16

	// Type is the network security type.
	Type NetworkType
}

// AccessPointPropertiesChanged describes normalized access-point property-change
// data.
type AccessPointPropertiesChanged struct {
	// Changed contains normalized property values keyed by property name.
	Changed map[string]any

	// Invalidated contains property names whose values should be re-read if needed.
	Invalidated []string
}

// AccessPointIface is the core AccessPoint surface the connect and public layers
// depend on, so they can substitute the concrete wrapper in tests.
type AccessPointIface interface {
	Started(ctx context.Context) (bool, error)
	Scanning(ctx context.Context) (bool, error)
	Name(ctx context.Context) (*string, error)
	Frequency(ctx context.Context) (*uint32, error)
	PairwiseCiphers(ctx context.Context) ([]string, error)
	GroupCipher(ctx context.Context) (*string, error)
	Properties(ctx context.Context) (*AccessPointProperties, error)
	Start(ctx context.Context, ssid, psk string) error
	StartProfile(ctx context.Context, ssid string) error
	Stop(ctx context.Context) error
	Scan(ctx context.Context) error
	OrderedNetworks(ctx context.Context) ([]AccessPointOrderedNetwork, error)
	SubscribePropertiesChanged(ctx context.Context, fn func(AccessPointPropertiesChanged)) (UnsubscribeFunc, error)
	SubscribeStartedChanged(ctx context.Context, fn func(bool)) (UnsubscribeFunc, error)
	SubscribeScanningChanged(ctx context.Context, fn func(bool)) (UnsubscribeFunc, error)
}

// AccessPoint is the core-layer wrapper over iwd's AccessPoint interface (a device
// running in AP mode). It validates Start inputs up front and classifies errors
// under ResourceAccessPoint.
type AccessPoint struct {
	raw accessPointRaw
}

// NewAccessPoint wraps raw. It returns nil when raw is nil so the connect layer
// can treat a missing AccessPoint interface as unavailable.
func NewAccessPoint(raw accessPointRaw) *AccessPoint {
	if raw == nil {
		return nil
	}
	return &AccessPoint{raw: raw}
}

func (a *AccessPoint) rawAccessPoint(op string) (accessPointRaw, error) {
	if a == nil || a.raw == nil {
		return nil, WrapInvalidState(ResourceAccessPoint, op, "access point wrapper was nil", ErrAccessPointNotInitialized)
	}
	return a.raw, nil
}

// Started reports whether the access point is currently running.
func (a *AccessPoint) Started(ctx context.Context) (bool, error) {
	const op = "AccessPoint.Started"

	raw, err := a.rawAccessPoint(op)
	if err != nil {
		return false, err
	}

	started, err := raw.GetStarted(ctx)
	if err != nil {
		return false, WrapAccessPointUnavailable(op, "failed querying iwd AccessPoint Started", err)
	}
	return started, nil
}

// Scanning reports whether the access point is currently scanning.
func (a *AccessPoint) Scanning(ctx context.Context) (bool, error) {
	const op = "AccessPoint.Scanning"

	raw, err := a.rawAccessPoint(op)
	if err != nil {
		return false, err
	}

	scanning, err := raw.GetScanning(ctx)
	if err != nil {
		return false, WrapAccessPointUnavailable(op, "failed querying iwd AccessPoint Scanning", err)
	}
	return scanning, nil
}

// Name returns the running access point's SSID, or nil when it is not running.
func (a *AccessPoint) Name(ctx context.Context) (*string, error) {
	const op = "AccessPoint.Name"

	raw, err := a.rawAccessPoint(op)
	if err != nil {
		return nil, err
	}

	name, err := raw.GetName(ctx)
	if err != nil {
		return nil, WrapAccessPointUnavailable(op, "failed querying iwd AccessPoint Name", err)
	}
	return name, nil
}

// Frequency returns the running access point's operating frequency in MHz, or nil
// when it is not running.
func (a *AccessPoint) Frequency(ctx context.Context) (*uint32, error) {
	const op = "AccessPoint.Frequency"

	raw, err := a.rawAccessPoint(op)
	if err != nil {
		return nil, err
	}

	freq, err := raw.GetFrequency(ctx)
	if err != nil {
		return nil, WrapAccessPointUnavailable(op, "failed querying iwd AccessPoint Frequency", err)
	}
	return freq, nil
}

// PairwiseCiphers returns the access point's pairwise (unicast) ciphers, or nil
// when it is not running.
func (a *AccessPoint) PairwiseCiphers(ctx context.Context) ([]string, error) {
	const op = "AccessPoint.PairwiseCiphers"

	raw, err := a.rawAccessPoint(op)
	if err != nil {
		return nil, err
	}

	ciphers, err := raw.GetPairwiseCiphers(ctx)
	if err != nil {
		return nil, WrapAccessPointUnavailable(op, "failed querying iwd AccessPoint PairwiseCiphers", err)
	}
	return ciphers, nil
}

// GroupCipher returns the access point's group (broadcast/multicast) cipher, or
// nil when it is not running.
func (a *AccessPoint) GroupCipher(ctx context.Context) (*string, error) {
	const op = "AccessPoint.GroupCipher"

	raw, err := a.rawAccessPoint(op)
	if err != nil {
		return nil, err
	}

	group, err := raw.GetGroupCipher(ctx)
	if err != nil {
		return nil, WrapAccessPointUnavailable(op, "failed querying iwd AccessPoint GroupCipher", err)
	}
	return group, nil
}

// Properties reads every access-point property in one call.
func (a *AccessPoint) Properties(ctx context.Context) (*AccessPointProperties, error) {
	const op = "AccessPoint.Properties"

	raw, err := a.rawAccessPoint(op)
	if err != nil {
		return nil, err
	}

	props, err := raw.GetProperties(ctx)
	if err != nil {
		return nil, WrapAccessPointUnavailable(op, "failed querying iwd AccessPoint properties", err)
	}

	return &AccessPointProperties{
		Started:         props.Started,
		Scanning:        props.Scanning,
		Name:            props.Name,
		Frequency:       props.Frequency,
		PairwiseCiphers: props.PairwiseCiphers,
		GroupCipher:     props.GroupCipher,
	}, nil
}

// Start starts a PSK-secured access point advertising ssid with passphrase psk.
// The SSID (1-32 bytes) and passphrase (8-63 characters) are validated up front,
// so a malformed request fails locally without a D-Bus round-trip.
func (a *AccessPoint) Start(ctx context.Context, ssid, psk string) error {
	const op = "AccessPoint.Start"

	raw, err := a.rawAccessPoint(op)
	if err != nil {
		return err
	}

	if err := validateAccessPointSSID(op, ssid); err != nil {
		return err
	}
	if l := len(psk); l < 8 || l > 63 {
		return WrapInvalidArgument(ResourceAccessPoint, op, fmt.Sprintf("passphrase must be 8-63 characters, got %d", l), ErrCore)
	}

	if err := raw.Start(ctx, ssid, psk); err != nil {
		return WrapAccessPointUnavailable(op, "failed starting iwd AccessPoint", err)
	}
	return nil
}

// StartProfile starts an access point from the stored profile named ssid.
func (a *AccessPoint) StartProfile(ctx context.Context, ssid string) error {
	const op = "AccessPoint.StartProfile"

	raw, err := a.rawAccessPoint(op)
	if err != nil {
		return err
	}

	if err := validateAccessPointSSID(op, ssid); err != nil {
		return err
	}

	if err := raw.StartProfile(ctx, ssid); err != nil {
		return WrapAccessPointUnavailable(op, "failed starting iwd AccessPoint profile", err)
	}
	return nil
}

// Stop stops the running access point.
func (a *AccessPoint) Stop(ctx context.Context) error {
	const op = "AccessPoint.Stop"

	raw, err := a.rawAccessPoint(op)
	if err != nil {
		return err
	}

	if err := raw.Stop(ctx); err != nil {
		return WrapAccessPointUnavailable(op, "failed stopping iwd AccessPoint", err)
	}
	return nil
}

// Scan schedules an access-point scan. It is asynchronous; the Scanning property
// tracks progress.
func (a *AccessPoint) Scan(ctx context.Context) error {
	const op = "AccessPoint.Scan"

	raw, err := a.rawAccessPoint(op)
	if err != nil {
		return err
	}

	if err := raw.Scan(ctx); err != nil {
		return WrapAccessPointUnavailable(op, "failed scheduling iwd AccessPoint scan", err)
	}
	return nil
}

// OrderedNetworks returns the networks from the most recent access-point scan,
// ordered by signal strength.
func (a *AccessPoint) OrderedNetworks(ctx context.Context) ([]AccessPointOrderedNetwork, error) {
	const op = "AccessPoint.OrderedNetworks"

	raw, err := a.rawAccessPoint(op)
	if err != nil {
		return nil, err
	}

	networks, err := raw.GetOrderedNetworks(ctx)
	if err != nil {
		return nil, WrapAccessPointUnavailable(op, "failed querying iwd AccessPoint ordered networks", err)
	}

	out := make([]AccessPointOrderedNetwork, 0, len(networks))
	for _, n := range networks {
		out = append(out, AccessPointOrderedNetwork{
			Name:           n.Name,
			SignalStrength: n.SignalStrength,
			Type:           n.Type,
		})
	}
	return out, nil
}

// SubscribePropertiesChanged registers fn for normalized property-change events.
func (a *AccessPoint) SubscribePropertiesChanged(ctx context.Context, fn func(AccessPointPropertiesChanged)) (UnsubscribeFunc, error) {
	const op = "AccessPoint.SubscribePropertiesChanged"

	raw, err := a.rawAccessPoint(op)
	if err != nil {
		return nil, err
	}
	if fn == nil {
		return nil, WrapInvalidArgument(ResourceAccessPoint, op, "callback cannot be nil", ErrCore)
	}

	unsub, err := raw.SubscribePropertiesChanged(ctx, func(rawEv iwdbus.AccessPointPropertiesChanged) {
		changed := make(map[string]any, len(rawEv.Changed))
		for k, v := range rawEv.Changed {
			changed[k] = v.Value()
		}
		var invalidated []string
		if rawEv.Invalidated != nil {
			invalidated = slices.Clone(rawEv.Invalidated)
		}

		fn(AccessPointPropertiesChanged{
			Changed:     changed,
			Invalidated: invalidated,
		})
	})
	if err != nil {
		return nil, WrapAccessPointUnavailable(op, "failed to call iwd AccessPoint subscribe method", err)
	}
	return UnsubscribeFunc(unsub), nil
}

// SubscribeStartedChanged registers fn for Started-state changes.
func (a *AccessPoint) SubscribeStartedChanged(ctx context.Context, fn func(bool)) (UnsubscribeFunc, error) {
	const op = "AccessPoint.SubscribeStartedChanged"

	raw, err := a.rawAccessPoint(op)
	if err != nil {
		return nil, err
	}
	if fn == nil {
		return nil, WrapInvalidArgument(ResourceAccessPoint, op, "callback cannot be nil", ErrCore)
	}

	unsub, err := raw.SubscribeStartedChanged(ctx, fn)
	if err != nil {
		return nil, WrapAccessPointUnavailable(op, "failed to call iwd AccessPoint subscribe method", err)
	}
	return UnsubscribeFunc(unsub), nil
}

// SubscribeScanningChanged registers fn for scanning-state changes.
func (a *AccessPoint) SubscribeScanningChanged(ctx context.Context, fn func(bool)) (UnsubscribeFunc, error) {
	const op = "AccessPoint.SubscribeScanningChanged"

	raw, err := a.rawAccessPoint(op)
	if err != nil {
		return nil, err
	}
	if fn == nil {
		return nil, WrapInvalidArgument(ResourceAccessPoint, op, "callback cannot be nil", ErrCore)
	}

	unsub, err := raw.SubscribeScanningChanged(ctx, fn)
	if err != nil {
		return nil, WrapAccessPointUnavailable(op, "failed to call iwd AccessPoint subscribe method", err)
	}
	return UnsubscribeFunc(unsub), nil
}

// validateAccessPointSSID checks that ssid is a valid 802.11 SSID (1-32 bytes).
func validateAccessPointSSID(op, ssid string) error {
	if l := len(ssid); l == 0 || l > 32 {
		return WrapInvalidArgument(ResourceAccessPoint, op, fmt.Sprintf("SSID must be 1-32 bytes, got %d", l), ErrCore)
	}
	return nil
}
