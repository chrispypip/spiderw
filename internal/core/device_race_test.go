//go:build race

package core

import (
	"context"
	"sync"
	"testing"
)

func TestRace_Core_Device_SetPowered_ConcurrentWithGet(t *testing.T) {
	device := newTestDevice(t)

	const N = 200
	var wg sync.WaitGroup

	for i := range N {
		wg.Go(func() {
			ctx := context.Background()
			if i%2 == 0 {
				_ = device.SetPowered(ctx, true)
			} else {
				_, _ = device.Powered(ctx)
			}
		})
	}

	wg.Wait()
}

func TestRace_Core_Device_SetMode_ConcurrentWithGet(t *testing.T) {
	device := newTestDevice(t)

	const N = 200
	var wg sync.WaitGroup

	for i := range N {
		wg.Go(func() {
			ctx := context.Background()
			if i%2 == 0 {
				_ = device.SetMode(ctx, ModeAP)
			} else {
				_, _ = device.Mode(ctx)
			}
		})
	}

	wg.Wait()
}

func TestRace_Core_Device_MixedMethods(t *testing.T) {
	device := newTestDevice(t)

	const N = 200
	var wg sync.WaitGroup

	for i := range N {
		wg.Go(func() {
			ctx := context.Background()

			switch i % 8 {
			case 0:
				_, _ = device.Name(ctx)
			case 1:
				_, _ = device.Address(ctx)
			case 2:
				_, _ = device.Powered(ctx)
			case 3:
				_ = device.SetPowered(ctx, i%3 == 0)
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

func TestRace_Core_Device_SubscribePoweredChanged_Concurrent(t *testing.T) {
	device := newTestDevice(t)

	const N = 200
	var wg sync.WaitGroup

	for range N {
		wg.Go(func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			_, _ = device.SubscribePoweredChanged(ctx, func(bool) {})
		})
	}

	wg.Wait()
}

func TestRace_Core_Device_SubscribeModeChanged_Concurrent(t *testing.T) {
	device := newTestDevice(t)

	const N = 200
	var wg sync.WaitGroup

	for range N {
		wg.Go(func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			_, _ = device.SubscribeModeChanged(ctx, func(Mode) {})
		})
	}

	wg.Wait()
}

func TestRace_Core_Device_SubscribePropertiesChanged_Concurrent(t *testing.T) {
	device := newTestDevice(t)

	const N = 200
	var wg sync.WaitGroup

	for range N {
		wg.Go(func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			_, _ = device.SubscribePropertiesChanged(ctx, func(DevicePropertiesChanged) {})
		})
	}

	wg.Wait()
}

func TestRace_Core_Device_NilReceiver(t *testing.T) {
	var d *Device

	const N = 50
	var wg sync.WaitGroup

	for range N {
		wg.Go(func() {
			_, _ = d.Powered(context.Background())
		})
	}

	wg.Wait()
}
