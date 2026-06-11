//go:build unit || race || stress

package spiderw

import (
	"context"
	"slices"
	"sync/atomic"

	"github.com/chrispypip/spiderw/internal/core"
)

type fakeCoreAdapterError struct {
	err error
}

type fakeCoreAdapter struct {
	powered       atomic.Bool
	name          atomic.Value // string
	model         atomic.Value // *string
	vendor        atomic.Value // *string
	modes         atomic.Value // []core.AdapterMode
	subPropsEvent atomic.Value // core.AdapterPropertiesChanged

	// Check for it SetPowered was called
	setPoweredCalled atomic.Bool

	err atomic.Pointer[fakeCoreAdapterError]
}

func (f *fakeCoreAdapter) setErr(err error) {
	if err == nil {
		f.err.Store(nil)
		return
	}

	f.err.Store(&fakeCoreAdapterError{err: err})
}

func (f *fakeCoreAdapter) loadErr() error {
	box := f.err.Load()
	if box == nil {
		return nil
	}
	return box.err
}

func (f *fakeCoreAdapter) Powered(ctx context.Context) (bool, error) {
	return f.powered.Load(), f.loadErr()
}

func (f *fakeCoreAdapter) SetPowered(ctx context.Context, powered bool) error {
	f.powered.Store(powered)
	f.setPoweredCalled.Store(true)
	return f.loadErr()
}

func (f *fakeCoreAdapter) Name(ctx context.Context) (string, error) {
	if v := f.name.Load(); v != nil {
		return v.(string), f.loadErr()
	}
	return "", f.loadErr()
}

func (f *fakeCoreAdapter) Model(ctx context.Context) (*string, error) {
	if v := f.model.Load(); v != nil {
		return v.(*string), f.loadErr()
	}
	return nil, f.loadErr()
}

func (f *fakeCoreAdapter) Vendor(ctx context.Context) (*string, error) {
	if v := f.vendor.Load(); v != nil {
		return v.(*string), f.loadErr()
	}
	return nil, f.loadErr()
}

func (f *fakeCoreAdapter) SupportedModes(ctx context.Context) ([]core.AdapterMode, error) {
	if v := f.modes.Load(); v != nil {
		in := v.([]core.AdapterMode)
		out := make([]core.AdapterMode, len(in))
		copy(out, in)
		return out, f.loadErr()
	}
	return nil, f.loadErr()
}

func (f *fakeCoreAdapter) Properties(ctx context.Context) (*core.AdapterProperties, error) {
	if err := f.loadErr(); err != nil {
		return nil, err
	}

	props := &core.AdapterProperties{Powered: f.powered.Load()}
	if v := f.name.Load(); v != nil {
		props.Name = v.(string)
	}
	if v := f.model.Load(); v != nil {
		props.Model = v.(*string)
	}
	if v := f.vendor.Load(); v != nil {
		props.Vendor = v.(*string)
	}
	if v := f.modes.Load(); v != nil {
		props.SupportedModes = slices.Clone(v.([]core.AdapterMode))
	}
	return props, nil
}

func (f *fakeCoreAdapter) SupportsMode(ctx context.Context, mode core.AdapterMode) (bool, error) {
	modes, _ := f.SupportedModes(ctx)
	return slices.Contains(modes, mode), f.loadErr()
}

func (f *fakeCoreAdapter) SupportsStation(ctx context.Context) (bool, error) {
	return f.SupportsMode(ctx, core.AdapterModeStation)
}

func (f *fakeCoreAdapter) SupportsAP(ctx context.Context) (bool, error) {
	return f.SupportsMode(ctx, core.AdapterModeAP)
}

func (f *fakeCoreAdapter) SupportsAdHoc(ctx context.Context) (bool, error) {
	return f.SupportsMode(ctx, core.AdapterModeAdHoc)
}

func (f *fakeCoreAdapter) SubscribePropertiesChanged(ctx context.Context, fn func(core.AdapterPropertiesChanged)) (core.UnsubscribeFunc, error) {
	if fn == nil {
		return nil, f.loadErr()
	}

	if v := f.subPropsEvent.Load(); v != nil {
		fn(v.(core.AdapterPropertiesChanged))
	}

	return func() error { return nil }, f.loadErr()
}

func (f *fakeCoreAdapter) SubscribePoweredChanged(ctx context.Context, fn func(bool)) (core.UnsubscribeFunc, error) {
	if fn == nil {
		return nil, f.loadErr()
	}

	if v := f.subPropsEvent.Load(); v != nil {
		props := v.(core.AdapterPropertiesChanged)
		pow, ok := props.Changed["Powered"]
		if !ok {
			return func() error { return nil }, f.loadErr()
		}
		b, ok := pow.(bool)
		if ok {
			fn(b)
		}
	}

	return func() error { return nil }, f.loadErr()
}
