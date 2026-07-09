//go:build unit || race || stress

package spiderw

import (
	"context"
	"sync/atomic"

	"github.com/chrispypip/spiderw/internal/core"
)

type fakeCoreDeviceError struct {
	err error
}

type fakeCoreDevice struct {
	name          atomic.Value // string
	address       atomic.Value // string
	powered       atomic.Bool
	mode          atomic.Value // core.Mode
	adapter       atomic.Value // string
	subPropsEvent atomic.Value // core.DevicePropertiesChanged

	// Track whether the mutating setters were called.
	setPoweredCalled atomic.Bool
	setModeCalled    atomic.Bool

	err atomic.Pointer[fakeCoreDeviceError]
}

func (f *fakeCoreDevice) setErr(err error) {
	if err == nil {
		f.err.Store(nil)
		return
	}

	f.err.Store(&fakeCoreDeviceError{err: err})
}

func (f *fakeCoreDevice) loadErr() error {
	box := f.err.Load()
	if box == nil {
		return nil
	}
	return box.err
}

func (f *fakeCoreDevice) Name(ctx context.Context) (string, error) {
	if v := f.name.Load(); v != nil {
		return v.(string), f.loadErr()
	}
	return "", f.loadErr()
}

func (f *fakeCoreDevice) Address(ctx context.Context) (string, error) {
	if v := f.address.Load(); v != nil {
		return v.(string), f.loadErr()
	}
	return "", f.loadErr()
}

func (f *fakeCoreDevice) Powered(ctx context.Context) (bool, error) {
	return f.powered.Load(), f.loadErr()
}

func (f *fakeCoreDevice) SetPowered(ctx context.Context, powered bool) error {
	f.powered.Store(powered)
	f.setPoweredCalled.Store(true)
	return f.loadErr()
}

func (f *fakeCoreDevice) Mode(ctx context.Context) (core.Mode, error) {
	if v := f.mode.Load(); v != nil {
		return v.(core.Mode), f.loadErr()
	}
	return core.ModeUnknown, f.loadErr()
}

func (f *fakeCoreDevice) SetMode(ctx context.Context, mode core.Mode) error {
	f.mode.Store(mode)
	f.setModeCalled.Store(true)
	return f.loadErr()
}

func (f *fakeCoreDevice) Adapter(ctx context.Context) (string, error) {
	if v := f.adapter.Load(); v != nil {
		return v.(string), f.loadErr()
	}
	return "", f.loadErr()
}

func (f *fakeCoreDevice) Properties(ctx context.Context) (*core.DeviceProperties, error) {
	if err := f.loadErr(); err != nil {
		return nil, err
	}

	props := &core.DeviceProperties{Powered: f.powered.Load()}
	if v := f.name.Load(); v != nil {
		props.Name = v.(string)
	}
	if v := f.address.Load(); v != nil {
		props.Address = v.(string)
	}
	if v := f.mode.Load(); v != nil {
		props.Mode = v.(core.Mode)
	}
	if v := f.adapter.Load(); v != nil {
		props.Adapter = v.(string)
	}
	return props, nil
}

func (f *fakeCoreDevice) SubscribePropertiesChanged(ctx context.Context, fn func(core.DevicePropertiesChanged)) (core.UnsubscribeFunc, error) {
	if fn == nil {
		return nil, f.loadErr()
	}

	if v := f.subPropsEvent.Load(); v != nil {
		fn(v.(core.DevicePropertiesChanged))
	}

	return func() error { return nil }, f.loadErr()
}

func (f *fakeCoreDevice) SubscribePoweredChanged(ctx context.Context, fn func(bool)) (core.UnsubscribeFunc, error) {
	if fn == nil {
		return nil, f.loadErr()
	}

	if v := f.subPropsEvent.Load(); v != nil {
		props := v.(core.DevicePropertiesChanged)
		if pow, ok := props.Changed["Powered"]; ok {
			if b, ok := pow.(bool); ok {
				fn(b)
			}
		}
	}

	return func() error { return nil }, f.loadErr()
}

func (f *fakeCoreDevice) SubscribeModeChanged(ctx context.Context, fn func(core.Mode)) (core.UnsubscribeFunc, error) {
	if fn == nil {
		return nil, f.loadErr()
	}

	if v := f.subPropsEvent.Load(); v != nil {
		props := v.(core.DevicePropertiesChanged)
		if m, ok := props.Changed["Mode"]; ok {
			// Deliver the raw mode as the core layer would; the public wrapper is
			// responsible for validating and dropping unrecognized modes.
			if s, ok := m.(string); ok {
				fn(core.Mode(s))
			}
		}
	}

	return func() error { return nil }, f.loadErr()
}
