//go:build stress

package spiderw

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/chrispypip/spiderw/internal/core"
)

// TestStress_Public_SignalLevelAgent_MonitorChurn drives many monitor+unregister
// cycles concurrently against one station under load.
func TestStress_Public_SignalLevelAgent_MonitorChurn(t *testing.T) {
	register := func(context.Context, string, core.SignalLevelConfig) (core.SignalLevelAgentIface, error) {
		return &fakeCoreSignalLevelAgent{}, nil
	}
	st := newStation(&fakeCoreStation{}, "/net/connman/iwd/0/3", "wlan0").withSignalMonitor(register)

	const N = 4000
	var wg sync.WaitGroup
	for range N {
		wg.Go(func() {
			ctx := context.Background()
			agent, err := st.MonitorSignalLevel(ctx, SignalLevelConfig{Thresholds: []int{-60, -70}, Changed: func(int) {}})
			require.NoError(t, err)
			require.NoError(t, agent.Unregister(ctx))
		})
	}
	wg.Wait()
}
