//go:build unit || race || stress

package spiderw

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/require"

	"github.com/chrispypip/spiderw/internal/connect"
	"github.com/chrispypip/spiderw/internal/core"
)

type fakeCoreKnownNetworkError struct {
	err error
}

// fakeCoreKnownNetwork is a concurrency-safe fake core.KnownNetworkIface so race
// and stress tests can drive the public wrapper from many goroutines.
type fakeCoreKnownNetwork struct {
	props        atomic.Pointer[core.KnownNetworkProperties]
	err          atomic.Pointer[fakeCoreKnownNetworkError]
	forgetErr    atomic.Pointer[fakeCoreKnownNetworkError]
	autoConnEvnt atomic.Pointer[bool]
	hiddenEvnt   atomic.Pointer[bool]
	lastConnEvnt atomic.Pointer[optStringEvent]
}

func (f *fakeCoreKnownNetwork) setProps(p core.KnownNetworkProperties) *fakeCoreKnownNetwork {
	cp := p
	f.props.Store(&cp)
	return f
}

// setErr makes every accessor (and the subscribe calls) return err so the public
// wrapper's backend-error mapping can be exercised.
func (f *fakeCoreKnownNetwork) setErr(err error) *fakeCoreKnownNetwork {
	if err == nil {
		f.err.Store(nil)
		return f
	}
	f.err.Store(&fakeCoreKnownNetworkError{err: err})
	return f
}

func (f *fakeCoreKnownNetwork) loadErr() error {
	if box := f.err.Load(); box != nil {
		return box.err
	}
	return nil
}

func (f *fakeCoreKnownNetwork) setForgetErr(err error) *fakeCoreKnownNetwork {
	if err == nil {
		f.forgetErr.Store(nil)
		return f
	}
	f.forgetErr.Store(&fakeCoreKnownNetworkError{err: err})
	return f
}

func (f *fakeCoreKnownNetwork) setAutoConnectEvent(autoConnect bool) *fakeCoreKnownNetwork {
	f.autoConnEvnt.Store(new(autoConnect))
	return f
}

func (f *fakeCoreKnownNetwork) loadProps() core.KnownNetworkProperties {
	if p := f.props.Load(); p != nil {
		return *p
	}
	return core.KnownNetworkProperties{}
}

func (f *fakeCoreKnownNetwork) Name(ctx context.Context) (string, error) {
	return f.loadProps().Name, f.loadErr()
}
func (f *fakeCoreKnownNetwork) Type(ctx context.Context) (core.NetworkType, error) {
	return f.loadProps().Type, f.loadErr()
}
func (f *fakeCoreKnownNetwork) Hidden(ctx context.Context) (bool, error) {
	return f.loadProps().Hidden, f.loadErr()
}
func (f *fakeCoreKnownNetwork) LastConnectedTime(ctx context.Context) (*string, error) {
	return f.loadProps().LastConnectedTime, f.loadErr()
}
func (f *fakeCoreKnownNetwork) AutoConnect(ctx context.Context) (bool, error) {
	return f.loadProps().AutoConnect, f.loadErr()
}

func (f *fakeCoreKnownNetwork) SetAutoConnect(ctx context.Context, autoConnect bool) error {
	if err := f.loadErr(); err != nil {
		return err
	}
	p := f.loadProps()
	p.AutoConnect = autoConnect
	f.props.Store(&p)
	return nil
}

func (f *fakeCoreKnownNetwork) Forget(ctx context.Context) error {
	if box := f.forgetErr.Load(); box != nil {
		return box.err
	}
	return nil
}

func (f *fakeCoreKnownNetwork) Properties(ctx context.Context) (*core.KnownNetworkProperties, error) {
	if err := f.loadErr(); err != nil {
		return nil, err
	}
	p := f.loadProps()
	return &p, nil
}

func (f *fakeCoreKnownNetwork) SubscribePropertiesChanged(ctx context.Context, fn func(core.KnownNetworkPropertiesChanged)) (core.UnsubscribeFunc, error) {
	if fn == nil {
		return nil, f.loadErr()
	}
	if ev := f.autoConnEvnt.Load(); ev != nil {
		fn(core.KnownNetworkPropertiesChanged{
			Changed:     map[string]any{"AutoConnect": *ev},
			Invalidated: []string{"LastConnectedTime"},
		})
	}
	return func() error { return nil }, f.loadErr()
}

func (f *fakeCoreKnownNetwork) SubscribeAutoConnectChanged(ctx context.Context, fn func(bool)) (core.UnsubscribeFunc, error) {
	if fn == nil {
		return nil, f.loadErr()
	}
	if ev := f.autoConnEvnt.Load(); ev != nil {
		fn(*ev)
	}
	return func() error { return nil }, f.loadErr()
}

func validCoreKnownNetworkProps() core.KnownNetworkProperties {
	return core.KnownNetworkProperties{
		Name:              "HomeNet",
		Type:              core.NetworkTypePSK,
		Hidden:            false,
		LastConnectedTime: new("2024-01-02T03:04:05Z"),
		AutoConnect:       true,
	}
}

// newTestKnownNetwork returns a public *KnownNetwork backed by a concurrency-safe
// fake.
func newTestKnownNetwork(t *testing.T) *KnownNetwork {
	t.Helper()
	k := newKnownNetwork((&fakeCoreKnownNetwork{}).setProps(validCoreKnownNetworkProps()), "/net/connman/iwd/abc")
	require.NotNil(t, k)
	return k
}

// newTestKnownNetworkClient returns a Client wired to a fake backend that
// enumerates a single known network and constructs concurrency-safe handles.
func newTestKnownNetworkClient(t *testing.T) *Client {
	t.Helper()

	fakeDaemon := &fakeCoreDaemon{}
	fakeDaemon.setKnownNetworks([]core.KnownNetworkRef{
		{Path: "/net/connman/iwd/abc", Name: "HomeNet"},
	})
	wire := &connect.Wiring{
		Conn:             &dbus.Conn{},
		ResolverOverride: connect.NoResolver{},
		Daemon:           fakeDaemon,
		Cleanup:          func() error { return nil },
		KnownNetworkFactory: func(ctx context.Context, path string) (core.KnownNetworkIface, error) {
			return (&fakeCoreKnownNetwork{}).setProps(validCoreKnownNetworkProps()), nil
		},
	}
	c, err := newClientFromWiring(wire)
	require.NoError(t, err)
	return c
}

func (f *fakeCoreKnownNetwork) SubscribeHiddenChanged(ctx context.Context, fn func(bool)) (core.UnsubscribeFunc, error) {
	if fn == nil {
		return nil, f.loadErr()
	}
	if ev := f.hiddenEvnt.Load(); ev != nil {
		fn(*ev)
	}
	return func() error { return nil }, f.loadErr()
}

func (f *fakeCoreKnownNetwork) SubscribeLastConnectedTimeChanged(ctx context.Context, fn func(*string)) (core.UnsubscribeFunc, error) {
	if fn == nil {
		return nil, f.loadErr()
	}
	if ev := f.lastConnEvnt.Load(); ev != nil {
		fn(ev.v)
	}
	return func() error { return nil }, f.loadErr()
}
