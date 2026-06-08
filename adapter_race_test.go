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

func TestRace_Public_Client_Adapter(t *testing.T) {
	client := newTestClient(t)
	refs, err := client.Daemon().Adapters(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, refs)

	const N = 100
	errCh := make(chan error, N)

	var wg sync.WaitGroup
	wg.Add(N)

	for range N {
		go func() {
			defer wg.Done()
			a, err := client.Adapter(context.Background(), refs[0].Path)
			if err != nil {
				errCh <- err
				return
			}
			if a == nil {
				errCh <- errors.New("nil adapter")
				return
			}
			errCh <- nil
		}()
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		require.NoError(t, err)
	}
}

func TestRace_Public_Adapter_ConcurrentAccessors(t *testing.T) {
	adapter := newTestAdapter(t)

	const N = 200
	var wg sync.WaitGroup
	wg.Add(N)

	for i := range N {
		go func(i int) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			switch i % 8 {
			case 0:
				_, _ = adapter.Powered(ctx)
			case 1:
				_, _ = adapter.Name(ctx)
			case 2:
				_, _ = adapter.Model(ctx)
			case 3:
				_, _ = adapter.Vendor(ctx)
			case 4:
				_, _ = adapter.SupportedModes(ctx)
			case 5:
				_, _ = adapter.SupportsStation(ctx)
			case 6:
				_, _ = adapter.SupportsAP(ctx)
			default:
				_, _ = adapter.SupportsAdHoc(ctx)
			}
		}(i)
	}

	wg.Wait()
}

func TestRace_Public_Adapter_SetPoweredAndPowered(t *testing.T) {
	adapter := newTestAdapter(t)

	const N = 200
	var wg sync.WaitGroup
	wg.Add(N)

	for i := range N {
		go func(i int) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			if i%2 == 0 {
				_ = adapter.SetPowered(ctx, i%4 == 0)
				return
			}

			_, _ = adapter.Powered(ctx)
		}(i)
	}

	wg.Wait()
}

func TestRace_Public_Adapter_SubscribePropertiesChanged_Concurrent(t *testing.T) {
	adapter := newTestAdapter(t)

	// Seed a stable event in the fake core adapter so every subscription
	// has something to normalize.
	fa := adapter.core.(*fakeCoreAdapter)
	fa.subPropsEvent.Store(core.AdapterPropertiesChanged{
		Changed: map[string]any{
			"Powered": true,
			"Name":    "phy0",
			"Model":   "MockModel",
		},
		Invalidated: []string{"Model"},
	})

	const N = 200
	var wg sync.WaitGroup
	wg.Add(N)

	for range N {
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			_, err := adapter.SubscribePropertiesChanged(ctx, func(ev AdapterPropertiesChanged) {
				// User code is allowed to mutate the map; ensure the wrapper
				// does not share map instances across callbacks.
				if ev.Changed != nil {
					ev.Changed["user-mutation"] = 1
				}
				_ = ev.Invalidated
			})
			require.NoError(t, err)
		}()
	}

	wg.Wait()
}

func TestRace_Public_Adapter_SubscribePoweredChanged_Concurrent(t *testing.T) {
	adapter := newTestAdapter(t)

	// Seed a stable event so SubscribePoweredChanged can derive a boolean.
	fa := adapter.core.(*fakeCoreAdapter)
	fa.subPropsEvent.Store(core.AdapterPropertiesChanged{
		Changed: map[string]any{
			"Powered": true,
		},
	})

	const N = 200
	var wg sync.WaitGroup
	wg.Add(N)

	for range N {
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			_, err := adapter.SubscribePoweredChanged(ctx, func(bool) {})
			require.NoError(t, err)
		}()
	}

	wg.Wait()
}

func TestRace_Public_Adapter_ContextCancel(t *testing.T) {
	adapter := newTestAdapter(t)

	const N = 100
	var wg sync.WaitGroup
	wg.Add(N)

	for i := range N {
		go func(i int) {
			defer wg.Done()
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			switch i % 4 {
			case 0:
				_, _ = adapter.Powered(ctx)
			case 1:
				_ = adapter.SetPowered(ctx, true)
			case 2:
				_, _ = adapter.SupportedModes(ctx)
			default:
				_, _ = adapter.SubscribePoweredChanged(ctx, func(bool) {})
			}
		}(i)
	}

	wg.Wait()
}
