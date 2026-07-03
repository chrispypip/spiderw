//go:build unit || race || stress

package core

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/chrispypip/spiderw/internal/iwdbus"
	"github.com/chrispypip/spiderw/internal/iwdvalue"
)

type fakeStationError struct {
	err error
}

type fakeIwdbusStation struct {
	state                atomic.Value // iwdbus.StationState
	scanning             atomic.Bool
	connectedNetwork     atomic.Pointer[string]
	connectedAccessPoint atomic.Pointer[string]
	affinities           atomic.Pointer[[]string]
	subPropsEvent        atomic.Value // iwdbus.StationPropertiesChanged

	err atomic.Pointer[fakeStationError]
}

func (f *fakeIwdbusStation) setErr(err error) {
	if err == nil {
		f.err.Store(nil)
		return
	}
	f.err.Store(&fakeStationError{err: err})
}

func (f *fakeIwdbusStation) loadErr() error {
	box := f.err.Load()
	if box == nil {
		return nil
	}
	return box.err
}

func (f *fakeIwdbusStation) GetState(context.Context) (iwdbus.StationState, error) {
	if v := f.state.Load(); v != nil {
		return v.(iwdbus.StationState), f.loadErr()
	}
	return iwdbus.StationStateUnknown, f.loadErr()
}

func (f *fakeIwdbusStation) GetScanning(context.Context) (bool, error) {
	return f.scanning.Load(), f.loadErr()
}

func (f *fakeIwdbusStation) GetConnectedNetwork(context.Context) (*string, error) {
	return f.connectedNetwork.Load(), f.loadErr()
}

func (f *fakeIwdbusStation) GetConnectedAccessPoint(context.Context) (*string, error) {
	return f.connectedAccessPoint.Load(), f.loadErr()
}

func (f *fakeIwdbusStation) GetAffinities(context.Context) ([]string, error) {
	if v := f.affinities.Load(); v != nil {
		return *v, f.loadErr()
	}
	return nil, f.loadErr()
}

func (f *fakeIwdbusStation) GetProperties(context.Context) (*iwdbus.StationProperties, error) {
	if err := f.loadErr(); err != nil {
		return nil, err
	}

	props := &iwdbus.StationProperties{
		Scanning:             f.scanning.Load(),
		ConnectedNetwork:     f.connectedNetwork.Load(),
		ConnectedAccessPoint: f.connectedAccessPoint.Load(),
	}
	if v := f.state.Load(); v != nil {
		props.State = v.(iwdbus.StationState)
	}
	if v := f.affinities.Load(); v != nil {
		props.Affinities = *v
	}
	return props, nil
}

func (f *fakeIwdbusStation) SubscribePropertiesChanged(ctx context.Context, fn func(iwdbus.StationPropertiesChanged)) (iwdbus.UnsubscribeFunc, error) {
	if fn == nil {
		return nil, f.loadErr()
	}

	if v := f.subPropsEvent.Load(); v != nil {
		fn(v.(iwdbus.StationPropertiesChanged))
	}

	return func() error { return nil }, f.loadErr()
}

func (f *fakeIwdbusStation) SubscribeStateChanged(ctx context.Context, fn func(iwdbus.StationState)) (iwdbus.UnsubscribeFunc, error) {
	if fn == nil {
		return nil, f.loadErr()
	}

	if v := f.subPropsEvent.Load(); v != nil {
		props := v.(iwdbus.StationPropertiesChanged)
		if variant, ok := props.Changed["State"]; ok {
			if s, ok := variant.Value().(string); ok {
				if state, parseOK := iwdvalue.ParseStationState(s); parseOK {
					fn(state)
				}
			}
		}
	}

	return func() error { return nil }, f.loadErr()
}

func (f *fakeIwdbusStation) SubscribeScanningChanged(ctx context.Context, fn func(bool)) (iwdbus.UnsubscribeFunc, error) {
	if fn == nil {
		return nil, f.loadErr()
	}

	if v := f.subPropsEvent.Load(); v != nil {
		props := v.(iwdbus.StationPropertiesChanged)
		if variant, ok := props.Changed["Scanning"]; ok {
			if b, ok := variant.Value().(bool); ok {
				fn(b)
			}
		}
	}

	return func() error { return nil }, f.loadErr()
}

// newTestStation returns a fully-initialized *Station backed by a
// concurrency-safe fake that reports a valid connected state.
func newTestStation(t *testing.T) *Station {
	t.Helper()

	f := &fakeIwdbusStation{}
	f.state.Store(iwdbus.StationStateConnected)
	f.scanning.Store(false)
	f.connectedNetwork.Store(new("/net/connman/iwd/phy0/wlan0/net0"))

	s := NewStation(f)
	require.NotNil(t, s)

	return s
}
