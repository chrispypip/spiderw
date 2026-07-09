//go:build race

package core

import (
	"context"
	"sync"
	"testing"

	"github.com/godbus/dbus/v5"
)

// TestRace_Core_Agent_RequestsConcurrentWithCancel hammers the in-flight
// cancellation state from many goroutines: credential requests set currentCancel
// while Cancel reads and invokes it.
func TestRace_Core_Agent_RequestsConcurrentWithCancel(t *testing.T) {
	a, h := NewAgent(CredentialCallbacks{
		Passphrase:          func(ctx context.Context, networkPath string) (string, error) { return "p", nil },
		UserNameAndPassword: func(ctx context.Context, networkPath string) (string, string, error) { return "u", "p", nil },
		UserPassword:        func(ctx context.Context, networkPath, user string) (string, error) { return "p", nil },
		OnCancel:            func(string) {},
		OnRelease:           func() {},
	})

	const N = 300
	var wg sync.WaitGroup
	path := dbus.ObjectPath(testAgentNetworkPath)

	for i := range N {
		wg.Go(func() {
			switch i % 5 {
			case 0:
				_, _ = h.RequestPassphrase(context.Background(), path)
			case 1:
				_, _, _ = h.RequestUserNameAndPassword(context.Background(), path)
			case 2:
				_, _ = h.RequestUserPassword(context.Background(), path, "u")
			case 3:
				h.Cancel("user-canceled")
			default:
				h.Release()
			}
		})
	}

	wg.Wait()
	_ = a.Unregister(context.Background())
}
