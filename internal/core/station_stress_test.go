//go:build stress

package core

import (
	"context"
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

func TestStress_Core_Station_SubscribeStateChanged_Fanout(t *testing.T) {
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

func TestStress_Core_Station_Nil(t *testing.T) {
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
