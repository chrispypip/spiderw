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

func TestRace_Public_Client_Network(t *testing.T) {
	client := newTestNetworkClient(t)
	refs, err := client.Daemon().Networks(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, refs)

	const N = 100
	errCh := make(chan error, N)

	var wg sync.WaitGroup

	for range N {
		wg.Go(func() {
			n, err := client.Network(context.Background(), refs[0].Path)
			if err != nil {
				errCh <- err
				return
			}
			if n == nil {
				errCh <- errors.New("nil network")
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

func TestRace_Public_Network_ConcurrentAccessors(t *testing.T) {
	network := newTestNetwork(t)

	const N = 200
	var wg sync.WaitGroup

	for i := range N {
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			switch i % 7 {
			case 0:
				_, _ = network.Name(ctx)
			case 1:
				_, _ = network.Connected(ctx)
			case 2:
				_, _ = network.Device(ctx)
			case 3:
				_, _ = network.Type(ctx)
			case 4:
				_, _ = network.KnownNetwork(ctx)
			case 5:
				_, _ = network.ExtendedServiceSet(ctx)
			default:
				_, _ = network.Properties(ctx)
			}
		})
	}

	wg.Wait()
}

func TestRace_Public_Network_ConnectAndConnected(t *testing.T) {
	network := newTestNetwork(t)

	const N = 200
	var wg sync.WaitGroup

	for i := range N {
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			if i%2 == 0 {
				_ = network.Connect(ctx)
				return
			}
			_, _ = network.Connected(ctx)
		})
	}

	wg.Wait()
}

func TestRace_Public_Network_SubscribeConnectedChanged_Concurrent(t *testing.T) {
	network := newTestNetwork(t)

	fn := network.core.(*fakeCoreNetwork)
	fn.setConnectedEvent(true)

	const N = 200
	var wg sync.WaitGroup

	for range N {
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			_, err := network.SubscribeConnectedChanged(ctx, func(bool) {})
			require.NoError(t, err)
		})
	}

	wg.Wait()
}

func TestRace_Public_Network_SubscribePropertiesChanged_Concurrent(t *testing.T) {
	network := newTestNetwork(t)

	fn := network.core.(*fakeCoreNetwork)
	fn.setConnectedEvent(true)

	const N = 200
	var wg sync.WaitGroup

	for range N {
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			_, err := network.SubscribePropertiesChanged(ctx, func(ev NetworkPropertiesChanged) {
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
