package spiderw

import (
	"context"
	"fmt"
	"slices"

	"github.com/chrispypip/spiderw/internal/core"
	"github.com/chrispypip/spiderw/internal/iwdvalue"
	"github.com/chrispypip/spiderw/internal/logging"
)

// UnsubscribeFunc unregisters a previously registered subscription callback.
//
// It is safe for implementations to make repeated calls no-ops.
type UnsubscribeFunc func() error

// Unsubscribe unregisters the subscription callback.
//
// Calling Unsubscribe on a nil UnsubscribeFunc is a no-op.
func (u UnsubscribeFunc) Unsubscribe() error {
	if u == nil {
		return nil
	}
	return u()
}

// AdapterMode identifies an iwd adapter operating mode exposed by spiderw.
type AdapterMode string

// Adapter mode constants identify the supported iwd adapter modes.
// AdapterModeUnknown is reserved for invalid or unrecognized values.
const (
	// AdapterModeUnknown represents an invalid or unrecognized adapter mode.
	AdapterModeUnknown AdapterMode = AdapterMode(iwdvalue.AdapterModeUnknown)

	// AdapterModeStation is the iwd station adapter mode.
	AdapterModeStation AdapterMode = AdapterMode(iwdvalue.AdapterModeStation)

	// AdapterModeAP is the iwd access point adapter mode.
	AdapterModeAP AdapterMode = AdapterMode(iwdvalue.AdapterModeAP)

	// AdapterModeAdHoc is the iwd ad-hoc adapter mode.
	AdapterModeAdHoc AdapterMode = AdapterMode(iwdvalue.AdapterModeAdHoc)
)

// String returns the canonical iwd string for the adapter mode.
func (m AdapterMode) String() string {
	return iwdvalue.AdapterMode(m).String()
}

// AdapterPropertiesChanged describes adapter properties reported by a D-Bus
// PropertiesChanged signal. Changed contains the new values by property name;
// Invalidated contains property names whose values should be re-read if needed.
type AdapterPropertiesChanged struct {
	// Changed contains new property values keyed by property name.
	Changed map[string]any

	// Invalidated contains property names whose values should be re-read if needed.
	Invalidated []string
}

// AdapterProperties is a snapshot of all adapter properties read in a single
// D-Bus call. Model and Vendor are nil when iwd does not report them.
type AdapterProperties struct {
	// Powered reports whether the adapter is currently powered.
	Powered bool

	// Name is the adapter's human-friendly Name property.
	Name string

	// Model is the adapter's Model property, or nil when not reported.
	Model *string

	// Vendor is the adapter's Vendor property, or nil when not reported.
	Vendor *string

	// SupportedModes lists the adapter's supported operating modes.
	SupportedModes []AdapterMode
}

// Adapter provides high-level operations for a specific iwd adapter object.
type Adapter struct {
	core core.AdapterIface
	path string
}

func newAdapter(c core.AdapterIface, path string) *Adapter {
	if c == nil {
		return nil
	}
	return &Adapter{core: c, path: path}
}

// Path returns the D-Bus object path the adapter was constructed from.
//
// Path is static adapter identity, not an iwd property: it requires no D-Bus
// round-trip and never fails. Path returns "" for a nil receiver.
func (a *Adapter) Path() string {
	if a == nil {
		return ""
	}
	return a.path
}

func (a *Adapter) coreAdapter(ctx context.Context, op string) (core.AdapterIface, error) {
	if a == nil || a.core == nil {
		logging.FromContext(ctx).Error(ctx, "adapter wrapper uninitialized", "op", op)
		return nil, wrapPublicError(op, ErrInternal)
	}
	return a.core, nil
}

// Powered reports whether the adapter is currently powered.
func (a *Adapter) Powered(ctx context.Context) (bool, error) {
	return delegate(ctx, "Adapter.Powered", a.coreAdapter, func(ctx context.Context, c core.AdapterIface) (bool, error) {
		return c.Powered(ctx)
	})
}

// SetPowered changes whether the adapter is powered.
func (a *Adapter) SetPowered(ctx context.Context, powered bool) error {
	return do(ctx, "Adapter.SetPowered", a.coreAdapter, func(ctx context.Context, c core.AdapterIface) error {
		return c.SetPowered(ctx, powered)
	})
}

// Name returns the adapter name.
func (a *Adapter) Name(ctx context.Context) (string, error) {
	return delegate(ctx, "Adapter.Name", a.coreAdapter, func(ctx context.Context, c core.AdapterIface) (string, error) {
		return c.Name(ctx)
	})
}

// Model returns the adapter model, or nil when iwd does not report one.
func (a *Adapter) Model(ctx context.Context) (*string, error) {
	return delegate(ctx, "Adapter.Model", a.coreAdapter, func(ctx context.Context, c core.AdapterIface) (*string, error) {
		return c.Model(ctx)
	})
}

// Vendor returns the adapter vendor, or nil when iwd does not report one.
func (a *Adapter) Vendor(ctx context.Context) (*string, error) {
	return delegate(ctx, "Adapter.Vendor", a.coreAdapter, func(ctx context.Context, c core.AdapterIface) (*string, error) {
		return c.Vendor(ctx)
	})
}

// Properties reads every adapter property in a single D-Bus call
// (Properties.GetAll) instead of one call per property. Prefer it when you need
// several properties at once, such as building an overview of an adapter.
func (a *Adapter) Properties(ctx context.Context) (*AdapterProperties, error) {
	return delegate(ctx, "Adapter.Properties", a.coreAdapter, func(ctx context.Context, c core.AdapterIface) (*AdapterProperties, error) {
		cp, err := c.Properties(ctx)
		if err != nil {
			return nil, err
		}

		modes, err := convertSupportedModes(cp.SupportedModes)
		if err != nil {
			return nil, err
		}

		return &AdapterProperties{
			Powered:        cp.Powered,
			Name:           cp.Name,
			Model:          cp.Model,
			Vendor:         cp.Vendor,
			SupportedModes: modes,
		}, nil
	})
}

// SupportedModes returns the adapter modes currently reported by iwd.
func (a *Adapter) SupportedModes(ctx context.Context) ([]AdapterMode, error) {
	return delegate(ctx, "Adapter.SupportedModes", a.coreAdapter, func(ctx context.Context, c core.AdapterIface) ([]AdapterMode, error) {
		modes, err := c.SupportedModes(ctx)
		if err != nil {
			return nil, err
		}
		return convertSupportedModes(modes)
	})
}

// SupportsMode reports whether the adapter supports the provided mode.
func (a *Adapter) SupportsMode(ctx context.Context, mode AdapterMode) (bool, error) {
	return delegate(ctx, "Adapter.SupportsMode", a.coreAdapter, func(ctx context.Context, c core.AdapterIface) (bool, error) {
		cm, err := convertSupportedModePublicToCore(mode)
		if err != nil {
			return false, err
		}
		return c.SupportsMode(ctx, cm)
	})
}

// SupportsStation reports whether the adapter supports station mode.
func (a *Adapter) SupportsStation(ctx context.Context) (bool, error) {
	return delegate(ctx, "Adapter.SupportsStation", a.coreAdapter, func(ctx context.Context, c core.AdapterIface) (bool, error) {
		return c.SupportsStation(ctx)
	})
}

// SupportsAP reports whether the adapter supports access point mode.
func (a *Adapter) SupportsAP(ctx context.Context) (bool, error) {
	return delegate(ctx, "Adapter.SupportsAP", a.coreAdapter, func(ctx context.Context, c core.AdapterIface) (bool, error) {
		return c.SupportsAP(ctx)
	})
}

// SupportsAdHoc reports whether the adapter supports ad-hoc mode.
func (a *Adapter) SupportsAdHoc(ctx context.Context) (bool, error) {
	return delegate(ctx, "Adapter.SupportsAdHoc", a.coreAdapter, func(ctx context.Context, c core.AdapterIface) (bool, error) {
		return c.SupportsAdHoc(ctx)
	})
}

// SubscribePropertiesChanged registers fn for adapter property-change signals and
// returns a handle that unregisters the callback.
func (a *Adapter) SubscribePropertiesChanged(ctx context.Context, fn func(AdapterPropertiesChanged)) (UnsubscribeFunc, error) {
	const op = "Adapter.SubscribePropertiesChanged"

	coreAdapter, err := a.coreAdapter(ctx, op)
	if err != nil {
		return nil, err
	}
	if fn == nil {
		return nil, &Error{Kind: KindInvalidArgument, Resource: ResourceAdapter, Op: op, Details: "callback cannot be nil", Err: ErrInvalidArgument}
	}

	unsubscribe, err := coreAdapter.SubscribePropertiesChanged(ctx, func(core core.AdapterPropertiesChanged) {
		changed := make(map[string]any, len(core.Changed))
		for k, v := range core.Changed {
			changed[k] = v
		}

		// Copy invalidated to avoid aliasing/mutation across layers.
		var invalidated []string
		if core.Invalidated != nil {
			invalidated = slices.Clone(core.Invalidated)
		}

		fn(AdapterPropertiesChanged{
			Changed:     changed,
			Invalidated: invalidated,
		})
	})
	if err != nil {
		return nil, wrapPublicError(op, err)
	}
	return UnsubscribeFunc(unsubscribe), nil
}

// SubscribePoweredChanged registers fn for adapter powered-state changes and
// returns a handle that unregisters the callback.
func (a *Adapter) SubscribePoweredChanged(ctx context.Context, fn func(bool)) (UnsubscribeFunc, error) {
	const op = "Adapter.SubscribePoweredChanged"

	coreAdapter, err := a.coreAdapter(ctx, op)
	if err != nil {
		return nil, err
	}
	if fn == nil {
		return nil, &Error{Kind: KindInvalidArgument, Resource: ResourceAdapter, Op: op, Details: "callback cannot be nil", Err: ErrInvalidArgument}
	}

	unsubscribe, err := coreAdapter.SubscribePoweredChanged(ctx, fn)
	if err != nil {
		return nil, wrapPublicError(op, err)
	}
	return UnsubscribeFunc(unsubscribe), nil
}

// ParseAdapterMode converts a canonical iwd mode string to an AdapterMode.
func ParseAdapterMode(s string) (AdapterMode, error) {
	mode, ok := iwdvalue.ParseAdapterMode(s)
	if !ok {
		details := fmt.Sprintf("invalid adapter mode %q", s)
		return AdapterModeUnknown, &Error{Kind: KindInvalidArgument, Resource: ResourceAdapter, Op: "Adapter.ParseAdapterMode", Details: details, Err: ErrInvalidArgument}
	}
	return AdapterMode(mode), nil
}

func convertSupportedModeCoreToPublic(mode core.AdapterMode) (AdapterMode, error) {
	if !iwdvalue.ValidAdapterMode(mode) {
		details := fmt.Sprintf("invalid adapter mode %q", mode)
		return AdapterModeUnknown, &Error{Kind: KindInvalidArgument, Resource: ResourceAdapter, Op: "Adapter.convertSupportedMode", Details: details, Err: ErrInvalidArgument}
	}
	return AdapterMode(mode), nil
}

func convertSupportedModePublicToCore(mode AdapterMode) (core.AdapterMode, error) {
	coreMode := core.AdapterMode(mode)
	if !iwdvalue.ValidAdapterMode(coreMode) {
		details := fmt.Sprintf("invalid adapter mode %q", mode)
		return core.AdapterModeUnknown, &Error{Kind: KindInvalidArgument, Resource: ResourceAdapter, Op: "Adapter.convertSupportedMode", Details: details, Err: ErrInvalidArgument}
	}
	return coreMode, nil
}

func convertSupportedModes(modes []core.AdapterMode) ([]AdapterMode, error) {
	ret := make([]AdapterMode, 0, len(modes))
	for _, mode := range modes {
		cm, err := convertSupportedModeCoreToPublic(mode)
		if err != nil {
			return nil, err
		}
		ret = append(ret, cm)
	}
	return ret, nil
}
