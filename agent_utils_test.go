//go:build unit || race || stress

package spiderw

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/require"

	"github.com/chrispypip/spiderw/internal/connect"
	"github.com/chrispypip/spiderw/internal/core"
)

// fakeCoreAgent is a concurrency-safe core.AgentIface for public agent tests.
type fakeCoreAgent struct {
	unregisterCalls atomic.Int32
	unregisterErr   error
}

func (f *fakeCoreAgent) Unregister(context.Context) error {
	f.unregisterCalls.Add(1)
	return f.unregisterErr
}

func (f *fakeCoreAgent) calls() int { return int(f.unregisterCalls.Load()) }

// newAgentTestClient returns a Client whose wiring delegates agent registration
// to factory.
func newAgentTestClient(t *testing.T, factory func(context.Context, core.CredentialCallbacks) (core.AgentIface, error)) *Client {
	t.Helper()

	wire := &connect.Wiring{
		Conn:         &dbus.Conn{},
		Daemon:       &fakeCoreDaemon{},
		Cleanup:      func() error { return nil },
		AgentFactory: factory,
	}
	c, err := newClientFromWiring(wire)
	require.NoError(t, err)
	return c
}

// validAgentConfig returns an AgentConfig with a single passphrase callback.
func validAgentConfig() AgentConfig {
	return AgentConfig{
		Passphrase: func(context.Context, string) (string, error) { return "hunter2", nil },
	}
}
