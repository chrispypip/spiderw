//go:build race

package core

import (
	"context"
	"sync"
	"testing"
)

func TestRace_Core_SimpleConfiguration_MixedMethods(t *testing.T) {
	c := newTestSimpleConfiguration(t)

	const N = 200
	var wg sync.WaitGroup

	for i := range N {
		wg.Go(func() {
			ctx := context.Background()

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

func TestRace_Core_SimpleConfiguration_NilReceiver(t *testing.T) {
	var c *SimpleConfiguration

	const N = 50
	var wg sync.WaitGroup

	for range N {
		wg.Go(func() {
			_ = c.PushButton(context.Background())
		})
	}

	wg.Wait()
}
