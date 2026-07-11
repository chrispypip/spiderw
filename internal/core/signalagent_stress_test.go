//go:build stress

package core

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/godbus/dbus/v5"

	"github.com/stretchr/testify/require"
)

// TestStress_Core_SignalLevelAgent_NotificationStorm drives a storm of Changed
// notifications from many goroutines while Release and Unregister race, ensuring
// the agent stays race-free and Unregister idempotent under load.
func TestStress_Core_SignalLevelAgent_NotificationStorm(t *testing.T) {
	var delivered atomic.Int64
	reg := &fakeSignalLevelRegistrar{}
	a, h := NewSignalLevelAgent(SignalLevelConfig{
		Thresholds: []int{-60, -70, -80},
		Changed:    func(int) { delivered.Add(1) },
	})
	a.Bind(reg, "/spiderw/signalagent", func() error { return nil })

	const N = 5000
	var wg sync.WaitGroup
	for i := range N {
		wg.Go(func() {
			switch i % 4 {
			case 0, 1:
				h.Changed(dbus.ObjectPath("/net/connman/iwd/0/3"), uint8(i%5))
			case 2:
				h.Release()
			default:
				_ = a.Unregister(context.Background())
			}
		})
	}
	wg.Wait()

	// Unregister fired concurrently many times but must have unregistered once.
	require.Equal(t, 1, reg.unregisterCount())
}
