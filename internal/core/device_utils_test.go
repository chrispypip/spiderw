//go:build unit || race || stress

package core

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/require"

	"github.com/chrispypip/spiderw/internal/iwdbus"
)

type fakeDeviceError struct {
	err error
}

type fakeIwdbusDevice struct {
	name          atomic.Value // string
	address       atomic.Value // string
	powered       atomic.Bool
	mode          atomic.Value // iwdbus.Mode
	adapter       atomic.Value // dbus.ObjectPath
	subPropsEvent atomic.Value // iwdbus.DevicePropertiesChanged

	// Check for whether the mutating setters were called.
	setPoweredCalled atomic.Bool
	setModeCalled    atomic.Bool

	err atomic.Pointer[fakeDeviceError]
}

func (f *fakeIwdbusDevice) setErr(err error) {
	if err == nil {
		f.err.Store(nil)
		return
	}

	f.err.Store(&fakeDeviceError{err: err})
}

func (f *fakeIwdbusDevice) loadErr() error {
	box := f.err.Load()
	if box == nil {
		return nil
	}
	return box.err
}

func (f *fakeIwdbusDevice) GetName(ctx context.Context) (string, error) {
	if v := f.name.Load(); v != nil {
		return v.(string), f.loadErr()
	}
	return "", f.loadErr()
}

func (f *fakeIwdbusDevice) GetAddress(ctx context.Context) (string, error) {
	if v := f.address.Load(); v != nil {
		return v.(string), f.loadErr()
	}
	return "", f.loadErr()
}

func (f *fakeIwdbusDevice) GetPowered(ctx context.Context) (bool, error) {
	return f.powered.Load(), f.loadErr()
}

func (f *fakeIwdbusDevice) SetPowered(ctx context.Context, powered bool) error {
	f.powered.Store(powered)
	f.setPoweredCalled.Store(true)
	return f.loadErr()
}

func (f *fakeIwdbusDevice) GetMode(ctx context.Context) (iwdbus.Mode, error) {
	if v := f.mode.Load(); v != nil {
		return v.(iwdbus.Mode), f.loadErr()
	}
	return iwdbus.ModeUnknown, f.loadErr()
}

func (f *fakeIwdbusDevice) SetMode(ctx context.Context, mode iwdbus.Mode) error {
	f.mode.Store(mode)
	f.setModeCalled.Store(true)
	return f.loadErr()
}

func (f *fakeIwdbusDevice) GetAdapter(ctx context.Context) (dbus.ObjectPath, error) {
	if v := f.adapter.Load(); v != nil {
		return v.(dbus.ObjectPath), f.loadErr()
	}
	return "", f.loadErr()
}

func (f *fakeIwdbusDevice) GetProperties(ctx context.Context) (*iwdbus.DeviceProperties, error) {
	if err := f.loadErr(); err != nil {
		return nil, err
	}

	props := &iwdbus.DeviceProperties{Powered: f.powered.Load()}
	if v := f.name.Load(); v != nil {
		props.Name = v.(string)
	}
	if v := f.address.Load(); v != nil {
		props.Address = v.(string)
	}
	if v := f.mode.Load(); v != nil {
		props.Mode = v.(iwdbus.Mode)
	}
	if v := f.adapter.Load(); v != nil {
		props.Adapter = v.(dbus.ObjectPath)
	}
	return props, nil
}

func (f *fakeIwdbusDevice) SubscribePropertiesChanged(ctx context.Context, fn func(iwdbus.DevicePropertiesChanged)) (iwdbus.UnsubscribeFunc, error) {
	if fn == nil {
		return nil, f.loadErr()
	}

	if v := f.subPropsEvent.Load(); v != nil {
		fn(v.(iwdbus.DevicePropertiesChanged))
	}

	return func() error { return nil }, f.loadErr()
}

func (f *fakeIwdbusDevice) SubscribePoweredChanged(ctx context.Context, fn func(bool)) (iwdbus.UnsubscribeFunc, error) {
	if fn == nil {
		return nil, f.loadErr()
	}

	if v := f.subPropsEvent.Load(); v != nil {
		props := v.(iwdbus.DevicePropertiesChanged)
		if variant, ok := props.Changed["Powered"]; ok {
			if b, ok := variant.Value().(bool); ok {
				fn(b)
			}
		}
	}

	return func() error { return nil }, f.loadErr()
}

func (f *fakeIwdbusDevice) SubscribeModeChanged(ctx context.Context, fn func(iwdbus.Mode)) (iwdbus.UnsubscribeFunc, error) {
	if fn == nil {
		return nil, f.loadErr()
	}

	if v := f.subPropsEvent.Load(); v != nil {
		props := v.(iwdbus.DevicePropertiesChanged)
		if variant, ok := props.Changed["Mode"]; ok {
			if s, ok := variant.Value().(string); ok {
				if mode, parseOK := iwdbus.ParseMode(s); parseOK == nil {
					fn(mode)
				}
			}
		}
	}

	return func() error { return nil }, f.loadErr()
}

// newTestDevice returns a fully-initialized *Device backed by a
// concurrency-safe fake that reports valid properties.
func newTestDevice(t *testing.T) *Device {
	t.Helper()

	f := &fakeIwdbusDevice{}
	f.name.Store("wlan0")
	f.address.Store("aa:bb:cc:dd:ee:ff")
	f.powered.Store(true)
	f.mode.Store(iwdbus.ModeStation)
	f.adapter.Store(dbus.ObjectPath("/net/connman/iwd/phy0"))

	d := NewDevice(f)
	require.NotNil(t, d)

	return d
}
