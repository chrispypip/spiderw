//go:build race

package spiderw

import (
	"context"
	"sync"
	"testing"

	"github.com/chrispypip/spiderw/internal/core"
)

// TestRace_Public_SimpleConfiguration_HandleAndMethods races
// Station.SimpleConfiguration and the handle methods from many goroutines against
// one station and one shared handle, checking the public wrappers read their
// immutable state race-free.
func TestRace_Public_SimpleConfiguration_HandleAndMethods(t *testing.T) {
	fake := &fakeCoreSimpleConfig{genPin: "12345670"}
	construct := func(context.Context, string) (core.SimpleConfigurationIface, error) {
		return fake, nil
	}
	st := newStation(&fakeCoreStation{}, "/net/connman/iwd/0/3", "wlan0").withSimpleConfiguration(construct)

	const N = 300
	var wg sync.WaitGroup
	for i := range N {
		wg.Go(func() {
			ctx := context.Background()
			wsc, err := st.SimpleConfiguration(ctx)
			if err != nil {
				return
			}
			switch i % 4 {
			case 0:
				_ = wsc.PushButton(ctx)
			case 1:
				_, _ = wsc.GeneratePin(ctx)
			case 2:
				_ = wsc.StartPin(ctx, "12345670")
			case 3:
				_ = wsc.Cancel(ctx)
			}
		})
	}
	wg.Wait()
}

func TestRace_Public_SimpleConfiguration_NilHandle(t *testing.T) {
	var wsc *SimpleConfiguration

	const N = 50
	var wg sync.WaitGroup
	for range N {
		wg.Go(func() {
			_ = wsc.PushButton(context.Background())
		})
	}
	wg.Wait()
}
