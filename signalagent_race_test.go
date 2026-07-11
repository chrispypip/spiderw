//go:build race

package spiderw

import (
	"context"
	"sync"
	"testing"

	"github.com/chrispypip/spiderw/internal/core"
)

// TestRace_Public_SignalLevelAgent_MonitorUnregister races MonitorSignalLevel and
// Unregister from many goroutines against one station, checking the public
// wrapper reads its immutable state race-free.
func TestRace_Public_SignalLevelAgent_MonitorUnregister(t *testing.T) {
	register := func(context.Context, string, core.SignalLevelConfig) (core.SignalLevelAgentIface, error) {
		return &fakeCoreSignalLevelAgent{}, nil
	}
	st := newStation(&fakeCoreStation{}, "/net/connman/iwd/0/3", "wlan0").withSignalMonitor(register)

	const N = 300
	var wg sync.WaitGroup
	for range N {
		wg.Go(func() {
			ctx := context.Background()
			agent, err := st.MonitorSignalLevel(ctx, SignalLevelConfig{Thresholds: []int{-60, -70}, Changed: func(int) {}})
			if err == nil {
				_ = agent.Unregister(ctx)
			}
		})
	}
	wg.Wait()
}
