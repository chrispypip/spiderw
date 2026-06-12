//go:build race

package spiderw

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/chrispypip/spiderw/internal/core"
)

func TestRace_Public_Client_Device(t *testing.T) {
	client := newTestClient(t)
	refs, err := client.Daemon().Devices(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, refs)

	const N = 100
	errCh := make(chan error, N)

	var wg sync.WaitGroup

	for range N {
		wg.Go(func() {
			d, err := client.Device(context.Background(), refs[0].Path)
			if err != nil {
				errCh <- err
				return
			}
			if d == nil {
				errCh <- errors.New("nil device")
				return
			}
			errCh <- nil
		})
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		require.NoError(t, err)
	}
}

func TestRace_Public_Device_ConcurrentAccessors(t *testing.T) {
	device := newTestDevice(t)

	const N = 200
	var wg sync.WaitGroup

	for i := range N {
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			switch i % 6 {
			case 0:
				_, _ = device.Name(ctx)
			case 1:
				_, _ = device.Address(ctx)
			case 2:
				_, _ = device.Powered(ctx)
			case 3:
				_, _ = device.Mode(ctx)
			case 4:
				_, _ = device.Adapter(ctx)
			default:
				_, _ = device.Properties(ctx)
			}
		})
	}

	wg.Wait()
}

func TestRace_Public_Device_SetPoweredAndPowered(t *testing.T) {
	device := newTestDevice(t)

	const N = 200
	var wg sync.WaitGroup

	for i := range N {
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			if i%2 == 0 {
				_ = device.SetPowered(ctx, i%4 == 0)
				return
			}

			_, _ = device.Powered(ctx)
		})
	}

	wg.Wait()
}

func TestRace_Public_Device_SetModeAndMode(t *testing.T) {
	device := newTestDevice(t)

	modes := []Mode{ModeStation, ModeAP, ModeAdHoc}

	const N = 200
	var wg sync.WaitGroup

	for i := range N {
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			if i%2 == 0 {
				_ = device.SetMode(ctx, modes[i%len(modes)])
				return
			}

			_, _ = device.Mode(ctx)
		})
	}

	wg.Wait()
}

func TestRace_Public_Device_SubscribePropertiesChanged_Concurrent(t *testing.T) {
	device := newTestDevice(t)

	// Seed a stable event in the fake core device so every subscription has
	// something to normalize.
	fd := device.core.(*fakeCoreDevice)
	fd.subPropsEvent.Store(core.DevicePropertiesChanged{
		Changed: map[string]any{
			"Powered": true,
			"Mode":    "ap",
		},
		Invalidated: []string{"Address"},
	})

	const N = 200
	var wg sync.WaitGroup

	for range N {
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			_, err := device.SubscribePropertiesChanged(ctx, func(ev DevicePropertiesChanged) {
				// User code is allowed to mutate the map; ensure the wrapper does
				// not share map instances across callbacks.
				if ev.Changed != nil {
					ev.Changed["user-mutation"] = 1
				}
				_ = ev.Invalidated
			})
			require.NoError(t, err)
		})
	}

	wg.Wait()
}

func TestRace_Public_Device_SubscribePoweredChanged_Concurrent(t *testing.T) {
	device := newTestDevice(t)

	fd := device.core.(*fakeCoreDevice)
	fd.subPropsEvent.Store(core.DevicePropertiesChanged{
		Changed: map[string]any{
			"Powered": true,
		},
	})

	const N = 200
	var wg sync.WaitGroup

	for range N {
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			_, err := device.SubscribePoweredChanged(ctx, func(bool) {})
			require.NoError(t, err)
		})
	}

	wg.Wait()
}

func TestRace_Public_Device_SubscribeModeChanged_Concurrent(t *testing.T) {
	device := newTestDevice(t)

	fd := device.core.(*fakeCoreDevice)
	fd.subPropsEvent.Store(core.DevicePropertiesChanged{
		Changed: map[string]any{
			"Mode": "ap",
		},
	})

	const N = 200
	var wg sync.WaitGroup

	for range N {
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			_, err := device.SubscribeModeChanged(ctx, func(Mode) {})
			require.NoError(t, err)
		})
	}

	wg.Wait()
}

func TestRace_Public_Device_ContextCancel(t *testing.T) {
	device := newTestDevice(t)

	const N = 100
	var wg sync.WaitGroup

	for i := range N {
		wg.Go(func() {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			switch i % 5 {
			case 0:
				_, _ = device.Powered(ctx)
			case 1:
				_ = device.SetPowered(ctx, true)
			case 2:
				_, _ = device.Mode(ctx)
			case 3:
				_ = device.SetMode(ctx, ModeStation)
			default:
				_, _ = device.SubscribeModeChanged(ctx, func(Mode) {})
			}
		})
	}

	wg.Wait()
}
