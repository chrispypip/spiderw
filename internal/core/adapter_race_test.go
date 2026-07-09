//go:build race

package core

import (
	"context"
	"sync"
	"testing"
)

func TestRace_Core_Adapter_SetPowered_ConcurrentWithGet(t *testing.T) {
	adapter := newTestAdapter(t)

	const N = 200
	var wg sync.WaitGroup

	for i := range N {
		wg.Go(func() {
			ctx := context.Background()
			if i%2 == 0 {
				_ = adapter.SetPowered(ctx, true)
			} else {
				_, _ = adapter.Powered(ctx)
			}
		})
	}

	wg.Wait()
}

func TestRace_Core_Adapter_MixedMethods(t *testing.T) {
	adapter := newTestAdapter(t)

	const N = 200
	var wg sync.WaitGroup

	for i := range N {
		wg.Go(func() {
			ctx := context.Background()

			switch i % 9 {
			case 0:
				_, _ = adapter.Powered(ctx)
			case 1:
				_ = adapter.SetPowered(ctx, i%3 == 0)
			case 2:
				_, _ = adapter.Name(ctx)
			case 3:
				_, _ = adapter.Model(ctx)
			case 4:
				_, _ = adapter.Vendor(ctx)
			case 5:
				_, _ = adapter.SupportedModes(ctx)
			case 6:
				_, _ = adapter.SupportsMode(ctx, ModeStation)
			case 7:
				_, _ = adapter.SupportsAP(ctx)
			case 8:
				_, _ = adapter.SupportsAdHoc(ctx)
			}
		})
	}

	wg.Wait()
}

func TestRace_Core_Adapter_SubscribePoweredChanged_Concurrent(t *testing.T) {
	// This uses a fake raw that invokes the callback; we amplify concurrency
	// at the call site to ensure the wrapper remains race-safe.
	adapter := newTestAdapter(t)

	const N = 200
	var wg sync.WaitGroup

	for range N {
		wg.Go(func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			_, _ = adapter.SubscribePoweredChanged(ctx, func(bool) {
				// intentionally empty
			})
		})
	}

	wg.Wait()
}

func TestRace_Core_Adapter_SubscribePropertiesChanged_Concurrent(t *testing.T) {
	adapter := newTestAdapter(t)

	const N = 200
	var wg sync.WaitGroup

	for range N {
		wg.Go(func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			_, _ = adapter.SubscribePropertiesChanged(ctx, func(AdapterPropertiesChanged) {
				// intentionally empty
			})
		})
	}

	wg.Wait()
}

func TestRace_Core_Adapter_NilReceiver(t *testing.T) {
	var a *Adapter

	const N = 50
	var wg sync.WaitGroup

	for range N {
		wg.Go(func() {
			_, _ = a.Powered(context.Background())
		})
	}

	wg.Wait()
}
