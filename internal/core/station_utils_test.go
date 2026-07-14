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

// optStringEvent and stringSliceEvent wrap subscribe payloads so a fake can
// distinguish "no event configured" (nil pointer) from "an event carrying nil"
// (e.g. ConnectedNetwork when disconnected).
type optStringEvent struct{ v *string }

type stringSliceEvent struct{ v []string }

type fakeIwdbusStation struct {
	state                atomic.Value // iwdbus.StationState
	scanning             atomic.Bool
	connectedNetwork     atomic.Pointer[string]
	connectedAccessPoint atomic.Pointer[string]
	affinities           atomic.Pointer[[]string]
	orderedNetworks      atomic.Pointer[[]iwdbus.OrderedNetwork]
	hiddenAPs            atomic.Pointer[[]iwdbus.HiddenAccessPoint]
	subPropsEvent        atomic.Value // iwdbus.StationPropertiesChanged
	connNetEvnt          atomic.Pointer[optStringEvent]
	connAPEvnt           atomic.Pointer[optStringEvent]
	affinityEvnt         atomic.Pointer[stringSliceEvent]

	scanCalled        atomic.Bool
	disconnectCalled  atomic.Bool
	setAffinitiesArg  atomic.Pointer[[]string]
	connectHiddenName atomic.Pointer[string]

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

func (f *fakeIwdbusStation) GetState(ctx context.Context) (iwdbus.StationState, error) {
	if v := f.state.Load(); v != nil {
		return v.(iwdbus.StationState), f.loadErr()
	}
	return iwdbus.StationStateUnknown, f.loadErr()
}

func (f *fakeIwdbusStation) GetScanning(ctx context.Context) (bool, error) {
	return f.scanning.Load(), f.loadErr()
}

func (f *fakeIwdbusStation) GetConnectedNetwork(ctx context.Context) (*string, error) {
	return f.connectedNetwork.Load(), f.loadErr()
}

func (f *fakeIwdbusStation) GetConnectedAccessPoint(ctx context.Context) (*string, error) {
	return f.connectedAccessPoint.Load(), f.loadErr()
}

func (f *fakeIwdbusStation) GetAffinities(ctx context.Context) ([]string, error) {
	if v := f.affinities.Load(); v != nil {
		return *v, f.loadErr()
	}
	return nil, f.loadErr()
}

func (f *fakeIwdbusStation) GetProperties(ctx context.Context) (*iwdbus.StationProperties, error) {
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

func (f *fakeIwdbusStation) Scan(ctx context.Context) error {
	f.scanCalled.Store(true)
	return f.loadErr()
}

func (f *fakeIwdbusStation) GetOrderedNetworks(ctx context.Context) ([]iwdbus.OrderedNetwork, error) {
	if err := f.loadErr(); err != nil {
		return nil, err
	}
	if v := f.orderedNetworks.Load(); v != nil {
		return *v, nil
	}
	return nil, nil
}

func (f *fakeIwdbusStation) SetAffinities(ctx context.Context, paths []string) error {
	if err := f.loadErr(); err != nil {
		return err
	}
	cp := append([]string(nil), paths...)
	f.setAffinitiesArg.Store(&cp)
	return nil
}

func (f *fakeIwdbusStation) Disconnect(ctx context.Context) error {
	f.disconnectCalled.Store(true)
	return f.loadErr()
}

func (f *fakeIwdbusStation) ConnectHiddenNetwork(ctx context.Context, name string) error {
	if err := f.loadErr(); err != nil {
		return err
	}
	f.connectHiddenName.Store(&name)
	return nil
}

func (f *fakeIwdbusStation) GetHiddenAccessPoints(ctx context.Context) ([]iwdbus.HiddenAccessPoint, error) {
	if err := f.loadErr(); err != nil {
		return nil, err
	}
	if v := f.hiddenAPs.Load(); v != nil {
		return *v, nil
	}
	return nil, nil
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

func (f *fakeIwdbusStation) SubscribeConnectedNetworkChanged(ctx context.Context, fn func(*string)) (iwdbus.UnsubscribeFunc, error) {
	if fn == nil {
		return nil, f.loadErr()
	}
	if ev := f.connNetEvnt.Load(); ev != nil {
		fn(ev.v)
	}
	return func() error { return nil }, f.loadErr()
}

func (f *fakeIwdbusStation) SubscribeConnectedAccessPointChanged(ctx context.Context, fn func(*string)) (iwdbus.UnsubscribeFunc, error) {
	if fn == nil {
		return nil, f.loadErr()
	}
	if ev := f.connAPEvnt.Load(); ev != nil {
		fn(ev.v)
	}
	return func() error { return nil }, f.loadErr()
}

func (f *fakeIwdbusStation) SubscribeAffinitiesChanged(ctx context.Context, fn func([]string)) (iwdbus.UnsubscribeFunc, error) {
	if fn == nil {
		return nil, f.loadErr()
	}
	if ev := f.affinityEvnt.Load(); ev != nil {
		fn(ev.v)
	}
	return func() error { return nil }, f.loadErr()
}
