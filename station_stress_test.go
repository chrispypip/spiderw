//go:build stress

package spiderw

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/chrispypip/spiderw/internal/core"
)

func newStressStation() *Station {
	f := &fakeCoreStation{}
	f.state.Store(core.StationStateConnected)
	f.scanning.Store(true)
	cn := "/net/connman/iwd/0/3/ssid_psk"
	ap := "/net/connman/iwd/0/3/ssid_psk/aabbccddeeff"
	f.connectedNetwork.Store(&cn)
	f.connectedAccessPoint.Store(&ap)
	aff := []string{ap}
	f.affinities.Store(&aff)
	nets := []core.OrderedNetwork{{Network: cn, SignalStrength: -6000}}
	f.orderedNetworks.Store(&nets)
	aps := []core.HiddenAccessPoint{{Address: "aa:bb:cc:dd:ee:ff", SignalStrength: -6000, Type: core.NetworkTypePSK}}
	f.hiddenAPs.Store(&aps)
	f.subPropsEvent.Store(core.StationPropertiesChanged{
		Changed: map[string]any{"State": "connected", "Scanning": true},
	})
	return newStation(f, "/net/connman/iwd/0/3", "wlan0")
}

func TestStress_Public_Station_MixedMethods(t *testing.T) {
	station := newStressStation()

	const N = 6000
	var wg sync.WaitGroup

	for i := range N {
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			switch i % 11 {
			case 0:
				_, _ = station.State(ctx)
			case 1:
				_, _ = station.Scanning(ctx)
			case 2:
				_, _ = station.ConnectedNetwork(ctx)
			case 3:
				_, _ = station.ConnectedAccessPoint(ctx)
			case 4:
				_, _ = station.Affinities(ctx)
			case 5:
				_, _ = station.Properties(ctx)
			case 6:
				_ = station.Scan(ctx)
			case 7:
				_, _ = station.OrderedNetworks(ctx)
			case 8:
				_ = station.SetAffinities(ctx, []string{"/net/connman/iwd/0/3/ssid_psk/aabbccddeeff"})
			case 9:
				_ = station.Disconnect(ctx)
			case 10:
				_, _ = station.HiddenAccessPoints(ctx)
			}
		})
	}

	wg.Wait()
}

func TestStress_Public_Station_Subscriptions_Concurrent(t *testing.T) {
	station := newStressStation()

	const N = 6000
	var wg sync.WaitGroup

	for i := range N {
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			switch i % 3 {
			case 0:
				_, _ = station.SubscribeStateChanged(ctx, func(StationState) {})
			case 1:
				_, _ = station.SubscribeScanningChanged(ctx, func(bool) {})
			case 2:
				_, _ = station.SubscribePropertiesChanged(ctx, func(StationPropertiesChanged) {})
			}
		})
	}

	wg.Wait()
}

func TestStress_Public_Station_NilReceiver(t *testing.T) {
	var s *Station

	const N = 1000
	var wg sync.WaitGroup

	for range N {
		wg.Go(func() {
			_, _ = s.State(context.Background())
		})
	}

	wg.Wait()
}
