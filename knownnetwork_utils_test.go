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
	forgetErr    atomic.Pointer[fakeCoreKnownNetworkError]
	autoConnEvnt atomic.Pointer[bool]
}

func (f *fakeCoreKnownNetwork) setProps(p core.KnownNetworkProperties) *fakeCoreKnownNetwork {
	cp := p
	f.props.Store(&cp)
	return f
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

func (f *fakeCoreKnownNetwork) Name(context.Context) (string, error) {
	return f.loadProps().Name, nil
}
func (f *fakeCoreKnownNetwork) Type(context.Context) (core.NetworkType, error) {
	return f.loadProps().Type, nil
}
func (f *fakeCoreKnownNetwork) Hidden(context.Context) (bool, error) {
	return f.loadProps().Hidden, nil
}
func (f *fakeCoreKnownNetwork) LastConnectedTime(context.Context) (*string, error) {
	return f.loadProps().LastConnectedTime, nil
}
func (f *fakeCoreKnownNetwork) AutoConnect(context.Context) (bool, error) {
	return f.loadProps().AutoConnect, nil
}

func (f *fakeCoreKnownNetwork) SetAutoConnect(_ context.Context, autoConnect bool) error {
	p := f.loadProps()
	p.AutoConnect = autoConnect
	f.props.Store(&p)
	return nil
}

func (f *fakeCoreKnownNetwork) Forget(context.Context) error {
	if box := f.forgetErr.Load(); box != nil {
		return box.err
	}
	return nil
}

func (f *fakeCoreKnownNetwork) Properties(context.Context) (*core.KnownNetworkProperties, error) {
	p := f.loadProps()
	return &p, nil
}

func (f *fakeCoreKnownNetwork) SubscribePropertiesChanged(_ context.Context, fn func(core.KnownNetworkPropertiesChanged)) (core.UnsubscribeFunc, error) {
	if fn == nil {
		return nil, nil
	}
	if ev := f.autoConnEvnt.Load(); ev != nil {
		fn(core.KnownNetworkPropertiesChanged{Changed: map[string]any{"AutoConnect": *ev}})
	}
	return func() error { return nil }, nil
}

func (f *fakeCoreKnownNetwork) SubscribeAutoConnectChanged(_ context.Context, fn func(bool)) (core.UnsubscribeFunc, error) {
	if fn == nil {
		return nil, nil
	}
	if ev := f.autoConnEvnt.Load(); ev != nil {
		fn(*ev)
	}
	return func() error { return nil }, nil
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
		Conn:    &dbus.Conn{},
		Daemon:  fakeDaemon,
		Cleanup: func() error { return nil },
		KnownNetworkFactory: func(_ context.Context, _ string) (core.KnownNetworkIface, error) {
			return (&fakeCoreKnownNetwork{}).setProps(validCoreKnownNetworkProps()), nil
		},
	}
	c, err := newClientFromWiring(wire)
	require.NoError(t, err)
	return c
}
