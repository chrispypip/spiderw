//go:build stress

package core

import (
	"context"
	"math/rand"
	"sync"
	"testing"
	"time"
)

func TestStress_Core_Network_MixedContexts(t *testing.T) {
	network := newTestNetwork(t)

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

			_, _ = network.Connected(ctx)
		})
	}

	wg.Wait()
}

func TestStress_Core_Network_MixedMethods(t *testing.T) {
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

func TestStress_Core_Network_SubscribeConnectedChanged_Fanout(t *testing.T) {
	network := newTestNetwork(t)

	const N = 4000
	var wg sync.WaitGroup

	for range N {
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_, _ = network.SubscribeConnectedChanged(ctx, func(bool) {})
		})
	}

	wg.Wait()
}

func TestStress_Core_Network_Nil(t *testing.T) {
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
