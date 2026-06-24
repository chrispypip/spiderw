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

type fakeNetworkError struct {
	err error
}

// fakeIwdbusNetwork is a concurrency-safe fake networkRaw backend so race and
// stress tests can call it from many goroutines while tests mutate its state.
type fakeIwdbusNetwork struct {
	props         atomic.Pointer[iwdbus.NetworkProperties]
	connectErr    atomic.Pointer[fakeNetworkError]
	err           atomic.Pointer[fakeNetworkError]
	connectedEvnt atomic.Pointer[bool]
}

func (f *fakeIwdbusNetwork) setProps(p iwdbus.NetworkProperties) *fakeIwdbusNetwork {
	cp := p
	f.props.Store(&cp)
	return f
}

func (f *fakeIwdbusNetwork) setConnectErr(err error) *fakeIwdbusNetwork {
	if err == nil {
		f.connectErr.Store(nil)
		return f
	}
	f.connectErr.Store(&fakeNetworkError{err: err})
	return f
}

func (f *fakeIwdbusNetwork) setConnectedEvent(connected bool) *fakeIwdbusNetwork {
	f.connectedEvnt.Store(&connected)
	return f
}

func (f *fakeIwdbusNetwork) loadErr() error {
	if box := f.err.Load(); box != nil {
		return box.err
	}
	return nil
}

func (f *fakeIwdbusNetwork) loadProps() iwdbus.NetworkProperties {
	if p := f.props.Load(); p != nil {
		return *p
	}
	return iwdbus.NetworkProperties{}
}

func (f *fakeIwdbusNetwork) GetName(context.Context) (string, error) {
	return f.loadProps().Name, f.loadErr()
}
func (f *fakeIwdbusNetwork) GetConnected(context.Context) (bool, error) {
	return f.loadProps().Connected, f.loadErr()
}
func (f *fakeIwdbusNetwork) GetDevice(context.Context) (dbus.ObjectPath, error) {
	return f.loadProps().Device, f.loadErr()
}
func (f *fakeIwdbusNetwork) GetType(context.Context) (iwdbus.SecurityType, error) {
	return f.loadProps().Type, f.loadErr()
}
func (f *fakeIwdbusNetwork) GetKnownNetwork(context.Context) (*string, error) {
	return f.loadProps().KnownNetwork, f.loadErr()
}
func (f *fakeIwdbusNetwork) GetExtendedServiceSet(context.Context) ([]string, error) {
	return f.loadProps().ExtendedServiceSet, f.loadErr()
}

func (f *fakeIwdbusNetwork) GetProperties(context.Context) (*iwdbus.NetworkProperties, error) {
	if err := f.loadErr(); err != nil {
		return nil, err
	}
	p := f.loadProps()
	return &p, nil
}

func (f *fakeIwdbusNetwork) Connect(context.Context) error {
	if box := f.connectErr.Load(); box != nil {
		return box.err
	}
	p := f.loadProps()
	p.Connected = true
	f.props.Store(&p)
	return nil
}

func (f *fakeIwdbusNetwork) SubscribePropertiesChanged(_ context.Context, fn func(iwdbus.NetworkPropertiesChanged)) (iwdbus.UnsubscribeFunc, error) {
	if fn == nil {
		return nil, f.loadErr()
	}
	if ev := f.connectedEvnt.Load(); ev != nil {
		fn(iwdbus.NetworkPropertiesChanged{Changed: map[string]dbus.Variant{"Connected": dbus.MakeVariant(*ev)}})
	}
	return func() error { return nil }, f.loadErr()
}

func (f *fakeIwdbusNetwork) SubscribeConnectedChanged(_ context.Context, fn func(bool)) (iwdbus.UnsubscribeFunc, error) {
	if fn == nil {
		return nil, f.loadErr()
	}
	if ev := f.connectedEvnt.Load(); ev != nil {
		fn(*ev)
	}
	return func() error { return nil }, f.loadErr()
}

// validNetworkProps returns a fully-populated set of valid network properties.
func validNetworkProps() iwdbus.NetworkProperties {
	known := "/net/connman/iwd/known_networks/1"
	return iwdbus.NetworkProperties{
		Name:               "OpenNet",
		Connected:          false,
		Device:             "/net/connman/iwd/phy0/wlan0",
		Type:               iwdbus.SecurityTypeOpen,
		KnownNetwork:       &known,
		ExtendedServiceSet: []string{"/net/connman/iwd/phy0/wlan0/aabbccddeeff"},
	}
}

// newTestNetwork returns a fully-initialized *Network backed by a
// concurrency-safe fake that reports valid properties.
func newTestNetwork(t *testing.T) *Network {
	t.Helper()

	f := (&fakeIwdbusNetwork{}).setProps(validNetworkProps())
	n := NewNetwork(f)
	require.NotNil(t, n)
	return n
}
