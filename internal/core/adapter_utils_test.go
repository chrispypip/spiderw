//go:build unit || race || stress

package core

import (
	"context"
	"slices"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/chrispypip/spiderw/internal/iwdbus"
)

type fakeAdapterError struct {
	err error
}

type fakeIwdbusAdapter struct {
	powered       atomic.Bool
	name          atomic.Value // string
	model         atomic.Value // *string
	vendor        atomic.Value // *string
	modes         atomic.Value // []iwdbus.AdapterMode
	subPropsEvent atomic.Value //iwdbus.AdapterPropertiesChanged

	// Check for if SetPowered was called
	setPoweredCalled atomic.Bool

	err atomic.Pointer[fakeAdapterError]
}

func (f *fakeIwdbusAdapter) setErr(err error) {
	if err == nil {
		f.err.Store(nil)
		return
	}

	f.err.Store(&fakeAdapterError{err: err})
}

func (f *fakeIwdbusAdapter) loadErr() error {
	box := f.err.Load()
	if box == nil {
		return nil
	}
	return box.err
}

func (f *fakeIwdbusAdapter) GetPowered(ctx context.Context) (bool, error) {
	return f.powered.Load(), f.loadErr()
}

func (f *fakeIwdbusAdapter) SetPowered(ctx context.Context, powered bool) error {
	f.powered.Store(powered)
	f.setPoweredCalled.Store(true)
	return f.loadErr()
}

func (f *fakeIwdbusAdapter) GetName(context.Context) (string, error) {
	if v := f.name.Load(); v != nil {
		return v.(string), f.loadErr()
	}
	return "", f.loadErr()
}

func (f *fakeIwdbusAdapter) GetModel(context.Context) (*string, error) {
	if v := f.model.Load(); v != nil {
		return v.(*string), f.loadErr()
	}
	return nil, f.loadErr()
}

func (f *fakeIwdbusAdapter) GetVendor(context.Context) (*string, error) {
	if v := f.vendor.Load(); v != nil {
		return v.(*string), f.loadErr()
	}
	return nil, f.loadErr()
}

func (f *fakeIwdbusAdapter) GetSupportedModes(context.Context) ([]iwdbus.AdapterMode, error) {
	if v := f.modes.Load(); v != nil {
		in := v.([]iwdbus.AdapterMode)
		out := make([]iwdbus.AdapterMode, len(in))
		copy(out, in)
		return out, f.loadErr()
	}
	return nil, f.loadErr()
}

func (f *fakeIwdbusAdapter) SupportsMode(ctx context.Context, mode iwdbus.AdapterMode) (bool, error) {
	modes, _ := f.GetSupportedModes(ctx)
	return slices.Contains(modes, mode), f.loadErr()
}

func (f *fakeIwdbusAdapter) SupportsStation(ctx context.Context) (bool, error) {
	return f.SupportsMode(ctx, iwdbus.AdapterModeStation)
}

func (f *fakeIwdbusAdapter) SupportsAP(ctx context.Context) (bool, error) {
	return f.SupportsMode(ctx, iwdbus.AdapterModeAP)
}

func (f *fakeIwdbusAdapter) SupportsAdHoc(ctx context.Context) (bool, error) {
	return f.SupportsMode(ctx, iwdbus.AdapterModeAdHoc)
}

func (f *fakeIwdbusAdapter) SubscribePropertiesChanged(ctx context.Context, fn func(iwdbus.AdapterPropertiesChanged)) (iwdbus.UnsubscribeFunc, error) {
	if fn == nil {
		return nil, f.loadErr()
	}

	if v := f.subPropsEvent.Load(); v != nil {
		fn(v.(iwdbus.AdapterPropertiesChanged))
	}

	return func() error { return nil }, f.loadErr()
}

func (f *fakeIwdbusAdapter) SubscribePoweredChanged(ctx context.Context, fn func(bool)) (iwdbus.UnsubscribeFunc, error) {
	if fn == nil {
		return nil, f.loadErr()
	}

	if v := f.subPropsEvent.Load(); v != nil {
		props := v.(iwdbus.AdapterPropertiesChanged)
		variant, ok := props.Changed["Powered"]
		if !ok {
			return func() error { return nil }, f.loadErr()
		}
		b, ok := variant.Value().(bool)
		if ok {
			fn(b)
		}
	}

	return func() error { return nil }, f.loadErr()
}

// newTestAdapter mirrors helpers used by internal/iwdbus tests.
// It returns a fully-initialized *Adapter backed by a concurrency-safe fake
// that always returns valid properties
func newTestAdapter(t *testing.T) *Adapter {
	t.Helper()

	model := "Broadcomm"
	f := &fakeIwdbusAdapter{}
	f.powered.Store(true)
	f.name.Store("phy0")
	f.vendor.Store(&model)

	a := NewAdapter(f)
	require.NotNil(t, a)

	return a
}
