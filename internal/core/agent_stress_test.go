//go:build stress

package core

import (
	"context"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
)

// TestStress_Core_Agent_CancelDuringBlockedRequests drives many requests whose
// callbacks block on their context while Cancel and Unregister race to abort
// them, exercising the cancellation plumbing under load.
func TestStress_Core_Agent_CancelDuringBlockedRequests(t *testing.T) {
	a, h := NewAgent(CredentialCallbacks{
		Passphrase: func(ctx context.Context, networkPath string) (string, error) {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(time.Millisecond):
				return "p", nil
			}
		},
		OnCancel:  func(string) {},
		OnRelease: func() {},
	})

	const N = 800
	var wg sync.WaitGroup
	path := dbus.ObjectPath(testAgentNetworkPath)

	for range N {
		wg.Go(func() {
			switch rand.Intn(3) {
			case 0:
				_, _ = h.RequestPassphrase(context.Background(), path)
			case 1:
				h.Cancel("timed-out")
			default:
				h.Release()
			}
		})
	}

	wg.Wait()
	_ = a.Unregister(context.Background())
}
