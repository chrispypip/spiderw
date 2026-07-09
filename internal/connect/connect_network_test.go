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

func TestWiring_NewNetwork_GuardsInputs(t *testing.T) {
	t.Run("nil wiring", func(t *testing.T) {
		var w *Wiring
		n, err := w.NewNetwork(context.Background(), "/net")
		require.Nil(t, n)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid state")
		require.Contains(t, err.Error(), "wiring cannot be nil")
	})

	t.Run("empty path", func(t *testing.T) {
		w := &Wiring{}
		n, err := w.NewNetwork(context.Background(), "")
		require.Nil(t, n)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid argument")
		require.Contains(t, err.Error(), "network path cannot be empty")
	})

	t.Run("relative path", func(t *testing.T) {
		w := &Wiring{}
		n, err := w.NewNetwork(context.Background(), "net")
		require.Nil(t, n)
		require.Error(t, err)
		require.Contains(t, err.Error(), "network path must be absolute")
	})

	t.Run("nil connection without factory", func(t *testing.T) {
		w := &Wiring{}
		n, err := w.NewNetwork(context.Background(), "/net")
		require.Nil(t, n)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid state")
		require.Contains(t, err.Error(), "D-Bus conn cannot be nil")
	})
}

func TestWiring_NewNetwork_UsesFactoryWhenPresent(t *testing.T) {
	want := &core.Network{}
	w := &Wiring{
		NetworkFactory: func(ctx context.Context, path string) (core.NetworkIface, error) {
			require.Equal(t, "/net", path)
			return want, nil
		},
	}
	got, err := w.NewNetwork(context.Background(), "/net")
	require.NoError(t, err)
	require.Same(t, want, got)
}

func TestWiring_NewNetwork_FailurePaths(t *testing.T) {
	origNewIwd := newIwdNetworkFn
	origNewCore := newCoreNetworkFn
	t.Cleanup(func() {
		newIwdNetworkFn = origNewIwd
		newCoreNetworkFn = origNewCore
	})

	t.Run("new_iwd_network_error", func(t *testing.T) {
		wantErr := errors.New("network init failed")
		newIwdNetworkFn = func(ctx context.Context, conn *dbus.Conn, path dbus.ObjectPath) (*iwdbus.Network, error) {
			return nil, wantErr
		}

		w := &Wiring{Conn: &dbus.Conn{}}
		n, err := w.NewNetwork(context.Background(), "/net")
		require.Nil(t, n)
		require.ErrorIs(t, err, wantErr)
	})

	t.Run("new_iwd_network_nil", func(t *testing.T) {
		newIwdNetworkFn = func(ctx context.Context, conn *dbus.Conn, path dbus.ObjectPath) (*iwdbus.Network, error) {
			return nil, nil
		}
		newCoreNetworkFn = origNewCore

		w := &Wiring{Conn: &dbus.Conn{}}
		n, err := w.NewNetwork(context.Background(), "/net")
		require.Nil(t, n)
		require.Error(t, err)
		require.Contains(t, err.Error(), "iwd network interface not available")
	})

	t.Run("new_core_network_nil", func(t *testing.T) {
		newIwdNetworkFn = func(ctx context.Context, conn *dbus.Conn, path dbus.ObjectPath) (*iwdbus.Network, error) {
			return &iwdbus.Network{}, nil
		}
		newCoreNetworkFn = func(raw *iwdbus.Network) *core.Network {
			return nil
		}

		w := &Wiring{Conn: &dbus.Conn{}}
		n, err := w.NewNetwork(context.Background(), "/net")
		require.Nil(t, n)
		require.Error(t, err)
		require.Contains(t, err.Error(), "core network interface not available")
	})
	t.Run("success", func(t *testing.T) {
		newIwdNetworkFn = func(ctx context.Context, conn *dbus.Conn, path dbus.ObjectPath) (*iwdbus.Network, error) {
			return &iwdbus.Network{}, nil
		}
		newCoreNetworkFn = origNewCore

		w := &Wiring{Conn: &dbus.Conn{}}
		n, err := w.NewNetwork(context.Background(), "/net")
		require.NoError(t, err)
		require.NotNil(t, n)
	})
}
