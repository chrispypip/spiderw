//go:build race

package core

import (
	"context"
	"sync"
	"testing"
)

func TestRace_Core_Station_MixedCalls(t *testing.T) {
	station := newTestStation(t)

	const N = 200
	var wg sync.WaitGroup

	for i := range N {
		wg.Go(func() {
			ctx := context.Background()

			switch i % 4 {
			case 0:
				_, _ = station.State(ctx)
			case 1:
				_, _ = station.Scanning(ctx)
			case 2:
				_, _ = station.ConnectedNetwork(ctx)
			case 3:
				_, _ = station.Properties(ctx)
			}
		})
	}

	wg.Wait()
}

func TestRace_Core_Station_SubscribeStateChanged_ConcurrentCallbacks(t *testing.T) {
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

func TestRace_Core_Station_SubscribeScanningChanged_ConcurrentCallbacks(t *testing.T) {
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
