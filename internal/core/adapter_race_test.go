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
	wg.Add(N)

	for i := range N {
		go func(i int) {
			defer wg.Done()
			ctx := context.Background()
			if i%2 == 0 {
				_ = adapter.SetPowered(ctx, true)
			} else {
				_, _ = adapter.Powered(ctx)
			}
		}(i)
	}

	wg.Wait()
}

func TestRace_Core_Adapter_MixedCalls(t *testing.T) {
	adapter := newTestAdapter(t)

	const N = 200
	var wg sync.WaitGroup
	wg.Add(N)

	for i := range N {
		go func(i int) {
			defer wg.Done()
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
				_, _ = adapter.SupportsMode(ctx, AdapterModeStation)
			case 7:
				_, _ = adapter.SupportsAP(ctx)
			case 8:
				_, _ = adapter.SupportsAdHoc(ctx)
			}
		}(i)
	}

	wg.Wait()
}

func TestRace_Core_Adapter_SubscribePoweredChanged_ConcurrentCallbacks(t *testing.T) {
	// This uses a fake raw that invokes the callback; we amplify concurrency
	// at the call site to ensure the wrapper remains race-safe.
	adapter := newTestAdapter(t)

	const N = 200
	var wg sync.WaitGroup
	wg.Add(N)

	for i := range N {
		go func(i int) {
			defer wg.Done()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			_, _ = adapter.SubscribePoweredChanged(ctx, func(bool) {
				// intentionally empty
			})
		}(i)
	}

	wg.Wait()
}

func TestRace_Core_Adapter_SubscribePropertiesChanged_ConcurrentCallbacks(t *testing.T) {
	adapter := newTestAdapter(t)

	const N = 200
	var wg sync.WaitGroup
	wg.Add(N)

	for range N {
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			_, _ = adapter.SubscribePropertiesChanged(ctx, func(AdapterPropertiesChanged) {
				// intentionally empty
			})
		}()
	}

	wg.Wait()
}

func TestRace_Core_Adapter_NilReceiver(t *testing.T) {
	var a *Adapter

	const N = 50
	var wg sync.WaitGroup
	wg.Add(N)

	for range N {
		go func() {
			defer wg.Done()
			_, _ = a.Powered(context.Background())
		}()
	}

	wg.Wait()
}
