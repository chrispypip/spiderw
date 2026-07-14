//go:build unit || race || stress

package spiderw

import (
	"context"
	"sync/atomic"

	"github.com/chrispypip/spiderw/internal/core"
)

type fakeCoreStationError struct {
	err error
}

// optStringEvent and stringSliceEvent wrap subscribe payloads so a fake can
// distinguish "no event configured" (nil pointer) from "an event carrying nil"
// (e.g. ConnectedNetwork when disconnected).
type optStringEvent struct{ v *string }

type stringSliceEvent struct{ v []string }

type fakeCoreStation struct {
	state                atomic.Value // core.StationState
	scanning             atomic.Bool
	connectedNetwork     atomic.Pointer[string]
	connectedAccessPoint atomic.Pointer[string]
	affinities           atomic.Pointer[[]string]
	orderedNetworks      atomic.Pointer[[]core.OrderedNetwork]
	hiddenAPs            atomic.Pointer[[]core.HiddenAccessPoint]
	subPropsEvent        atomic.Value // core.StationPropertiesChanged
	connNetEvnt          atomic.Pointer[optStringEvent]
	connAPEvnt           atomic.Pointer[optStringEvent]
	affinityEvnt         atomic.Pointer[stringSliceEvent]

	scanCalled        atomic.Bool
	disconnectCalled  atomic.Bool
	setAffinitiesArg  atomic.Pointer[[]string]
	connectHiddenName atomic.Pointer[string]

	err atomic.Pointer[fakeCoreStationError]
}

func (f *fakeCoreStation) setErr(err error) {
	if err == nil {
		f.err.Store(nil)
		return
	}
	f.err.Store(&fakeCoreStationError{err: err})
}

func (f *fakeCoreStation) loadErr() error {
	box := f.err.Load()
	if box == nil {
		return nil
	}
	return box.err
}

func (f *fakeCoreStation) State(ctx context.Context) (core.StationState, error) {
	if v := f.state.Load(); v != nil {
		return v.(core.StationState), f.loadErr()
	}
	return core.StationStateUnknown, f.loadErr()
}

func (f *fakeCoreStation) Scanning(ctx context.Context) (bool, error) {
	return f.scanning.Load(), f.loadErr()
}

func (f *fakeCoreStation) ConnectedNetwork(ctx context.Context) (*string, error) {
	return f.connectedNetwork.Load(), f.loadErr()
}

func (f *fakeCoreStation) ConnectedAccessPoint(ctx context.Context) (*string, error) {
	return f.connectedAccessPoint.Load(), f.loadErr()
}

func (f *fakeCoreStation) Affinities(ctx context.Context) ([]string, error) {
	if v := f.affinities.Load(); v != nil {
		return *v, f.loadErr()
	}
	return nil, f.loadErr()
}

func (f *fakeCoreStation) Properties(ctx context.Context) (*core.StationProperties, error) {
	if err := f.loadErr(); err != nil {
		return nil, err
	}

	props := &core.StationProperties{
		Scanning:             f.scanning.Load(),
		ConnectedNetwork:     f.connectedNetwork.Load(),
		ConnectedAccessPoint: f.connectedAccessPoint.Load(),
	}
	if v := f.state.Load(); v != nil {
		props.State = v.(core.StationState)
	}
	if v := f.affinities.Load(); v != nil {
		props.Affinities = *v
	}
	return props, nil
}

func (f *fakeCoreStation) Scan(ctx context.Context) error {
	f.scanCalled.Store(true)
	return f.loadErr()
}

func (f *fakeCoreStation) OrderedNetworks(ctx context.Context) ([]core.OrderedNetwork, error) {
	if err := f.loadErr(); err != nil {
		return nil, err
	}
	if v := f.orderedNetworks.Load(); v != nil {
		return *v, nil
	}
	return nil, nil
}

func (f *fakeCoreStation) SetAffinities(ctx context.Context, paths []string) error {
	if err := f.loadErr(); err != nil {
		return err
	}
	cp := append([]string(nil), paths...)
	f.setAffinitiesArg.Store(&cp)
	return nil
}

func (f *fakeCoreStation) Disconnect(ctx context.Context) error {
	f.disconnectCalled.Store(true)
	return f.loadErr()
}

func (f *fakeCoreStation) ConnectHiddenNetwork(ctx context.Context, name string) error {
	if err := f.loadErr(); err != nil {
		return err
	}
	f.connectHiddenName.Store(&name)
	return nil
}

func (f *fakeCoreStation) HiddenAccessPoints(ctx context.Context) ([]core.HiddenAccessPoint, error) {
	if err := f.loadErr(); err != nil {
		return nil, err
	}
	if v := f.hiddenAPs.Load(); v != nil {
		return *v, nil
	}
	return nil, nil
}

func (f *fakeCoreStation) SubscribePropertiesChanged(ctx context.Context, fn func(core.StationPropertiesChanged)) (core.UnsubscribeFunc, error) {
	if fn == nil {
		return nil, f.loadErr()
	}

	if v := f.subPropsEvent.Load(); v != nil {
		fn(v.(core.StationPropertiesChanged))
	}

	return func() error { return nil }, f.loadErr()
}

func (f *fakeCoreStation) SubscribeStateChanged(ctx context.Context, fn func(core.StationState)) (core.UnsubscribeFunc, error) {
	if fn == nil {
		return nil, f.loadErr()
	}

	if v := f.subPropsEvent.Load(); v != nil {
		props := v.(core.StationPropertiesChanged)
		if st, ok := props.Changed["State"]; ok {
			// Deliver the raw state as the core layer would; the public wrapper is
			// responsible for validating and dropping unrecognized states.
			if s, ok := st.(string); ok {
				fn(core.StationState(s))
			}
		}
	}

	return func() error { return nil }, f.loadErr()
}

func (f *fakeCoreStation) SubscribeScanningChanged(ctx context.Context, fn func(bool)) (core.UnsubscribeFunc, error) {
	if fn == nil {
		return nil, f.loadErr()
	}

	if v := f.subPropsEvent.Load(); v != nil {
		props := v.(core.StationPropertiesChanged)
		if sc, ok := props.Changed["Scanning"]; ok {
			if b, ok := sc.(bool); ok {
				fn(b)
			}
		}
	}

	return func() error { return nil }, f.loadErr()
}

func (f *fakeCoreStation) SubscribeConnectedNetworkChanged(ctx context.Context, fn func(*string)) (core.UnsubscribeFunc, error) {
	if fn == nil {
		return nil, f.loadErr()
	}
	if ev := f.connNetEvnt.Load(); ev != nil {
		fn(ev.v)
	}
	return func() error { return nil }, f.loadErr()
}

func (f *fakeCoreStation) SubscribeConnectedAccessPointChanged(ctx context.Context, fn func(*string)) (core.UnsubscribeFunc, error) {
	if fn == nil {
		return nil, f.loadErr()
	}
	if ev := f.connAPEvnt.Load(); ev != nil {
		fn(ev.v)
	}
	return func() error { return nil }, f.loadErr()
}

func (f *fakeCoreStation) SubscribeAffinitiesChanged(ctx context.Context, fn func([]string)) (core.UnsubscribeFunc, error) {
	if fn == nil {
		return nil, f.loadErr()
	}
	if ev := f.affinityEvnt.Load(); ev != nil {
		fn(ev.v)
	}
	return func() error { return nil }, f.loadErr()
}
