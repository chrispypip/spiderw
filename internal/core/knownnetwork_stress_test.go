//go:build stress

package core

import (
	"context"
	"math/rand"
	"sync"
	"testing"
	"time"
)

func TestStress_Core_KnownNetwork_MixedContexts(t *testing.T) {
	known := newTestKnownNetwork(t)

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

			_, _ = known.AutoConnect(ctx)
		})
	}

	wg.Wait()
}

func TestStress_Core_KnownNetwork_MixedMethods(t *testing.T) {
	known := newTestKnownNetwork(t)

	const N = 6000
	var wg sync.WaitGroup

	for i := range N {
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			switch i % 8 {
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
			case 5:
				_ = known.SetAutoConnect(ctx, i%2 == 0)
			case 6:
				_, _ = known.Properties(ctx)
			case 7:
				_ = known.Forget(ctx)
			}
		})
	}

	wg.Wait()
}

func TestStress_Core_KnownNetwork_SubscribeAutoConnectChanged_Fanout(t *testing.T) {
	known := newTestKnownNetwork(t)

	const N = 4000
	var wg sync.WaitGroup

	for range N {
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_, _ = known.SubscribeAutoConnectChanged(ctx, func(bool) {})
		})
	}

	wg.Wait()
}

func TestStress_Core_KnownNetwork_Nil(t *testing.T) {
	var k *KnownNetwork

	const N = 1000
	var wg sync.WaitGroup

	for range N {
		wg.Go(func() {
			_, _ = k.AutoConnect(context.Background())
		})
	}

	wg.Wait()
}
