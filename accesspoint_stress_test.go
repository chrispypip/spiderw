//go:build stress

package spiderw

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/chrispypip/spiderw/internal/core"
)

func TestStress_Public_AccessPoint_MixedMethods(t *testing.T) {
	ssid := "MyHostedNet"
	ap := newAccessPoint(&fakeCoreAccessPoint{started: true, ssid: &ssid}, "/net/connman/iwd/0/4", "wlan1")

	const N = 6000
	var wg sync.WaitGroup
	for i := range N {
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
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

func TestStress_Public_AccessPoint_Subscribes(t *testing.T) {
	ssid := "MyHostedNet"
	ap := newAccessPoint(&fakeCoreAccessPoint{
		started: true,
		ssid:    &ssid,
		subEvent: &core.AccessPointPropertiesChanged{
			Changed:     map[string]any{"Started": true, "Scanning": true},
			Invalidated: []string{"Frequency"},
		},
	}, "/net/connman/iwd/0/4", "wlan1")

	const N = 6000
	var wg sync.WaitGroup
	for i := range N {
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			switch i % 3 {
			case 0:
				_, _ = ap.SubscribeStartedChanged(ctx, func(bool) {})
			case 1:
				_, _ = ap.SubscribeScanningChanged(ctx, func(bool) {})
			default:
				_, _ = ap.SubscribePropertiesChanged(ctx, func(ev AccessPointPropertiesChanged) {
					if ev.Changed != nil {
						ev.Changed["user-mutation"] = 1
					}
				})
			}
		})
	}
	wg.Wait()
}

func TestStress_Public_AccessPoint_MixedContexts(t *testing.T) {
	ssid := "MyHostedNet"
	ap := newAccessPoint(&fakeCoreAccessPoint{started: true, ssid: &ssid}, "/net/connman/iwd/0/4", "wlan1")

	const N = 3000
	var wg sync.WaitGroup
	for i := range N {
		wg.Go(func() {
			// Half the callers hand in an already-cancelled context; the resolver
			// guard and delegate helpers must stay well-behaved either way.
			ctx, cancel := context.WithCancel(context.Background())
			if i%2 == 0 {
				cancel()
			} else {
				defer cancel()
			}

			switch i % 4 {
			case 0:
				_, _ = ap.Properties(ctx)
			case 1:
				_ = ap.Scan(ctx)
			case 2:
				_, _ = ap.OrderedNetworks(ctx)
			default:
				_ = ap.Stop(ctx)
			}
		})
	}
	wg.Wait()
}

func TestStress_Public_AccessPoint_NilReceiver(t *testing.T) {
	var ap *AccessPoint

	const N = 1000
	var wg sync.WaitGroup
	for range N {
		wg.Go(func() {
			_, _ = ap.Started(context.Background())
		})
	}
	wg.Wait()
}
