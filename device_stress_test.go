//go:build stress

package spiderw

import (
	"context"
	"math/rand"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/chrispypip/spiderw/internal/core"
)

func TestStress_Public_Device_MixedMethods(t *testing.T) {
	device := newTestDevice(t)

	// Ensure subscription tests have something to normalize.
	fd := device.core.(*fakeCoreDevice)
	fd.subPropsEvent.Store(core.DevicePropertiesChanged{
		Changed: map[string]any{
			"Powered": true,
			"Mode":    "ap",
		},
		Invalidated: []string{"Address"},
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

			switch i % 9 {
			case 0:
				_, _ = device.Name(ctx)
			case 1:
				_, _ = device.Address(ctx)
			case 2:
				_, _ = device.Powered(ctx)
			case 3:
				_ = device.SetPowered(ctx, i%4 == 0)
			case 4:
				_, _ = device.Mode(ctx)
			case 5:
				_ = device.SetMode(ctx, ModeStation)
			case 6:
				_, _ = device.Adapter(ctx)
			case 7:
				_, _ = device.Properties(ctx)
			default:
				// Mix subscriptions into the load.
				_, _ = device.SubscribeModeChanged(ctx, func(Mode) {})
			}
		})
	}

	wg.Wait()
}

func TestStress_Public_Device_Subscriptions_Concurrent(t *testing.T) {
	device := newTestDevice(t)

	fd := device.core.(*fakeCoreDevice)
	fd.subPropsEvent.Store(core.DevicePropertiesChanged{
		Changed: map[string]any{
			"Powered": true,
			"Mode":    "ap",
		},
		Invalidated: []string{"Address"},
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

			switch i % 3 {
			case 0:
				_, _ = device.SubscribePropertiesChanged(ctx, func(ev DevicePropertiesChanged) {
					// Simulate "normal" user mutation.
					if ev.Changed != nil {
						ev.Changed["k"] = i
					}
				})
			case 1:
				_, _ = device.SubscribePoweredChanged(ctx, func(bool) {})
			default:
				_, _ = device.SubscribeModeChanged(ctx, func(Mode) {})
			}
		})
	}

	wg.Wait()
}

func TestStress_Public_Device_SubscribePropertiesChanged_SlowCallback(t *testing.T) {
	device := newTestDevice(t)

	fd := device.core.(*fakeCoreDevice)
	fd.subPropsEvent.Store(core.DevicePropertiesChanged{
		Changed: map[string]any{
			"Powered": true,
			"Mode":    "ap",
			"Name":    "wlan0",
			"Address": "aa:bb:cc:dd:ee:ff",
		},
		Invalidated: []string{"Address"},
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

			_, _ = device.SubscribePropertiesChanged(ctx, func(ev DevicePropertiesChanged) {
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
