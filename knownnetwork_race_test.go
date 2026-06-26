//go:build race

package spiderw

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRace_Public_Client_KnownNetwork(t *testing.T) {
	client := newTestKnownNetworkClient(t)
	refs, err := client.Daemon().KnownNetworks(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, refs)

	const N = 100
	errCh := make(chan error, N)

	var wg sync.WaitGroup

	for range N {
		wg.Go(func() {
			k, err := client.KnownNetwork(context.Background(), refs[0].Path)
			if err != nil {
				errCh <- err
				return
			}
			if k == nil {
				errCh <- errors.New("nil known network")
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

func TestRace_Public_KnownNetwork_ConcurrentAccessors(t *testing.T) {
	known := newTestKnownNetwork(t)

	const N = 200
	var wg sync.WaitGroup

	for i := range N {
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			switch i % 6 {
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
			default:
				_, _ = known.Properties(ctx)
			}
		})
	}

	wg.Wait()
}

func TestRace_Public_KnownNetwork_SetAutoConnectAndGet(t *testing.T) {
	known := newTestKnownNetwork(t)

	const N = 200
	var wg sync.WaitGroup

	for i := range N {
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			if i%2 == 0 {
				_ = known.SetAutoConnect(ctx, i%4 == 0)
				return
			}
			_, _ = known.AutoConnect(ctx)
		})
	}

	wg.Wait()
}

func TestRace_Public_KnownNetwork_ForgetAndAccessors(t *testing.T) {
	known := newTestKnownNetwork(t)

	const N = 200
	var wg sync.WaitGroup

	for i := range N {
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			if i%2 == 0 {
				_ = known.Forget(ctx)
				return
			}
			_, _ = known.Properties(ctx)
		})
	}

	wg.Wait()
}

func TestRace_Public_KnownNetwork_SubscribeAutoConnectChanged_Concurrent(t *testing.T) {
	known := newTestKnownNetwork(t)

	fn := known.core.(*fakeCoreKnownNetwork)
	fn.setAutoConnectEvent(true)

	const N = 200
	var wg sync.WaitGroup

	for range N {
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			_, err := known.SubscribeAutoConnectChanged(ctx, func(bool) {})
			require.NoError(t, err)
		})
	}

	wg.Wait()
}

func TestRace_Public_KnownNetwork_SubscribePropertiesChanged_Concurrent(t *testing.T) {
	known := newTestKnownNetwork(t)

	fn := known.core.(*fakeCoreKnownNetwork)
	fn.setAutoConnectEvent(true)

	const N = 200
	var wg sync.WaitGroup

	for range N {
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			_, err := known.SubscribePropertiesChanged(ctx, func(ev KnownNetworkPropertiesChanged) {
				// User code may mutate the delivered map; the wrapper must not share
				// map instances across callbacks.
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
