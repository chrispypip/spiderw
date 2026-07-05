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

type fakeKnownNetworkError struct {
	err error
}

// fakeIwdbusKnownNetwork is a concurrency-safe fake knownNetworkRaw backend so
// race and stress tests can call it from many goroutines.
type fakeIwdbusKnownNetwork struct {
	props        atomic.Pointer[iwdbus.KnownNetworkProperties]
	err          atomic.Pointer[fakeKnownNetworkError]
	forgetErr    atomic.Pointer[fakeKnownNetworkError]
	autoConnEvnt atomic.Pointer[bool]
}

func (f *fakeIwdbusKnownNetwork) setProps(p iwdbus.KnownNetworkProperties) *fakeIwdbusKnownNetwork {
	cp := p
	f.props.Store(&cp)
	return f
}

func (f *fakeIwdbusKnownNetwork) setErr(err error) *fakeIwdbusKnownNetwork {
	if err == nil {
		f.err.Store(nil)
		return f
	}
	f.err.Store(&fakeKnownNetworkError{err: err})
	return f
}

func (f *fakeIwdbusKnownNetwork) setForgetErr(err error) *fakeIwdbusKnownNetwork {
	if err == nil {
		f.forgetErr.Store(nil)
		return f
	}
	f.forgetErr.Store(&fakeKnownNetworkError{err: err})
	return f
}

func (f *fakeIwdbusKnownNetwork) setAutoConnectEvent(autoConnect bool) *fakeIwdbusKnownNetwork {
	f.autoConnEvnt.Store(new(autoConnect))
	return f
}

func (f *fakeIwdbusKnownNetwork) loadErr() error {
	if box := f.err.Load(); box != nil {
		return box.err
	}
	return nil
}

func (f *fakeIwdbusKnownNetwork) loadProps() iwdbus.KnownNetworkProperties {
	if p := f.props.Load(); p != nil {
		return *p
	}
	return iwdbus.KnownNetworkProperties{}
}

func (f *fakeIwdbusKnownNetwork) GetName(ctx context.Context) (string, error) {
	return f.loadProps().Name, f.loadErr()
}
func (f *fakeIwdbusKnownNetwork) GetType(ctx context.Context) (iwdbus.NetworkType, error) {
	return f.loadProps().Type, f.loadErr()
}
func (f *fakeIwdbusKnownNetwork) GetHidden(ctx context.Context) (bool, error) {
	return f.loadProps().Hidden, f.loadErr()
}
func (f *fakeIwdbusKnownNetwork) GetLastConnectedTime(ctx context.Context) (*string, error) {
	return f.loadProps().LastConnectedTime, f.loadErr()
}
func (f *fakeIwdbusKnownNetwork) GetAutoConnect(ctx context.Context) (bool, error) {
	return f.loadProps().AutoConnect, f.loadErr()
}

func (f *fakeIwdbusKnownNetwork) SetAutoConnect(ctx context.Context, autoConnect bool) error {
	if err := f.loadErr(); err != nil {
		return err
	}
	p := f.loadProps()
	p.AutoConnect = autoConnect
	f.props.Store(&p)
	return nil
}

func (f *fakeIwdbusKnownNetwork) Forget(ctx context.Context) error {
	if box := f.forgetErr.Load(); box != nil {
		return box.err
	}
	return nil
}

func (f *fakeIwdbusKnownNetwork) GetProperties(ctx context.Context) (*iwdbus.KnownNetworkProperties, error) {
	if err := f.loadErr(); err != nil {
		return nil, err
	}
	p := f.loadProps()
	return &p, nil
}

func (f *fakeIwdbusKnownNetwork) SubscribePropertiesChanged(ctx context.Context, fn func(iwdbus.KnownNetworkPropertiesChanged)) (iwdbus.UnsubscribeFunc, error) {
	if fn == nil {
		return nil, f.loadErr()
	}
	if ev := f.autoConnEvnt.Load(); ev != nil {
		fn(iwdbus.KnownNetworkPropertiesChanged{Changed: map[string]dbus.Variant{"AutoConnect": dbus.MakeVariant(*ev)}})
	}
	return func() error { return nil }, f.loadErr()
}

func (f *fakeIwdbusKnownNetwork) SubscribeAutoConnectChanged(ctx context.Context, fn func(bool)) (iwdbus.UnsubscribeFunc, error) {
	if fn == nil {
		return nil, f.loadErr()
	}
	if ev := f.autoConnEvnt.Load(); ev != nil {
		fn(*ev)
	}
	return func() error { return nil }, f.loadErr()
}

// validKnownNetworkProps returns a fully-populated set of valid known-network
// properties.
func validKnownNetworkProps() iwdbus.KnownNetworkProperties {
	return iwdbus.KnownNetworkProperties{
		Name:              "HomeNet",
		Type:              iwdbus.NetworkTypePSK,
		Hidden:            false,
		LastConnectedTime: new("2024-01-02T03:04:05Z"),
		AutoConnect:       true,
	}
}

// newTestKnownNetwork returns a fully-initialized *KnownNetwork backed by a
// concurrency-safe fake that reports valid properties.
func newTestKnownNetwork(t *testing.T) *KnownNetwork {
	t.Helper()

	f := (&fakeIwdbusKnownNetwork{}).setProps(validKnownNetworkProps())
	k := NewKnownNetwork(f)
	require.NotNil(t, k)
	return k
}
