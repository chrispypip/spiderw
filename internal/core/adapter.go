package core

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/chrispypip/spiderw/internal/iwdbus"
	"github.com/chrispypip/spiderw/internal/iwdvalue"
)

// AdapterMode identifies a normalized iwd adapter operating mode.
type AdapterMode = iwdvalue.AdapterMode

// Adapter mode constants identify normalized adapter modes.
// AdapterModeUnknown is reserved for invalid or unrecognized values.
const (
	// AdapterModeUnknown represents an invalid or unrecognized adapter mode.
	AdapterModeUnknown = iwdvalue.AdapterModeUnknown

	// AdapterModeStation is the iwd station adapter mode.
	AdapterModeStation = iwdvalue.AdapterModeStation

	// AdapterModeAP is the iwd access point adapter mode.
	AdapterModeAP = iwdvalue.AdapterModeAP

	// AdapterModeAdHoc is the iwd ad-hoc adapter mode.
	AdapterModeAdHoc = iwdvalue.AdapterModeAdHoc
)

// UnsubscribeFunc unregisters a core-layer subscription callback.
type UnsubscribeFunc func() error

// Unsubscribe unregisters the core subscription callback.
func (u UnsubscribeFunc) Unsubscribe() error {
	if u == nil {
		return nil
	}
	return u()
}

// AdapterPropertiesChanged describes normalized adapter property-change data.
type AdapterPropertiesChanged struct {
	// Changed contains normalized property values keyed by property name.
	Changed map[string]any

	// Invalidated contains property names whose values should be re-read if needed.
	Invalidated []string
}

type adapterRaw interface {
	GetPowered(ctx context.Context) (bool, error)
	SetPowered(ctx context.Context, powered bool) error
	GetName(ctx context.Context) (string, error)
	GetModel(ctx context.Context) (*string, error)
	GetVendor(ctx context.Context) (*string, error)
	GetSupportedModes(ctx context.Context) ([]iwdbus.AdapterMode, error)
	GetProperties(ctx context.Context) (*iwdbus.AdapterProperties, error)
	SupportsMode(ctx context.Context, mode iwdbus.AdapterMode) (bool, error)
	SupportsStation(ctx context.Context) (bool, error)
	SupportsAP(ctx context.Context) (bool, error)
	SupportsAdHoc(ctx context.Context) (bool, error)
	SubscribePropertiesChanged(ctx context.Context, fn func(iwdbus.AdapterPropertiesChanged)) (iwdbus.UnsubscribeFunc, error)
	SubscribePoweredChanged(ctx context.Context, fn func(bool)) (iwdbus.UnsubscribeFunc, error)
}

// AdapterIface defines the core adapter operations used by the public layer.
type AdapterIface interface {
	Powered(ctx context.Context) (bool, error)
	SetPowered(ctx context.Context, powered bool) error
	Name(ctx context.Context) (string, error)
	Model(ctx context.Context) (*string, error)
	Vendor(ctx context.Context) (*string, error)
	SupportedModes(ctx context.Context) ([]AdapterMode, error)
	Properties(ctx context.Context) (*AdapterProperties, error)
	SupportsMode(ctx context.Context, mode AdapterMode) (bool, error)
	SupportsStation(ctx context.Context) (bool, error)
	SupportsAP(ctx context.Context) (bool, error)
	SupportsAdHoc(ctx context.Context) (bool, error)
	SubscribePropertiesChanged(ctx context.Context, fn func(AdapterPropertiesChanged)) (UnsubscribeFunc, error)
	SubscribePoweredChanged(ctx context.Context, fn func(bool)) (UnsubscribeFunc, error)
}

// AdapterProperties holds normalized adapter properties read in a single backend
// call. Model and Vendor are nil when the adapter does not report them.
type AdapterProperties struct {
	Powered        bool
	Name           string
	Model          *string
	Vendor         *string
	SupportedModes []AdapterMode
}

// Adapter is the core-layer facade over a raw iwd adapter backend.
type Adapter struct {
	raw adapterRaw
}

// NewAdapter wraps a raw adapter backend in a core-layer Adapter.
func NewAdapter(raw adapterRaw) *Adapter {
	if raw == nil {
		return nil
	}
	return &Adapter{raw: raw}
}

func (a *Adapter) rawAdapter(op string) (adapterRaw, error) {
	if a == nil || a.raw == nil {
		return nil, WrapInvalidState(ResourceAdapter, op, "adapter wrapper was nil", ErrAdapterNotInitialized)
	}
	return a.raw, nil
}

// Powered returns the normalized adapter powered state.
func (a *Adapter) Powered(ctx context.Context) (bool, error) {
	const op = "Adapter.Powered"

	raw, err := a.rawAdapter(op)
	if err != nil {
		return false, err
	}

	value, err := raw.GetPowered(ctx)
	if err != nil {
		return false, WrapAdapterUnavailable(op, "failed querying iwd Adapter powered", err)
	}

	return value, nil
}

// SetPowered sets the adapter powered state through the raw backend.
func (a *Adapter) SetPowered(ctx context.Context, powered bool) error {
	const op = "Adapter.SetPowered"

	raw, err := a.rawAdapter(op)
	if err != nil {
		return err
	}

	if err := raw.SetPowered(ctx, powered); err != nil {
		return WrapAdapterUnavailable(op, "failed setting iwd Adapter powered", err)
	}

	return nil
}

// Name returns the normalized adapter name.
func (a *Adapter) Name(ctx context.Context) (string, error) {
	const op = "Adapter.Name"

	rawAdapter, err := a.rawAdapter(op)
	if err != nil {
		return "", err
	}

	raw, err := rawAdapter.GetName(ctx)
	if err != nil {
		return "", WrapAdapterUnavailable(op, "failed querying iwd Adapter name", err)
	}

	n := strings.TrimSpace(raw)
	if n == "" {
		return "", WrapInvalidState(ResourceAdapter, op, "adapter returned empty Name", fmt.Errorf("missing or invalid Name field"))
	}

	return n, nil
}

// Model returns the normalized adapter model when present.
func (a *Adapter) Model(ctx context.Context) (*string, error) {
	const op = "Adapter.Model"

	rawAdapter, err := a.rawAdapter(op)
	if err != nil {
		return nil, err
	}

	raw, err := rawAdapter.GetModel(ctx)
	if err != nil {
		return nil, WrapAdapterUnavailable(op, "failed querying iwd Adapter model", err)
	}

	if raw != nil {
		m := strings.TrimSpace(*raw)
		return &m, nil
	}
	return nil, nil
}

// Vendor returns the normalized adapter vendor when present.
func (a *Adapter) Vendor(ctx context.Context) (*string, error) {
	const op = "Adapter.Vendor"

	rawAdapter, err := a.rawAdapter(op)
	if err != nil {
		return nil, err
	}

	raw, err := rawAdapter.GetVendor(ctx)
	if err != nil {
		return nil, WrapAdapterUnavailable(op, "failed querying iwd Adapter vendor", err)
	}

	if raw != nil {
		v := strings.TrimSpace(*raw)
		return &v, nil
	}
	return nil, nil
}

// SupportedModes returns normalized adapter modes.
func (a *Adapter) SupportedModes(ctx context.Context) ([]AdapterMode, error) {
	const op = "Adapter.SupportedModes"

	rawAdapter, err := a.rawAdapter(op)
	if err != nil {
		return nil, err
	}

	raw, err := rawAdapter.GetSupportedModes(ctx)
	if err != nil {
		return nil, WrapAdapterUnavailable(op, "failed querying iwd Adapter supported modes", err)
	}

	return validateSupportedModes(raw)
}

// Properties returns all normalized adapter properties read in a single backend
// call (Properties.GetAll), applying the same normalization as the per-property
// getters: Name is trimmed and required, Model/Vendor are trimmed when present,
// and SupportedModes are validated.
func (a *Adapter) Properties(ctx context.Context) (*AdapterProperties, error) {
	const op = "Adapter.Properties"

	rawAdapter, err := a.rawAdapter(op)
	if err != nil {
		return nil, err
	}

	raw, err := rawAdapter.GetProperties(ctx)
	if err != nil {
		return nil, WrapAdapterUnavailable(op, "failed querying iwd Adapter properties", err)
	}

	name := strings.TrimSpace(raw.Name)
	if name == "" {
		return nil, WrapInvalidState(ResourceAdapter, op, "adapter returned empty Name", fmt.Errorf("missing or invalid Name field"))
	}

	modes, err := validateSupportedModes(raw.SupportedModes)
	if err != nil {
		return nil, err
	}

	props := &AdapterProperties{
		Powered:        raw.Powered,
		Name:           name,
		SupportedModes: modes,
	}
	if raw.Model != nil {
		m := strings.TrimSpace(*raw.Model)
		props.Model = &m
	}
	if raw.Vendor != nil {
		v := strings.TrimSpace(*raw.Vendor)
		props.Vendor = &v
	}

	return props, nil
}

// SupportsMode reports whether the adapter supports mode.
func (a *Adapter) SupportsMode(ctx context.Context, mode AdapterMode) (bool, error) {
	const op = "Adapter.SupportsMode"

	rawAdapter, err := a.rawAdapter(op)
	if err != nil {
		return false, err
	}

	if !iwdvalue.ValidAdapterMode(mode) {
		err := fmt.Errorf("unknown supported mode %q", mode)
		return false, WrapInvalidArgument(ResourceAdapter, op, "unknown adapter mode", err)
	}

	raw, err := rawAdapter.SupportsMode(ctx, mode)
	if err != nil {
		return false, WrapAdapterUnavailable(op, "failed querying iwd Adapter supports mode", err)
	}

	return raw, nil
}

// SupportsStation reports whether the adapter supports station mode.
func (a *Adapter) SupportsStation(ctx context.Context) (bool, error) {
	const op = "Adapter.SupportsStation"

	rawAdapter, err := a.rawAdapter(op)
	if err != nil {
		return false, err
	}

	raw, err := rawAdapter.SupportsStation(ctx)
	if err != nil {
		return false, WrapAdapterUnavailable(op, "failed querying iwd Adapter supports station", err)
	}

	return raw, nil
}

// SupportsAP reports whether the adapter supports access point mode.
func (a *Adapter) SupportsAP(ctx context.Context) (bool, error) {
	const op = "Adapter.SupportsAP"

	rawAdapter, err := a.rawAdapter(op)
	if err != nil {
		return false, err
	}

	raw, err := rawAdapter.SupportsAP(ctx)
	if err != nil {
		return false, WrapAdapterUnavailable(op, "failed querying iwd Adapter supports AP", err)
	}

	return raw, nil
}

// SupportsAdHoc reports whether the adapter supports ad-hoc mode.
func (a *Adapter) SupportsAdHoc(ctx context.Context) (bool, error) {
	const op = "Adapter.SupportsAdHoc"

	rawAdapter, err := a.rawAdapter(op)
	if err != nil {
		return false, err
	}

	raw, err := rawAdapter.SupportsAdHoc(ctx)
	if err != nil {
		return false, WrapAdapterUnavailable(op, "failed querying iwd Adapter supports ad-hoc", err)
	}

	return raw, nil
}

// SubscribePropertiesChanged registers fn for normalized property-change events.
func (a *Adapter) SubscribePropertiesChanged(ctx context.Context, fn func(AdapterPropertiesChanged)) (UnsubscribeFunc, error) {
	const op = "Adapter.SubscribePropertiesChanged"

	rawAdapter, err := a.rawAdapter(op)
	if err != nil {
		return nil, err
	}
	if fn == nil {
		return nil, WrapInvalidArgument(ResourceAdapter, op, "callback cannot be nil", ErrCore)
	}

	unsub, err := rawAdapter.SubscribePropertiesChanged(ctx, func(raw iwdbus.AdapterPropertiesChanged) {
		changed := make(map[string]any, len(raw.Changed))
		for k, v := range raw.Changed {
			changed[k] = v.Value()
		}
		// Copy invalidated to avoid aliasing/mutation across layers.
		var invalidated []string
		if raw.Invalidated != nil {
			invalidated = slices.Clone(raw.Invalidated)
		}

		fn(AdapterPropertiesChanged{
			Changed:     changed,
			Invalidated: invalidated,
		})
	})
	if err != nil {
		return nil, WrapAdapterUnavailable(op, "failed to call iwd Adapter subscribe method", err)
	}
	return UnsubscribeFunc(unsub), nil
}

// SubscribePoweredChanged registers fn for normalized powered-state events.
func (a *Adapter) SubscribePoweredChanged(ctx context.Context, fn func(bool)) (UnsubscribeFunc, error) {
	const op = "Adapter.SubscribePoweredChanged"

	rawAdapter, err := a.rawAdapter(op)
	if err != nil {
		return nil, err
	}
	if fn == nil {
		return nil, WrapInvalidArgument(ResourceAdapter, op, "callback cannot be nil", ErrCore)
	}

	unsub, err := rawAdapter.SubscribePoweredChanged(ctx, fn)
	if err != nil {
		return nil, WrapAdapterUnavailable(op, "failed to call iwd Adapter subscribe method", err)
	}
	return UnsubscribeFunc(unsub), nil
}

// ParseAdapterMode converts a canonical iwd mode string to an AdapterMode.
func ParseAdapterMode(s string) (AdapterMode, error) {
	mode, ok := iwdvalue.ParseAdapterMode(s)
	if !ok {
		details := fmt.Sprintf("invalid adapter mode %q", s)
		return AdapterModeUnknown, &Error{Kind: KindInvalidArgument, Resource: ResourceAdapter, Op: "Adapter.ParseAdapterMode", Details: details, Err: ErrCore}
	}
	return mode, nil
}

func validateSupportedModes(modes []iwdbus.AdapterMode) ([]AdapterMode, error) {
	for _, mode := range modes {
		if !iwdvalue.ValidAdapterMode(mode) {
			details := fmt.Sprintf("unknown supported mode %q", mode)
			return nil, &Error{Kind: KindInvalidArgument, Resource: ResourceAdapter, Op: "Adapter.validateSupportedModes", Details: details, Err: ErrCore}
		}
	}
	return slices.Clone(modes), nil
}
