//go:build stress

package core

import (
	"context"
	"math/rand"
	"sync"
	"testing"
	"time"
)

func TestStress_Core_Station_MixedMethods(t *testing.T) {
	station := newTestStation(t)

	const N = 6000
	var wg sync.WaitGroup

	for i := range N {
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

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

func TestStress_Core_Station_MixedContexts(t *testing.T) {
	station := newTestStation(t)

	const N = 800
	var wg sync.WaitGroup

	for range N {
		wg.Go(func() {
			var ctx context.Context
			var cancel context.CancelFunc

			switch rand.Intn(3) {
			case 0:
				ctx, cancel = context.WithTimeout(context.Background(), time.Millisecond)
			case 1:
				ctx, cancel = context.WithCancel(context.Background())
				cancel()
			default:
				ctx, cancel = context.WithTimeout(context.Background(), time.Second)
			}
			defer cancel()

			_, _ = station.State(ctx)
		})
	}

	wg.Wait()
}

func TestStress_Core_Station_SubscribeStateChanged_Concurrent(t *testing.T) {
	station := newTestStation(t)

	const N = 4000
	var wg sync.WaitGroup

	for range N {
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_, _ = station.SubscribeStateChanged(ctx, func(StationState) {})
		})
	}

	wg.Wait()
}

func TestStress_Core_Station_SubscribeScanningChanged_Concurrent(t *testing.T) {
	station := newTestStation(t)

	const N = 4000
	var wg sync.WaitGroup

	for range N {
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_, _ = station.SubscribeScanningChanged(ctx, func(bool) {})
		})
	}

	wg.Wait()
}

func TestStress_Core_Station_SubscribePropertiesChanged_Concurrent(t *testing.T) {
	station := newTestStation(t)

	const N = 4000
	var wg sync.WaitGroup

	for range N {
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_, _ = station.SubscribePropertiesChanged(ctx, func(StationPropertiesChanged) {})
		})
	}

	wg.Wait()
}

func TestStress_Core_Station_NilReceiver(t *testing.T) {
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
