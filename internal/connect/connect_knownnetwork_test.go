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

func TestWiring_NewKnownNetwork_GuardsInputs(t *testing.T) {
	t.Run("nil wiring", func(t *testing.T) {
		var w *Wiring
		k, err := w.NewKnownNetwork(context.Background(), "/known")
		require.Nil(t, k)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid state")
		require.Contains(t, err.Error(), "wiring cannot be nil")
	})

	t.Run("empty path", func(t *testing.T) {
		w := &Wiring{}
		k, err := w.NewKnownNetwork(context.Background(), "")
		require.Nil(t, k)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid argument")
		require.Contains(t, err.Error(), "known network path cannot be empty")
	})

	t.Run("relative path", func(t *testing.T) {
		w := &Wiring{}
		k, err := w.NewKnownNetwork(context.Background(), "known")
		require.Nil(t, k)
		require.Error(t, err)
		require.Contains(t, err.Error(), "known network path must be absolute")
	})

	t.Run("nil connection without factory", func(t *testing.T) {
		w := &Wiring{}
		k, err := w.NewKnownNetwork(context.Background(), "/known")
		require.Nil(t, k)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid state")
		require.Contains(t, err.Error(), "D-Bus conn cannot be nil")
	})
}

func TestWiring_NewKnownNetwork_UsesFactoryWhenPresent(t *testing.T) {
	want := &core.KnownNetwork{}
	w := &Wiring{
		KnownNetworkFactory: func(ctx context.Context, path string) (core.KnownNetworkIface, error) {
			require.Equal(t, "/known", path)
			return want, nil
		},
	}
	got, err := w.NewKnownNetwork(context.Background(), "/known")
	require.NoError(t, err)
	require.Same(t, want, got)
}

func TestWiring_NewKnownNetwork_FailurePaths(t *testing.T) {
	origNewIwd := newIwdKnownNetworkFn
	origNewCore := newCoreKnownNetworkFn
	t.Cleanup(func() {
		newIwdKnownNetworkFn = origNewIwd
		newCoreKnownNetworkFn = origNewCore
	})

	t.Run("new_iwd_known_network_error", func(t *testing.T) {
		wantErr := errors.New("known network init failed")
		newIwdKnownNetworkFn = func(ctx context.Context, conn *dbus.Conn, path dbus.ObjectPath) (*iwdbus.KnownNetwork, error) {
			return nil, wantErr
		}

		w := &Wiring{Conn: &dbus.Conn{}}
		k, err := w.NewKnownNetwork(context.Background(), "/known")
		require.Nil(t, k)
		require.ErrorIs(t, err, wantErr)
	})

	t.Run("new_iwd_known_network_nil", func(t *testing.T) {
		newIwdKnownNetworkFn = func(ctx context.Context, conn *dbus.Conn, path dbus.ObjectPath) (*iwdbus.KnownNetwork, error) {
			return nil, nil
		}
		newCoreKnownNetworkFn = origNewCore

		w := &Wiring{Conn: &dbus.Conn{}}
		k, err := w.NewKnownNetwork(context.Background(), "/known")
		require.Nil(t, k)
		require.Error(t, err)
		require.Contains(t, err.Error(), "iwd known network interface not available")
	})

	t.Run("new_core_known_network_nil", func(t *testing.T) {
		newIwdKnownNetworkFn = func(ctx context.Context, conn *dbus.Conn, path dbus.ObjectPath) (*iwdbus.KnownNetwork, error) {
			return &iwdbus.KnownNetwork{}, nil
		}
		newCoreKnownNetworkFn = func(raw *iwdbus.KnownNetwork) *core.KnownNetwork {
			return nil
		}

		w := &Wiring{Conn: &dbus.Conn{}}
		k, err := w.NewKnownNetwork(context.Background(), "/known")
		require.Nil(t, k)
		require.Error(t, err)
		require.Contains(t, err.Error(), "core known network interface not available")
	})
	t.Run("success", func(t *testing.T) {
		newIwdKnownNetworkFn = func(ctx context.Context, conn *dbus.Conn, path dbus.ObjectPath) (*iwdbus.KnownNetwork, error) {
			return &iwdbus.KnownNetwork{}, nil
		}
		newCoreKnownNetworkFn = origNewCore

		w := &Wiring{Conn: &dbus.Conn{}}
		k, err := w.NewKnownNetwork(context.Background(), "/known")
		require.NoError(t, err)
		require.NotNil(t, k)
	})
}
