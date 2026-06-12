//go:build stress

package core

import (
	"context"
	"math/rand"
	"sync"
	"testing"
	"time"
)

func TestStress_Core_Daemon_MixedContexts(t *testing.T) {
	daemon := newTestDaemon(t)

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

			_, _ = daemon.Info(ctx)
		})
	}

	wg.Wait()
}

func TestStress_Core_Daemon_MixedMethods(t *testing.T) {
	daemon := newTestDaemon(t)

	const N = 1000
	var wg sync.WaitGroup

	for i := range N {
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			switch i % 5 {
			case 0:
				_, _ = daemon.Info(ctx)
			case 1:
				_, _ = daemon.Version(ctx)
			case 2:
				_, _ = daemon.StateDirectory(ctx)
			case 3:
				_, _ = daemon.NetworkConfigurationEnabled(ctx)
			case 4:
				_, _ = daemon.Adapters(ctx)
			}
		})
	}

	wg.Wait()
}

func TestStress_Core_Daemon_Nil(t *testing.T) {
	var d *Daemon

	const N = 500
	var wg sync.WaitGroup

	for range N {
		wg.Go(func() {
			_, _ = d.Info(context.Background())
		})
	}

	wg.Wait()
}
