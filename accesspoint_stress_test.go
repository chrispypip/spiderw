//go:build stress

package spiderw

import (
	"context"
	"sync"
	"testing"
	"time"
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
