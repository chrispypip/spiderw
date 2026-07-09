//go:build stress

package core

import (
	"context"
	"math/rand"
	"sync"
	"testing"
	"time"
)

func TestStress_Core_Device_MixedContexts(t *testing.T) {
	device := newTestDevice(t)

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

			_, _ = device.Powered(ctx)
		})
	}

	wg.Wait()
}

func TestStress_Core_Device_MixedMethods(t *testing.T) {
	device := newTestDevice(t)

	const N = 6000
	var wg sync.WaitGroup

	for i := range N {
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			switch i % 8 {
			case 0:
				_, _ = device.Name(ctx)
			case 1:
				_, _ = device.Address(ctx)
			case 2:
				_, _ = device.Powered(ctx)
			case 3:
				_ = device.SetPowered(ctx, i%2 == 0)
			case 4:
				_, _ = device.Mode(ctx)
			case 5:
				_ = device.SetMode(ctx, ModeStation)
			case 6:
				_, _ = device.Adapter(ctx)
			case 7:
				_, _ = device.Properties(ctx)
			}
		})
	}

	wg.Wait()
}

func TestStress_Core_Device_SubscribePropertiesChanged_Concurrent(t *testing.T) {
	device := newTestDevice(t)

	const N = 4000
	var wg sync.WaitGroup

	for range N {
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_, _ = device.SubscribePropertiesChanged(ctx, func(DevicePropertiesChanged) {})
		})
	}

	wg.Wait()
}

func TestStress_Core_Device_SubscribeModeChanged_Concurrent(t *testing.T) {
	device := newTestDevice(t)

	const N = 4000
	var wg sync.WaitGroup

	for range N {
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_, _ = device.SubscribeModeChanged(ctx, func(Mode) {})
		})
	}

	wg.Wait()
}

func TestStress_Core_Device_NilReceiver(t *testing.T) {
	var d *Device

	const N = 1000
	var wg sync.WaitGroup

	for range N {
		wg.Go(func() {
			_, _ = d.Powered(context.Background())
		})
	}

	wg.Wait()
}
