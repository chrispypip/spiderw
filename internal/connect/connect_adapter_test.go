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

func TestWiring_NewAdapter_GuardsInputs(t *testing.T) {
	t.Run("nil wiring", func(t *testing.T) {
		var w *Wiring
		a, err := w.NewAdapter(context.Background(), "/phy0")
		require.Nil(t, a)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid state")
		require.Contains(t, err.Error(), "wiring cannot be nil")
	})

	t.Run("empty path", func(t *testing.T) {
		w := &Wiring{}
		a, err := w.NewAdapter(context.Background(), "")
		require.Nil(t, a)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid argument")
		require.Contains(t, err.Error(), "adapter path cannot be empty")
	})

	t.Run("relative path", func(t *testing.T) {
		w := &Wiring{}
		a, err := w.NewAdapter(context.Background(), "phy0")
		require.Nil(t, a)
		require.Error(t, err)
		require.Contains(t, err.Error(), "adapter path must be absolute")
	})

	t.Run("nil connection without factory", func(t *testing.T) {
		w := &Wiring{}
		a, err := w.NewAdapter(context.Background(), "/phy0")
		require.Nil(t, a)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid state")
		require.Contains(t, err.Error(), "D-Bus conn cannot be nil")
	})
}

func TestWiring_NewAdapter_UsesFactoryWhenPresent(t *testing.T) {
	want := &core.Adapter{}
	w := &Wiring{
		AdapterFactory: func(ctx context.Context, path string) (core.AdapterIface, error) {
			require.Equal(t, "/phy0", path)
			return want, nil
		},
	}
	got, err := w.NewAdapter(context.Background(), "/phy0")
	require.NoError(t, err)
	require.Same(t, want, got)
}

func TestWiring_NewAdapter_FailurePaths(t *testing.T) {
	origNewIwdAdapter := newIwdAdapterFn
	origNewCoreAdapter := newCoreAdapterFn
	t.Cleanup(func() {
		newIwdAdapterFn = origNewIwdAdapter
		newCoreAdapterFn = origNewCoreAdapter
	})

	t.Run("new_iwd_adapter_error", func(t *testing.T) {
		wantErr := errors.New("adapter init failed")
		newIwdAdapterFn = func(ctx context.Context, conn *dbus.Conn, path dbus.ObjectPath) (*iwdbus.Adapter, error) {
			return nil, wantErr
		}

		w := &Wiring{Conn: &dbus.Conn{}}
		a, err := w.NewAdapter(context.Background(), "/phy0")
		require.Nil(t, a)
		require.ErrorIs(t, err, wantErr)
	})

	t.Run("new_iwd_adapter_nil", func(t *testing.T) {
		newIwdAdapterFn = func(ctx context.Context, conn *dbus.Conn, path dbus.ObjectPath) (*iwdbus.Adapter, error) {
			return nil, nil
		}
		newCoreAdapterFn = func(raw *iwdbus.Adapter) *core.Adapter {
			return core.NewAdapter(raw)
		}

		w := &Wiring{Conn: &dbus.Conn{}}
		a, err := w.NewAdapter(context.Background(), "/phy0")
		require.Nil(t, a)
		require.Error(t, err)
		require.Contains(t, err.Error(), "iwd adapter interface not available")
	})

	t.Run("new_core_adapter_nil", func(t *testing.T) {
		newIwdAdapterFn = func(ctx context.Context, conn *dbus.Conn, path dbus.ObjectPath) (*iwdbus.Adapter, error) {
			return &iwdbus.Adapter{}, nil
		}
		newCoreAdapterFn = func(raw *iwdbus.Adapter) *core.Adapter {
			return nil
		}

		w := &Wiring{Conn: &dbus.Conn{}}
		a, err := w.NewAdapter(context.Background(), "/phy0")
		require.Nil(t, a)
		require.Error(t, err)
		require.Contains(t, err.Error(), "core adapter interface not available")
	})
}
