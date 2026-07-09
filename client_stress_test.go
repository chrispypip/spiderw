//go:build stress

package spiderw

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestStress_Public_Client_CloseDuringUse hammers a single client with many
// concurrent operations while Close is called mid-flight. It guards the client
// lifecycle: no panic, no deadlock, no data race between the RWMutex-guarded
// closed flag and in-flight readers, and operations after close degrade to an
// error rather than crashing.
func TestStress_Public_Client_CloseDuringUse(t *testing.T) {
	client := newTestClient(t)

	const N = 4000
	var wg sync.WaitGroup

	op := func(i int) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
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
	}

	for i := range N {
		wg.Go(func() { op(i) })
	}

	// Close from several goroutines partway through: Close must be idempotent and
	// safe to call concurrently with in-flight operations.
	for range 4 {
		wg.Go(func() {
			time.Sleep(time.Microsecond * 50)
			_ = client.Close()
		})
	}

	wg.Wait()

	// Post-close: the client is closed and every operation returns gracefully.
	require.Nil(t, client.Daemon())
	_, err := client.AllAdapters(context.Background())
	require.ErrorIs(t, err, ErrInvalidState)
}

// TestStress_Public_Client_ConcurrentEnumeration hammers the enumeration entry
// points, which each take the read lock and build fresh handles, to surface any
// contention or shared-state corruption in handle construction.
func TestStress_Public_Client_ConcurrentEnumeration(t *testing.T) {
	client := newTestClient(t)
	defer func() { _ = client.Close() }()

	const N = 4000
	var wg sync.WaitGroup

	for i := range N {
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			switch i % 3 {
			case 0:
				_, _ = client.AllAdapters(ctx)
			case 1:
				_, _ = client.AllDevices(ctx)
			case 2:
				if d := client.Daemon(); d != nil {
					_, _ = d.Adapters(ctx)
				}
			}
		})
	}

	wg.Wait()
}
