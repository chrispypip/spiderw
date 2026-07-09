//go:build race

package spiderw

import (
	"context"
	"sync"
	"testing"
)

// TestRace_Public_Client_CloseDuringOperations targets the client-lifecycle data
// race: Close() takes the write lock and flips the closed flag while in-flight
// operations hold the read lock and read it. The -race detector must find no
// race, and operations must not panic when the client closes underneath them.
func TestRace_Public_Client_CloseDuringOperations(t *testing.T) {
	client := newTestClient(t)

	const N = 200
	var wg sync.WaitGroup

	for i := range N {
		wg.Go(func() {
			ctx := context.Background()
			switch i % 5 {
			case 0:
				if d := client.Daemon(); d != nil {
					_, _ = d.Info(ctx)
				}
			case 1:
				_, _ = client.AllAdapters(ctx)
			case 2:
				_, _ = client.AllDevices(ctx)
			case 3:
				_, _ = client.Adapter(ctx, "/phy0")
			case 4:
				_, _ = client.Device(ctx, "/net/connman/iwd/phy0/wlan0")
			}
		})
	}

	// Concurrent, idempotent Close from several goroutines while ops are in flight.
	for range 3 {
		wg.Go(func() { _ = client.Close() })
	}

	wg.Wait()
}
