//go:build stress

package spiderw

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/chrispypip/spiderw/internal/core"
)

func TestStress_Public_SimpleConfiguration_HandleAndMethods(t *testing.T) {
	fake := &fakeCoreSimpleConfig{genPin: "12345670"}
	construct := func(context.Context, string) (core.SimpleConfigurationIface, error) {
		return fake, nil
	}
	st := newStation(&fakeCoreStation{}, "/net/connman/iwd/0/3", "wlan0").withSimpleConfiguration(construct)

	const N = 6000
	var wg sync.WaitGroup
	for i := range N {
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

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

func TestStress_Public_SimpleConfiguration_NilHandle(t *testing.T) {
	var wsc *SimpleConfiguration

	const N = 1000
	var wg sync.WaitGroup
	for range N {
		wg.Go(func() {
			_ = wsc.PushButton(context.Background())
		})
	}
	wg.Wait()
}
