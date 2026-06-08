//go:build stress

package spiderw

import (
	"context"
	"math/rand"
	"runtime"
	"sync"
	"testing"
	"time"
)

func TestStress_Public_Client_DaemonAndInfo(t *testing.T) {
	client := newTestClient(t)

	const N = 1000
	var wg sync.WaitGroup
	wg.Add(N)

	sem := make(chan struct{}, runtime.GOMAXPROCS(0)*2)

	for range N {
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			time.Sleep(time.Microsecond * time.Duration(rand.Intn(200)))
			d := client.Daemon()
			if d == nil {
				return
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_, _ = d.Info(ctx)
		}()
	}

	wg.Wait()
}
