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

func TestWiring_NewAccessPoint_GuardsInputs(t *testing.T) {
	t.Run("nil wiring", func(t *testing.T) {
		var w *Wiring
		a, err := w.NewAccessPoint(context.Background(), "/phy0/wlan1")
		require.Nil(t, a)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid state")
		require.Contains(t, err.Error(), "wiring cannot be nil")
	})

	t.Run("empty path", func(t *testing.T) {
		w := &Wiring{}
		a, err := w.NewAccessPoint(context.Background(), "")
		require.Nil(t, a)
		require.Error(t, err)
		require.Contains(t, err.Error(), "access point path cannot be empty")
	})

	t.Run("relative path", func(t *testing.T) {
		w := &Wiring{}
		a, err := w.NewAccessPoint(context.Background(), "phy0/wlan1")
		require.Nil(t, a)
		require.Error(t, err)
		require.Contains(t, err.Error(), "access point path must be absolute")
	})

	t.Run("nil connection without factory", func(t *testing.T) {
		w := &Wiring{}
		a, err := w.NewAccessPoint(context.Background(), "/phy0/wlan1")
		require.Nil(t, a)
		require.Error(t, err)
		require.Contains(t, err.Error(), "D-Bus conn cannot be nil")
	})
}

func TestWiring_NewAccessPoint_UsesFactoryWhenPresent(t *testing.T) {
	want := &core.AccessPoint{}
	w := &Wiring{
		AccessPointFactory: func(ctx context.Context, path string) (core.AccessPointIface, error) {
			require.Equal(t, "/phy0/wlan1", path)
			return want, nil
		},
	}
	got, err := w.NewAccessPoint(context.Background(), "/phy0/wlan1")
	require.NoError(t, err)
	require.Same(t, want, got)
}

func TestWiring_NewAccessPoint_FailurePaths(t *testing.T) {
	origNewIwd := newIwdAccessPointFn
	origNewCore := newCoreAccessPointFn
	t.Cleanup(func() {
		newIwdAccessPointFn = origNewIwd
		newCoreAccessPointFn = origNewCore
	})

	t.Run("new_iwd_ap_error", func(t *testing.T) {
		wantErr := errors.New("ap init failed")
		newIwdAccessPointFn = func(ctx context.Context, conn *dbus.Conn, path dbus.ObjectPath) (*iwdbus.AccessPoint, error) {
			return nil, wantErr
		}

		w := &Wiring{Conn: &dbus.Conn{}}
		a, err := w.NewAccessPoint(context.Background(), "/phy0/wlan1")
		require.Nil(t, a)
		require.ErrorIs(t, err, wantErr)
		require.Contains(t, err.Error(), "access point unavailable")
	})

	t.Run("new_iwd_ap_nil", func(t *testing.T) {
		newIwdAccessPointFn = func(ctx context.Context, conn *dbus.Conn, path dbus.ObjectPath) (*iwdbus.AccessPoint, error) {
			return nil, nil
		}
		newCoreAccessPointFn = origNewCore

		w := &Wiring{Conn: &dbus.Conn{}}
		a, err := w.NewAccessPoint(context.Background(), "/phy0/wlan1")
		require.Nil(t, a)
		require.Error(t, err)
		require.Contains(t, err.Error(), "iwd access point interface not available")
	})

	t.Run("new_core_ap_nil", func(t *testing.T) {
		newIwdAccessPointFn = func(ctx context.Context, conn *dbus.Conn, path dbus.ObjectPath) (*iwdbus.AccessPoint, error) {
			return &iwdbus.AccessPoint{}, nil
		}
		newCoreAccessPointFn = func(raw *iwdbus.AccessPoint) *core.AccessPoint {
			return nil
		}

		w := &Wiring{Conn: &dbus.Conn{}}
		a, err := w.NewAccessPoint(context.Background(), "/phy0/wlan1")
		require.Nil(t, a)
		require.Error(t, err)
		require.Contains(t, err.Error(), "core access point interface not available")
	})

	t.Run("success", func(t *testing.T) {
		newIwdAccessPointFn = func(ctx context.Context, conn *dbus.Conn, path dbus.ObjectPath) (*iwdbus.AccessPoint, error) {
			return &iwdbus.AccessPoint{}, nil
		}
		newCoreAccessPointFn = origNewCore

		w := &Wiring{Conn: &dbus.Conn{}}
		a, err := w.NewAccessPoint(context.Background(), "/phy0/wlan1")
		require.NoError(t, err)
		require.NotNil(t, a)
	})
}
