//go:build unit

package connect

import (
	"context"
	"errors"
	"testing"

	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/require"

	"github.com/chrispypip/spiderw/internal/core"
	"github.com/chrispypip/spiderw/internal/iwdbus"
)

// NOTE: These tests intentionally do NOT run in parallel because they mutate
// package-level seam variables in connect.

type fakeConnectAgent struct{}

func (*fakeConnectAgent) Unregister(ctx context.Context) error { return nil }

func validConnectCallbacks() core.CredentialCallbacks {
	return core.CredentialCallbacks{
		Passphrase: func(ctx context.Context, networkPath string) (string, error) { return "p", nil },
	}
}

func TestWiring_NewAgent_GuardsInputs(t *testing.T) {
	t.Run("nil wiring", func(t *testing.T) {
		var w *Wiring
		a, err := w.NewAgent(context.Background(), validConnectCallbacks())
		require.Nil(t, a)
		require.Error(t, err)
		require.Contains(t, err.Error(), "wiring cannot be nil")
	})

	t.Run("nil connection without factory", func(t *testing.T) {
		w := &Wiring{}
		a, err := w.NewAgent(context.Background(), validConnectCallbacks())
		require.Nil(t, a)
		require.Error(t, err)
		require.Contains(t, err.Error(), "D-Bus conn cannot be nil")
	})
}

func TestWiring_NewAgent_UsesFactoryWhenPresent(t *testing.T) {
	want := &fakeConnectAgent{}
	w := &Wiring{
		AgentFactory: func(ctx context.Context, cc core.CredentialCallbacks) (core.AgentIface, error) {
			require.NotNil(t, cc.Passphrase)
			return want, nil
		},
	}
	got, err := w.NewAgent(context.Background(), validConnectCallbacks())
	require.NoError(t, err)
	require.Same(t, want, got)
}

func TestWiring_NewAgent_FailurePaths(t *testing.T) {
	origExport := exportAgentFn
	origNewManager := newIwdAgentManagerFn
	t.Cleanup(func() {
		exportAgentFn = origExport
		newIwdAgentManagerFn = origNewManager
	})

	t.Run("export_error", func(t *testing.T) {
		wantErr := errors.New("export failed")
		exportAgentFn = func(*dbus.Conn, dbus.ObjectPath, iwdbus.AgentHandler) (func() error, error) {
			return nil, wantErr
		}

		w := &Wiring{Conn: &dbus.Conn{}}
		a, err := w.NewAgent(context.Background(), validConnectCallbacks())
		require.Nil(t, a)
		require.ErrorIs(t, err, wantErr)
		require.Contains(t, err.Error(), "failed exporting agent object")
	})

	t.Run("agent_manager_error_unexports", func(t *testing.T) {
		var unexported bool
		exportAgentFn = func(*dbus.Conn, dbus.ObjectPath, iwdbus.AgentHandler) (func() error, error) {
			return func() error { unexported = true; return nil }, nil
		}
		wantErr := errors.New("no agent manager")
		newIwdAgentManagerFn = func(ctx context.Context, conn *dbus.Conn) (*iwdbus.AgentManager, error) {
			return nil, wantErr
		}

		w := &Wiring{Conn: &dbus.Conn{}}
		a, err := w.NewAgent(context.Background(), validConnectCallbacks())
		require.Nil(t, a)
		require.ErrorIs(t, err, wantErr)
		require.Contains(t, err.Error(), "agent manager unavailable")
		require.True(t, unexported, "export must be undone when registration fails")
	})
}
