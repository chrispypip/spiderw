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

type fakeCoreNetworkError struct {
	err error
}

// fakeCoreNetwork is a concurrency-safe fake core.NetworkIface so race and
// stress tests can drive the public wrapper from many goroutines.
type fakeCoreNetwork struct {
	props         atomic.Pointer[core.NetworkProperties]
	connectErr    atomic.Pointer[fakeCoreNetworkError]
	err           atomic.Pointer[fakeCoreNetworkError]
	connectedEvnt atomic.Pointer[bool]
}

func (f *fakeCoreNetwork) setProps(p core.NetworkProperties) *fakeCoreNetwork {
	cp := p
	f.props.Store(&cp)
	return f
}

func (f *fakeCoreNetwork) setConnectErr(err error) *fakeCoreNetwork {
	if err == nil {
		f.connectErr.Store(nil)
		return f
	}
	f.connectErr.Store(&fakeCoreNetworkError{err: err})
	return f
}

func (f *fakeCoreNetwork) setConnectedEvent(connected bool) *fakeCoreNetwork {
	f.connectedEvnt.Store(new(connected))
	return f
}

func (f *fakeCoreNetwork) loadErr() error {
	if box := f.err.Load(); box != nil {
		return box.err
	}
	return nil
}

func (f *fakeCoreNetwork) loadProps() core.NetworkProperties {
	if p := f.props.Load(); p != nil {
		return *p
	}
	return core.NetworkProperties{}
}

func (f *fakeCoreNetwork) Name(ctx context.Context) (string, error) {
	return f.loadProps().Name, f.loadErr()
}
func (f *fakeCoreNetwork) Connected(ctx context.Context) (bool, error) {
	return f.loadProps().Connected, f.loadErr()
}
func (f *fakeCoreNetwork) Device(ctx context.Context) (string, error) {
	return f.loadProps().Device, f.loadErr()
}
func (f *fakeCoreNetwork) Type(ctx context.Context) (core.NetworkType, error) {
	return f.loadProps().Type, f.loadErr()
}
func (f *fakeCoreNetwork) KnownNetwork(ctx context.Context) (*string, error) {
	return f.loadProps().KnownNetwork, f.loadErr()
}
func (f *fakeCoreNetwork) ExtendedServiceSet(ctx context.Context) ([]string, error) {
	return f.loadProps().ExtendedServiceSet, f.loadErr()
}

func (f *fakeCoreNetwork) Properties(ctx context.Context) (*core.NetworkProperties, error) {
	if err := f.loadErr(); err != nil {
		return nil, err
	}
	p := f.loadProps()
	return &p, nil
}

func (f *fakeCoreNetwork) Connect(ctx context.Context) error {
	if box := f.connectErr.Load(); box != nil {
		return box.err
	}
	p := f.loadProps()
	p.Connected = true
	f.props.Store(&p)
	return nil
}

func (f *fakeCoreNetwork) SubscribePropertiesChanged(ctx context.Context, fn func(core.NetworkPropertiesChanged)) (core.UnsubscribeFunc, error) {
	if fn == nil {
		return nil, f.loadErr()
	}
	if ev := f.connectedEvnt.Load(); ev != nil {
		fn(core.NetworkPropertiesChanged{Changed: map[string]any{"Connected": *ev}})
	}
	return func() error { return nil }, f.loadErr()
}

func (f *fakeCoreNetwork) SubscribeConnectedChanged(ctx context.Context, fn func(bool)) (core.UnsubscribeFunc, error) {
	if fn == nil {
		return nil, f.loadErr()
	}
	if ev := f.connectedEvnt.Load(); ev != nil {
		fn(*ev)
	}
	return func() error { return nil }, f.loadErr()
}

func validCoreNetworkProps() core.NetworkProperties {
	return core.NetworkProperties{
		Name:               "OpenNet",
		Device:             "/net/connman/iwd/phy0/wlan0",
		Type:               core.NetworkTypeOpen,
		ExtendedServiceSet: []string{"/net/connman/iwd/phy0/wlan0/aabbccddeeff"},
	}
}

// newTestNetwork returns a public *Network backed by a concurrency-safe fake.
func newTestNetwork(t *testing.T) *Network {
	t.Helper()
	n := newNetwork((&fakeCoreNetwork{}).setProps(validCoreNetworkProps()), "/net/connman/iwd/phy0/wlan0/open")
	require.NotNil(t, n)
	return n
}

// newTestNetworkClient returns a Client wired to a fake backend that enumerates a
// single network and constructs concurrency-safe network handles.
func newTestNetworkClient(t *testing.T) *Client {
	t.Helper()

	fakeDaemon := &fakeCoreDaemon{}
	fakeDaemon.setNetworks([]core.NetworkRef{
		{Path: "/net/connman/iwd/phy0/wlan0/open", Name: "OpenNet"},
	})
	wire := &connect.Wiring{
		Conn:             &dbus.Conn{},
		ResolverOverride: connect.NoResolver{},
		Daemon:           fakeDaemon,
		Cleanup:          func() error { return nil },
		NetworkFactory: func(ctx context.Context, path string) (core.NetworkIface, error) {
			return (&fakeCoreNetwork{}).setProps(validCoreNetworkProps()), nil
		},
	}
	c, err := newClientFromWiring(wire)
	require.NoError(t, err)
	return c
}
