//go:build race

package core

import (
	"context"
	"sync"
	"testing"
)

func TestRace_Core_AccessPoint_MixedMethods(t *testing.T) {
	ap := newTestAccessPoint(t)

	const N = 200
	var wg sync.WaitGroup

	for i := range N {
		wg.Go(func() {
			ctx := context.Background()

			switch i % 9 {
			case 0:
				_, _ = ap.Started(ctx)
			case 1:
				_, _ = ap.Scanning(ctx)
			case 2:
				_, _ = ap.Name(ctx)
			case 3:
				_, _ = ap.Properties(ctx)
			case 4:
				_ = ap.Start(ctx, "MyAP", "s3cretpass")
			case 5:
				_ = ap.StartProfile(ctx, "HomeAP")
			case 6:
				_ = ap.Stop(ctx)
			case 7:
				_ = ap.Scan(ctx)
			case 8:
				_, _ = ap.OrderedNetworks(ctx)
			}
		})
	}

	wg.Wait()
}

func TestRace_Core_AccessPoint_SubscribeConcurrent(t *testing.T) {
	ap := newTestAccessPoint(t)

	const N = 200
	var wg sync.WaitGroup

	for range N {
		wg.Go(func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			_, _ = ap.SubscribeStartedChanged(ctx, func(bool) {})
		})
	}

	wg.Wait()
}

func TestRace_Core_AccessPoint_NilReceiver(t *testing.T) {
	var ap *AccessPoint

	const N = 50
	var wg sync.WaitGroup

	for range N {
		wg.Go(func() {
			_, _ = ap.Started(context.Background())
		})
	}

	wg.Wait()
}
