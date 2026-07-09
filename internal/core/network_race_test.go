//go:build race

package core

import (
	"context"
	"sync"
	"testing"
)

func TestRace_Core_Network_ConnectConcurrentWithGet(t *testing.T) {
	network := newTestNetwork(t)

	const N = 200
	var wg sync.WaitGroup

	for i := range N {
		wg.Go(func() {
			ctx := context.Background()
			if i%2 == 0 {
				_ = network.Connect(ctx)
			} else {
				_, _ = network.Connected(ctx)
			}
		})
	}

	wg.Wait()
}

func TestRace_Core_Network_MixedMethods(t *testing.T) {
	network := newTestNetwork(t)

	const N = 200
	var wg sync.WaitGroup

	for i := range N {
		wg.Go(func() {
			ctx := context.Background()

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

func TestRace_Core_Network_SubscribeConnectedChanged_Concurrent(t *testing.T) {
	network := newTestNetwork(t)

	const N = 200
	var wg sync.WaitGroup

	for range N {
		wg.Go(func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			_, _ = network.SubscribeConnectedChanged(ctx, func(bool) {})
		})
	}

	wg.Wait()
}

func TestRace_Core_Network_SubscribePropertiesChanged_Concurrent(t *testing.T) {
	network := newTestNetwork(t)

	const N = 200
	var wg sync.WaitGroup

	for range N {
		wg.Go(func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			_, _ = network.SubscribePropertiesChanged(ctx, func(NetworkPropertiesChanged) {})
		})
	}

	wg.Wait()
}

func TestRace_Core_Network_NilReceiver(t *testing.T) {
	var n *Network

	const N = 50
	var wg sync.WaitGroup

	for range N {
		wg.Go(func() {
			_, _ = n.Connected(context.Background())
		})
	}

	wg.Wait()
}
