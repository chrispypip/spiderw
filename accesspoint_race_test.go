//go:build race

package spiderw

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/chrispypip/spiderw/internal/core"
)

// newRaceAccessPoint builds a public AccessPoint over a concurrency-safe fake
// core whose subscribes deliver an event, so the subscribe paths do real work
// under the race detector.
func newRaceAccessPoint() *AccessPoint {
	ssid := "MyHostedNet"
	freq := uint32(5180)
	return newAccessPoint(&fakeCoreAccessPoint{
		started:   true,
		ssid:      &ssid,
		frequency: &freq,
		subEvent: &core.AccessPointPropertiesChanged{
			Changed:     map[string]any{"Started": true, "Scanning": true},
			Invalidated: []string{"Frequency"},
		},
	}, "/net/connman/iwd/0/4", "wlan1")
}

func TestRace_Public_AccessPoint_MixedMethods(t *testing.T) {
	ssid := "MyHostedNet"
	freq := uint32(5180)
	ap := newAccessPoint(&fakeCoreAccessPoint{started: true, ssid: &ssid, frequency: &freq}, "/net/connman/iwd/0/4", "wlan1")

	const N = 300
	var wg sync.WaitGroup
	for i := range N {
		wg.Go(func() {
			ctx := context.Background()
			switch i % 9 {
			case 0:
				_, _ = ap.Started(ctx)
			case 1:
				_, _ = ap.Scanning(ctx)
			case 2:
				_, _ = ap.SSID(ctx)
			case 3:
				_, _ = ap.Properties(ctx)
			case 4:
				_ = ap.Start(ctx, "MyAP", "s3cretpass")
			case 5:
				_ = ap.StartProfile(ctx, "HomeAP")
			case 6:
				_ = ap.Stop(ctx)
			case 7:
				_ = ap.Scan(ctx)
			case 8:
				_, _ = ap.OrderedNetworks(ctx)
			}
		})
	}
	wg.Wait()
}

func TestRace_Public_AccessPoint_SubscribeStartedChanged_Concurrent(t *testing.T) {
	ap := newRaceAccessPoint()

	const N = 200
	var wg sync.WaitGroup
	for range N {
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_, err := ap.SubscribeStartedChanged(ctx, func(bool) {})
			require.NoError(t, err)
		})
	}
	wg.Wait()
}

func TestRace_Public_AccessPoint_SubscribeScanningChanged_Concurrent(t *testing.T) {
	ap := newRaceAccessPoint()

	const N = 200
	var wg sync.WaitGroup
	for range N {
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_, err := ap.SubscribeScanningChanged(ctx, func(bool) {})
			require.NoError(t, err)
		})
	}
	wg.Wait()
}

func TestRace_Public_AccessPoint_SubscribePropertiesChanged_Concurrent(t *testing.T) {
	ap := newRaceAccessPoint()

	const N = 200
	var wg sync.WaitGroup
	for range N {
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_, err := ap.SubscribePropertiesChanged(ctx, func(ev AccessPointPropertiesChanged) {
				// User code may mutate its copy; the wrapper must not share maps or
				// slices across concurrent callbacks.
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

func TestRace_Public_AccessPoint_ContextCancel(t *testing.T) {
	ap := newRaceAccessPoint()

	const N = 100
	var wg sync.WaitGroup
	for i := range N {
		wg.Go(func() {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			switch i % 6 {
			case 0:
				_, _ = ap.Started(ctx)
			case 1:
				_ = ap.Scan(ctx)
			case 2:
				_, _ = ap.Properties(ctx)
			case 3:
				_ = ap.Start(ctx, "MyAP", "s3cretpass")
			case 4:
				_, _ = ap.OrderedNetworks(ctx)
			default:
				_, _ = ap.SubscribeStartedChanged(ctx, func(bool) {})
			}
		})
	}
	wg.Wait()
}

func TestRace_Public_AccessPoint_NilReceiver(t *testing.T) {
	var ap *AccessPoint

	const N = 50
	var wg sync.WaitGroup
	for range N {
		wg.Go(func() {
			_, _ = ap.Started(context.Background())
		})
	}
	wg.Wait()
}
