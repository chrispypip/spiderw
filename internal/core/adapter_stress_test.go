//go:build stress

package core

import (
	"context"
	"math/rand"
	"sync"
	"testing"
	"time"
)

func TestStress_Core_Adapter_MixedContexts(t *testing.T) {
	adapter := newTestAdapter(t)

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

			_, _ = adapter.Powered(ctx)
		})
	}

	wg.Wait()
}

func TestStress_Core_Adapter_MixedMethods(t *testing.T) {
	adapter := newTestAdapter(t)

	const N = 6000
	var wg sync.WaitGroup

	for i := range N {
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			switch i % 9 {
			case 0:
				_, _ = adapter.Powered(ctx)
			case 1:
				_ = adapter.SetPowered(ctx, i%2 == 0)
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

func TestStress_Core_Adapter_SubscribePropertiesChanged_Concurrent(t *testing.T) {
	adapter := newTestAdapter(t)

	const N = 4000
	var wg sync.WaitGroup

	for range N {
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_, _ = adapter.SubscribePropertiesChanged(ctx, func(AdapterPropertiesChanged) {})
		})
	}

	wg.Wait()
}

func TestStress_Core_Adapter_SubscribePoweredChanged_Concurrent(t *testing.T) {
	adapter := newTestAdapter(t)

	const N = 4000
	var wg sync.WaitGroup

	for range N {
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_, _ = adapter.SubscribePoweredChanged(ctx, func(bool) {})
		})
	}

	wg.Wait()
}

func TestStress_Core_Adapter_NilReceiver(t *testing.T) {
	var a *Adapter

	const N = 1000
	var wg sync.WaitGroup

	for range N {
		wg.Go(func() {
			_, _ = a.Powered(context.Background())
		})
	}

	wg.Wait()
}
