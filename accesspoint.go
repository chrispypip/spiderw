package spiderw

import (
	"context"

	"github.com/chrispypip/spiderw/internal/core"
	"github.com/chrispypip/spiderw/internal/logging"
)

// AccessPointProperties is a snapshot of an access point's state. Started is
// always present; the rest are absent (nil, or false for Scanning) while the AP
// is not running.
type AccessPointProperties struct {
	// Started reports whether the access point is running.
	Started bool

	// Scanning reports whether the access point is scanning. iwd only exposes this
	// while the AP is started, so it reads false when the AP is not running.
	Scanning bool

	// SSID is the hosted network's SSID (iwd's "Name" property), or nil when the
	// AP is not running.
	SSID *string

	// Frequency is the operating frequency in MHz, or nil when not running.
	Frequency *uint32

	// PairwiseCiphers are the negotiated unicast ciphers (e.g. "CCMP"), or nil.
	PairwiseCiphers []string

	// GroupCipher is the broadcast/multicast cipher, or nil.
	GroupCipher *string
}

// AccessPointOrderedNetwork is one network an access point heard while scanning.
type AccessPointOrderedNetwork struct {
	// Name is the scanned network's SSID.
	Name string

	// SignalStrength is the signal strength in dBm (e.g. -60.5). iwd reports it in
	// units of 100 * dBm; spiderw exposes it as dBm here.
	SignalStrength float64

	// Type is the network security type.
	Type NetworkType
}

// AccessPointPropertiesChanged is a normalized access-point property-change event.
type AccessPointPropertiesChanged struct {
	// Changed holds the changed property values keyed by name.
	Changed map[string]any

	// Invalidated names properties whose values should be re-read if needed.
	Invalidated []string
}

// AccessPoint is a device running in AP mode, hosting a network. Obtain one with
// Client.AccessPoint or Client.AllAccessPoints.
type AccessPoint struct {
	core core.AccessPointIface
	path string
	name string
}

func newAccessPoint(c core.AccessPointIface, path, name string) *AccessPoint {
	if c == nil {
		return nil
	}
	return &AccessPoint{core: c, path: path, name: name}
}

// wrapAccessPoint adapts clientObject's (core, path) construction; the Client
// fills in the device name afterward.
func wrapAccessPoint(c core.AccessPointIface, path string) *AccessPoint {
	return newAccessPoint(c, path, "")
}

// Path returns the access point's D-Bus object path (a device path).
func (a *AccessPoint) Path() string {
	if a == nil {
		return ""
	}
	return a.path
}

// Name returns the underlying device's name (e.g. "wlan1"). This is the device
// identity, not the hosted network's SSID — see SSID.
func (a *AccessPoint) Name() string {
	if a == nil {
		return ""
	}
	return a.name
}

func (a *AccessPoint) coreAccessPoint(ctx context.Context, op string) (core.AccessPointIface, error) {
	if a == nil || a.core == nil {
		logging.FromContext(ctx).Error(ctx, "access point wrapper uninitialized", "op", op)
		return nil, wrapPublicError(op, ErrInternal)
	}
	return a.core, nil
}

// Started reports whether the access point is running.
func (a *AccessPoint) Started(ctx context.Context) (bool, error) {
	return delegate(ctx, "AccessPoint.Started", a.coreAccessPoint, func(ctx context.Context, c core.AccessPointIface) (bool, error) {
		return c.Started(ctx)
	})
}

// Scanning reports whether the access point is scanning.
func (a *AccessPoint) Scanning(ctx context.Context) (bool, error) {
	return delegate(ctx, "AccessPoint.Scanning", a.coreAccessPoint, func(ctx context.Context, c core.AccessPointIface) (bool, error) {
		return c.Scanning(ctx)
	})
}

// SSID returns the hosted network's SSID (iwd's "Name" property), or nil when the
// access point is not running.
func (a *AccessPoint) SSID(ctx context.Context) (*string, error) {
	return delegate(ctx, "AccessPoint.SSID", a.coreAccessPoint, func(ctx context.Context, c core.AccessPointIface) (*string, error) {
		return c.Name(ctx)
	})
}

// Frequency returns the operating frequency in MHz, or nil when not running.
func (a *AccessPoint) Frequency(ctx context.Context) (*uint32, error) {
	return delegate(ctx, "AccessPoint.Frequency", a.coreAccessPoint, func(ctx context.Context, c core.AccessPointIface) (*uint32, error) {
		return c.Frequency(ctx)
	})
}

// PairwiseCiphers returns the negotiated unicast ciphers, or nil when not running.
func (a *AccessPoint) PairwiseCiphers(ctx context.Context) ([]string, error) {
	return delegate(ctx, "AccessPoint.PairwiseCiphers", a.coreAccessPoint, func(ctx context.Context, c core.AccessPointIface) ([]string, error) {
		return c.PairwiseCiphers(ctx)
	})
}

// GroupCipher returns the broadcast/multicast cipher, or nil when not running.
func (a *AccessPoint) GroupCipher(ctx context.Context) (*string, error) {
	return delegate(ctx, "AccessPoint.GroupCipher", a.coreAccessPoint, func(ctx context.Context, c core.AccessPointIface) (*string, error) {
		return c.GroupCipher(ctx)
	})
}

// Properties returns a snapshot of every access-point property in one call.
func (a *AccessPoint) Properties(ctx context.Context) (*AccessPointProperties, error) {
	return delegate(ctx, "AccessPoint.Properties", a.coreAccessPoint, func(ctx context.Context, c core.AccessPointIface) (*AccessPointProperties, error) {
		p, err := c.Properties(ctx)
		if err != nil {
			return nil, err
		}
		return &AccessPointProperties{
			Started:         p.Started,
			Scanning:        p.Scanning,
			SSID:            p.Name,
			Frequency:       p.Frequency,
			PairwiseCiphers: p.PairwiseCiphers,
			GroupCipher:     p.GroupCipher,
		}, nil
	})
}

// Start starts a PSK-secured access point advertising ssid with passphrase psk
// (SSID 1-32 bytes, passphrase 8-63 characters). It blocks until iwd reports the
// outcome. iwd returns an error matching ErrAlreadyExists when an AP is already
// running.
func (a *AccessPoint) Start(ctx context.Context, ssid, psk string) error {
	return do(ctx, "AccessPoint.Start", a.coreAccessPoint, func(ctx context.Context, c core.AccessPointIface) error {
		return c.Start(ctx, ssid, psk)
	})
}

// StartProfile starts an access point from the stored profile named ssid, which
// may configure security modes beyond PSK. iwd returns an error matching
// ErrNotFound when no such profile exists.
func (a *AccessPoint) StartProfile(ctx context.Context, ssid string) error {
	return do(ctx, "AccessPoint.StartProfile", a.coreAccessPoint, func(ctx context.Context, c core.AccessPointIface) error {
		return c.StartProfile(ctx, ssid)
	})
}

// Stop stops the running access point.
func (a *AccessPoint) Stop(ctx context.Context) error {
	return do(ctx, "AccessPoint.Stop", a.coreAccessPoint, func(ctx context.Context, c core.AccessPointIface) error {
		return c.Stop(ctx)
	})
}

// Scan schedules an access-point scan. It is asynchronous; the Scanning property
// tracks progress.
func (a *AccessPoint) Scan(ctx context.Context) error {
	return do(ctx, "AccessPoint.Scan", a.coreAccessPoint, func(ctx context.Context, c core.AccessPointIface) error {
		return c.Scan(ctx)
	})
}

// OrderedNetworks returns the networks from the most recent access-point scan,
// ordered by signal strength.
func (a *AccessPoint) OrderedNetworks(ctx context.Context) ([]AccessPointOrderedNetwork, error) {
	return delegate(ctx, "AccessPoint.OrderedNetworks", a.coreAccessPoint, func(ctx context.Context, c core.AccessPointIface) ([]AccessPointOrderedNetwork, error) {
		raw, err := c.OrderedNetworks(ctx)
		if err != nil {
			return nil, err
		}
		out := make([]AccessPointOrderedNetwork, 0, len(raw))
		for _, n := range raw {
			// Scan results routinely include neighbor networks whose security iwd
			// does not classify (an empty or unrecognized Security field), so an
			// unknown type collapses to NetworkTypeUnknown rather than failing the
			// whole list. This differs from a specific Network, whose Type is strict.
			netType, err := convertNetworkType(n.Type)
			if err != nil {
				netType = NetworkTypeUnknown
			}
			out = append(out, AccessPointOrderedNetwork{
				Name:           n.Name,
				SignalStrength: float64(n.SignalStrength) / 100,
				Type:           netType,
			})
		}
		return out, nil
	})
}

// SubscribePropertiesChanged registers fn for access-point property changes.
func (a *AccessPoint) SubscribePropertiesChanged(ctx context.Context, fn func(AccessPointPropertiesChanged)) (UnsubscribeFunc, error) {
	const op = "AccessPoint.SubscribePropertiesChanged"

	coreAP, err := a.coreAccessPoint(ctx, op)
	if err != nil {
		return nil, err
	}
	if fn == nil {
		return nil, &Error{Kind: KindInvalidArgument, Resource: ResourceAccessPoint, Op: op, Details: "callback cannot be nil", Err: ErrInvalidArgument}
	}

	unsubscribe, err := coreAP.SubscribePropertiesChanged(ctx, func(ev core.AccessPointPropertiesChanged) {
		fn(AccessPointPropertiesChanged{Changed: ev.Changed, Invalidated: ev.Invalidated})
	})
	if err != nil {
		return nil, wrapPublicError(op, err)
	}
	return UnsubscribeFunc(unsubscribe), nil
}

// SubscribeStartedChanged registers fn for changes to the Started state.
func (a *AccessPoint) SubscribeStartedChanged(ctx context.Context, fn func(bool)) (UnsubscribeFunc, error) {
	const op = "AccessPoint.SubscribeStartedChanged"

	coreAP, err := a.coreAccessPoint(ctx, op)
	if err != nil {
		return nil, err
	}
	if fn == nil {
		return nil, &Error{Kind: KindInvalidArgument, Resource: ResourceAccessPoint, Op: op, Details: "callback cannot be nil", Err: ErrInvalidArgument}
	}

	unsubscribe, err := coreAP.SubscribeStartedChanged(ctx, fn)
	if err != nil {
		return nil, wrapPublicError(op, err)
	}
	return UnsubscribeFunc(unsubscribe), nil
}

// SubscribeScanningChanged registers fn for changes to the Scanning state.
func (a *AccessPoint) SubscribeScanningChanged(ctx context.Context, fn func(bool)) (UnsubscribeFunc, error) {
	const op = "AccessPoint.SubscribeScanningChanged"

	coreAP, err := a.coreAccessPoint(ctx, op)
	if err != nil {
		return nil, err
	}
	if fn == nil {
		return nil, &Error{Kind: KindInvalidArgument, Resource: ResourceAccessPoint, Op: op, Details: "callback cannot be nil", Err: ErrInvalidArgument}
	}

	unsubscribe, err := coreAP.SubscribeScanningChanged(ctx, fn)
	if err != nil {
		return nil, wrapPublicError(op, err)
	}
	return UnsubscribeFunc(unsubscribe), nil
}
