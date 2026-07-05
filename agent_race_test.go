//go:build race

package spiderw

import (
	"context"
	"sync"
	"testing"

	"github.com/chrispypip/spiderw/internal/core"
)

// TestRace_Public_Agent_RegisterUnregisterClose exercises the closeMu and
// agent-slot clearing interplay: concurrent RegisterAgent, Unregister, and Close
// must not race on c.agent.
func TestRace_Public_Agent_RegisterUnregisterClose(t *testing.T) {
	c := newAgentTestClient(t, func(ctx context.Context, cc core.CredentialCallbacks) (core.AgentIface, error) {
		return &fakeCoreAgent{}, nil
	})

	const N = 200
	var wg sync.WaitGroup

	for i := range N {
		wg.Go(func() {
			ctx := context.Background()
			switch i % 3 {
			case 0:
				if a, err := c.RegisterAgent(ctx, validAgentConfig()); err == nil {
					_ = a.Unregister(ctx)
				}
			case 1:
				if a, err := c.RegisterAgent(ctx, validAgentConfig()); err == nil {
					_ = a.Unregister(ctx)
				}
			default:
				_, _ = c.RegisterAgent(ctx, validAgentConfig())
			}
		})
	}

	wg.Wait()
	_ = c.Close()
}
