//go:build stress

package core

import (
	"context"
	"math/rand"
	"sync"
	"testing"
	"time"
)

func TestStress_Core_SimpleConfiguration_MixedMethods(t *testing.T) {
	c := newTestSimpleConfiguration(t)

	const N = 6000
	var wg sync.WaitGroup

	for i := range N {
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			switch i % 4 {
			case 0:
				_ = c.PushButton(ctx)
			case 1:
				_, _ = c.GeneratePin(ctx)
			case 2:
				_ = c.StartPin(ctx, "12345670")
			case 3:
				_ = c.Cancel(ctx)
			}
		})
	}

	wg.Wait()
}

func TestStress_Core_SimpleConfiguration_MixedContexts(t *testing.T) {
	c := newTestSimpleConfiguration(t)

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

			_ = c.PushButton(ctx)
		})
	}

	wg.Wait()
}

func TestStress_Core_SimpleConfiguration_NilReceiver(t *testing.T) {
	var c *SimpleConfiguration

	const N = 1000
	var wg sync.WaitGroup

	for range N {
		wg.Go(func() {
			_ = c.PushButton(context.Background())
		})
	}

	wg.Wait()
}
