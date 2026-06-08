//go:build race

package spiderw

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestRace_Public_Daemon_ContextCancel(t *testing.T) {
	client := newTestClient(t)
	daemon := client.Daemon()

	const N = 50
	var wg sync.WaitGroup
	wg.Add(N)

	for range N {
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			_, _ = daemon.Info(ctx)
		}()
	}

	wg.Wait()
}

func TestRace_Public_Client_DaemonAndInfo(t *testing.T) {
	client := newTestClient(t)

	const N = 100
	var wg sync.WaitGroup
	wg.Add(N)

	for range N {
		go func() {
			defer wg.Done()
			d := client.Daemon()
			if d != nil {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				_, _ = d.Info(ctx)
			}
		}()
	}

	wg.Wait()
}
