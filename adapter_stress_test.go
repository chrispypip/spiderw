//go:build stress

package spiderw

import (
	"context"
	"math/rand"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/chrispypip/spiderw/internal/core"
)

func TestStress_Public_Adapter_MixedOperations(t *testing.T) {
	adapter := newTestAdapter(t)

	// Ensure subscription tests have something to normalize.
	fa := adapter.core.(*fakeCoreAdapter)
	fa.subPropsEvent.Store(core.AdapterPropertiesChanged{
		Changed: map[string]any{
			"Powered": true,
			"Name":    "phy0",
			"Model":   "MockModel",
			"Vendor":  "MockVendor",
		},
		Invalidated: []string{"Model"},
	})

	const N = 2000
	var wg sync.WaitGroup

	sem := make(chan struct{}, runtime.GOMAXPROCS(0)*2)

	for i := range N {
		sem <- struct{}{}
		wg.Go(func() {
			defer func() { <-sem }()
			time.Sleep(time.Microsecond * time.Duration(rand.Intn(200)))

			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			switch i % 10 {
			case 0:
				_, _ = adapter.Powered(ctx)
			case 1:
				_ = adapter.SetPowered(ctx, i%4 == 0)
			case 2:
				_, _ = adapter.Name(ctx)
			case 3:
				_, _ = adapter.Model(ctx)
			case 4:
				_, _ = adapter.Vendor(ctx)
			case 5:
				_, _ = adapter.SupportedModes(ctx)
			case 6:
				_, _ = adapter.SupportsStation(ctx)
			case 7:
				_, _ = adapter.SupportsAP(ctx)
			case 8:
				_, _ = adapter.SupportsAdHoc(ctx)
			default:
				// Mix subscriptions into the load.
				_, _ = adapter.SubscribePoweredChanged(ctx, func(bool) {})
			}
		})
	}

	wg.Wait()
}

func TestStress_Public_Adapter_Subscriptions_Concurrent(t *testing.T) {
	adapter := newTestAdapter(t)

	// Seed a stable event payload.
	fa := adapter.core.(*fakeCoreAdapter)
	fa.subPropsEvent.Store(core.AdapterPropertiesChanged{
		Changed: map[string]any{
			"Powered": true,
			"Name":    "phy0",
		},
		Invalidated: []string{"Model"},
	})

	const N = 3000
	var wg sync.WaitGroup

	sem := make(chan struct{}, runtime.GOMAXPROCS(0)*2)

	for i := range N {
		sem <- struct{}{}
		wg.Go(func() {
			defer func() { <-sem }()
			time.Sleep(time.Microsecond * time.Duration(rand.Intn(200)))

			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			if i%2 == 0 {
				_, _ = adapter.SubscribePropertiesChanged(ctx, func(ev AdapterPropertiesChanged) {
					// Simulate "normal" user mutation.
					if ev.Changed != nil {
						ev.Changed["k"] = i
					}
				})
				return
			}

			_, _ = adapter.SubscribePoweredChanged(ctx, func(bool) {})
		})
	}

	wg.Wait()
}

func TestStress_Public_Adapter_SubscribePropertiesChanged_SlowCallback(t *testing.T) {
	adapter := newTestAdapter(t)

	const size = 64

	// A larger payload stresses the wrapper's map copying.
	base := make(map[string]any, size)
	for i := range size {
		base["k"+strconv.Itoa(i)] = i
	}
	base["Powered"] = true
	base["Name"] = "phy0"

	fa := adapter.core.(*fakeCoreAdapter)
	fa.subPropsEvent.Store(core.AdapterPropertiesChanged{
		Changed:     base,
		Invalidated: []string{"Model", "Vendor"},
	})

	const N = 1000
	var wg sync.WaitGroup

	sem := make(chan struct{}, runtime.GOMAXPROCS(0)*2)
	for i := range N {
		sem <- struct{}{}
		wg.Go(func() {
			defer func() { <-sem }()

			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			_, _ = adapter.SubscribePropertiesChanged(ctx, func(ev AdapterPropertiesChanged) {
				// Slow user callback: intentionally blocks to mimic expensive
				// consumer logic. This should not create internal sharing races.
				time.Sleep(time.Microsecond * time.Duration(250+rand.Intn(750)))

				// Mutate the received map to validate that the wrapper does not
				// reuse internal map instances.
				if ev.Changed != nil {
					ev.Changed["slow"] = i
					delete(ev.Changed, "Powered")
				}
			})
		})
	}

	wg.Wait()
}
