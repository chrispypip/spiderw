//go:build race

package core

import (
	"context"
	"sync"
	"testing"

	"github.com/godbus/dbus/v5"
)

// TestRace_Core_SignalLevelAgent_NotificationsVsUnregister races the agent's
// Changed/Release notifications against Unregister from many goroutines, checking
// the mutex-guarded lifecycle is free of data races and Unregister stays
// idempotent.
func TestRace_Core_SignalLevelAgent_NotificationsVsUnregister(t *testing.T) {
	a, h := NewSignalLevelAgent(SignalLevelConfig{
		Thresholds: []int{-60, -70, -80},
		Changed:    func(int) {},
		Released:   func() {},
	})
	a.Bind(&fakeSignalLevelRegistrar{}, "/spiderw/signalagent", func() error { return nil })

	const N = 300
	var wg sync.WaitGroup
	for i := range N {
		wg.Go(func() {
			switch i % 3 {
			case 0:
				h.Changed(dbus.ObjectPath("/net/connman/iwd/0/3"), uint8(i%5))
			case 1:
				h.Release()
			default:
				_ = a.Unregister(context.Background())
			}
		})
	}
	wg.Wait()
}
