//go:build race

package core

import (
	"context"
	"sync"
	"testing"

	"github.com/godbus/dbus/v5"

	"github.com/chrispypip/spiderw/internal/iwdbus"
)

func TestRace_Core_Station_MixedMethods(t *testing.T) {
	station := newTestStation(t)

	const N = 200
	var wg sync.WaitGroup

	for i := range N {
		wg.Go(func() {
			ctx := context.Background()

			switch i % 10 {
			case 0:
				_, _ = station.State(ctx)
			case 1:
				_, _ = station.Scanning(ctx)
			case 2:
				_, _ = station.ConnectedNetwork(ctx)
			case 3:
				_, _ = station.Properties(ctx)
			case 4:
				_ = station.Scan(ctx)
			case 5:
				_, _ = station.OrderedNetworks(ctx)
			case 6:
				_ = station.SetAffinities(ctx, []string{"/net/connman/iwd/phy0/wlan0/aaa"})
			case 7:
				_ = station.Disconnect(ctx)
			case 8:
				_ = station.ConnectHiddenNetwork(ctx, "HiddenNet")
			case 9:
				_, _ = station.HiddenAccessPoints(ctx)
			}
		})
	}

	wg.Wait()
}

func TestRace_Core_Station_SubscribeStateChanged_Concurrent(t *testing.T) {
	station := newTestStation(t)

	const N = 200
	var wg sync.WaitGroup

	for range N {
		wg.Go(func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			_, _ = station.SubscribeStateChanged(ctx, func(StationState) {})
		})
	}

	wg.Wait()
}

func TestRace_Core_Station_SubscribeScanningChanged_Concurrent(t *testing.T) {
	station := newTestStation(t)

	const N = 200
	var wg sync.WaitGroup

	for range N {
		wg.Go(func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			_, _ = station.SubscribeScanningChanged(ctx, func(bool) {})
		})
	}

	wg.Wait()
}

func TestRace_Core_Station_SubscribePropertiesChanged_Concurrent(t *testing.T) {
	station := newTestStation(t)

	f := station.raw.(*fakeIwdbusStation)
	f.subPropsEvent.Store(iwdbus.StationPropertiesChanged{
		Changed:     map[string]dbus.Variant{"State": dbus.MakeVariant("roaming")},
		Invalidated: []string{"ConnectedNetwork"},
	})

	const N = 200
	var wg sync.WaitGroup

	for range N {
		wg.Go(func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			_, _ = station.SubscribePropertiesChanged(ctx, func(ev StationPropertiesChanged) {
				// The core wrapper must clone per callback; mutating here must not
				// race another concurrent callback.
				if ev.Changed != nil {
					ev.Changed["user-mutation"] = 1
				}
				_ = ev.Invalidated
			})
		})
	}

	wg.Wait()
}

func TestRace_Core_Station_NilReceiver(t *testing.T) {
	var s *Station

	const N = 50
	var wg sync.WaitGroup

	for range N {
		wg.Go(func() {
			_, _ = s.State(context.Background())
		})
	}

	wg.Wait()
}
