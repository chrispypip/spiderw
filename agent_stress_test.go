//go:build stress

package spiderw

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/chrispypip/spiderw/internal/core"
)

// TestStress_Public_Agent_RegisterUnregisterChurn hammers the single-agent-slot
// lifecycle from many goroutines. Concurrent RegisterAgent calls contend for the
// one slot: at most one wins, the rest get an already-registered error, and each
// winner frees the slot via Unregister. The client's closeMu serializes slot
// mutation and the clear() callback; this guards that invariant under contention
// (no panic, no deadlock, no double-occupied slot).
func TestStress_Public_Agent_RegisterUnregisterChurn(t *testing.T) {
	client := newAgentTestClient(t, func(ctx context.Context, cc core.CredentialCallbacks) (core.AgentIface, error) {
		return &fakeCoreAgent{}, nil
	})
	defer func() { _ = client.Close() }()

	ctx := context.Background()
	const N = 4000
	var wg sync.WaitGroup

	for range N {
		wg.Go(func() {
			agent, err := client.RegisterAgent(ctx, validAgentConfig())
			if err != nil {
				// Slot already occupied by a concurrent registration; expected.
				return
			}
			// Won the slot: free it so another registration can proceed.
			_ = agent.Unregister(ctx)
		})
	}

	wg.Wait()

	// The slot is free after the churn, so a final registration succeeds.
	agent, err := client.RegisterAgent(ctx, validAgentConfig())
	require.NoError(t, err)
	require.NotNil(t, agent)
}
