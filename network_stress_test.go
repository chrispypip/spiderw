//go:build stress

package spiderw

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestStress_Public_Network_MixedMethods(t *testing.T) {
	network := newTestNetwork(t)

	const N = 6000
	var wg sync.WaitGroup

	for i := range N {
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			switch i % 8 {
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
			case 6:
				_, _ = network.Properties(ctx)
			case 7:
				_ = network.Connect(ctx)
			}
		})
	}

	wg.Wait()
}

func TestStress_Public_Network_SubscribeConnectedChanged_Concurrent(t *testing.T) {
	network := newTestNetwork(t)

	fn := network.core.(*fakeCoreNetwork)
	fn.setConnectedEvent(true)

	const N = 4000
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

func TestStress_Public_Network_NilReceiver(t *testing.T) {
	var n *Network

	const N = 1000
	var wg sync.WaitGroup

	for range N {
		wg.Go(func() {
			_, _ = n.Connected(context.Background())
		})
	}

	wg.Wait()
}
