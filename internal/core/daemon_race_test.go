//go:build race

package core

import (
	"context"
	"sync"
	"testing"
)

func TestRace_Core_Daemon_Info_ContextCancel(t *testing.T) {
	daemon := newTestDaemon(t)

	const N = 50
	var wg sync.WaitGroup

	for range N {
		wg.Go(func() {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			_, _ = daemon.Info(ctx)
		})
	}

	wg.Wait()
}

func TestRace_Core_Daemon_MixedCalls(t *testing.T) {
	daemon := newTestDaemon(t)

	const N = 100
	var wg sync.WaitGroup

	for i := range N {
		wg.Go(func() {
			ctx := context.Background()

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

func TestRace_Core_Daemon_NilReceiver(t *testing.T) {
	var d *Daemon

	const N = 50
	var wg sync.WaitGroup

	for range N {
		wg.Go(func() {
			_, _ = d.Info(context.Background())
		})
	}

	wg.Wait()
}
