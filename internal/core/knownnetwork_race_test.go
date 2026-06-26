//go:build race

package core

import (
	"context"
	"sync"
	"testing"
)

func TestRace_Core_KnownNetwork_SetAutoConnectConcurrentWithGet(t *testing.T) {
	known := newTestKnownNetwork(t)

	const N = 200
	var wg sync.WaitGroup

	for i := range N {
		wg.Go(func() {
			ctx := context.Background()
			if i%2 == 0 {
				_ = known.SetAutoConnect(ctx, i%4 == 0)
			} else {
				_, _ = known.AutoConnect(ctx)
			}
		})
	}

	wg.Wait()
}

func TestRace_Core_KnownNetwork_MixedCalls(t *testing.T) {
	known := newTestKnownNetwork(t)

	const N = 200
	var wg sync.WaitGroup

	for i := range N {
		wg.Go(func() {
			ctx := context.Background()

			switch i % 8 {
			case 0:
				_, _ = known.Name(ctx)
			case 1:
				_, _ = known.Type(ctx)
			case 2:
				_, _ = known.Hidden(ctx)
			case 3:
				_, _ = known.LastConnectedTime(ctx)
			case 4:
				_, _ = known.AutoConnect(ctx)
			case 5:
				_ = known.SetAutoConnect(ctx, i%3 == 0)
			case 6:
				_, _ = known.Properties(ctx)
			case 7:
				_ = known.Forget(ctx)
			}
		})
	}

	wg.Wait()
}

func TestRace_Core_KnownNetwork_SubscribeAutoConnectChanged_ConcurrentCallbacks(t *testing.T) {
	known := newTestKnownNetwork(t)

	const N = 200
	var wg sync.WaitGroup

	for range N {
		wg.Go(func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			_, _ = known.SubscribeAutoConnectChanged(ctx, func(bool) {})
		})
	}

	wg.Wait()
}

func TestRace_Core_KnownNetwork_SubscribePropertiesChanged_ConcurrentCallbacks(t *testing.T) {
	known := newTestKnownNetwork(t)

	const N = 200
	var wg sync.WaitGroup

	for range N {
		wg.Go(func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			_, _ = known.SubscribePropertiesChanged(ctx, func(KnownNetworkPropertiesChanged) {})
		})
	}

	wg.Wait()
}

func TestRace_Core_KnownNetwork_NilReceiver(t *testing.T) {
	var k *KnownNetwork

	const N = 50
	var wg sync.WaitGroup

	for range N {
		wg.Go(func() {
			_, _ = k.AutoConnect(context.Background())
		})
	}

	wg.Wait()
}
