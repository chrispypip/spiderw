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

func TestWiring_NewBasicServiceSet_GuardsInputs(t *testing.T) {
	t.Run("nil wiring", func(t *testing.T) {
		var w *Wiring
		b, err := w.NewBasicServiceSet(context.Background(), "/bss")
		require.Nil(t, b)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid state")
		require.Contains(t, err.Error(), "wiring cannot be nil")
	})

	t.Run("empty path", func(t *testing.T) {
		w := &Wiring{}
		b, err := w.NewBasicServiceSet(context.Background(), "")
		require.Nil(t, b)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid argument")
		require.Contains(t, err.Error(), "basic service set path cannot be empty")
	})

	t.Run("relative path", func(t *testing.T) {
		w := &Wiring{}
		b, err := w.NewBasicServiceSet(context.Background(), "bss")
		require.Nil(t, b)
		require.Error(t, err)
		require.Contains(t, err.Error(), "basic service set path must be absolute")
	})

	t.Run("nil connection without factory", func(t *testing.T) {
		w := &Wiring{}
		b, err := w.NewBasicServiceSet(context.Background(), "/bss")
		require.Nil(t, b)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid state")
		require.Contains(t, err.Error(), "D-Bus conn cannot be nil")
	})
}

func TestWiring_NewBasicServiceSet_UsesFactoryWhenPresent(t *testing.T) {
	want := &core.BasicServiceSet{}
	w := &Wiring{
		BasicServiceSetFactory: func(ctx context.Context, path string) (core.BasicServiceSetIface, error) {
			require.Equal(t, "/bss", path)
			return want, nil
		},
	}
	got, err := w.NewBasicServiceSet(context.Background(), "/bss")
	require.NoError(t, err)
	require.Same(t, want, got)
}

func TestWiring_NewBasicServiceSet_FailurePaths(t *testing.T) {
	origNewIwd := newIwdBSSFn
	origNewCore := newCoreBSSFn
	t.Cleanup(func() {
		newIwdBSSFn = origNewIwd
		newCoreBSSFn = origNewCore
	})

	t.Run("new_iwd_bss_error", func(t *testing.T) {
		wantErr := errors.New("bss init failed")
		newIwdBSSFn = func(ctx context.Context, conn *dbus.Conn, path dbus.ObjectPath) (*iwdbus.BasicServiceSet, error) {
			return nil, wantErr
		}

		w := &Wiring{Conn: &dbus.Conn{}}
		b, err := w.NewBasicServiceSet(context.Background(), "/bss")
		require.Nil(t, b)
		require.ErrorIs(t, err, wantErr)
	})

	t.Run("new_iwd_bss_nil", func(t *testing.T) {
		newIwdBSSFn = func(ctx context.Context, conn *dbus.Conn, path dbus.ObjectPath) (*iwdbus.BasicServiceSet, error) {
			return nil, nil
		}
		newCoreBSSFn = origNewCore

		w := &Wiring{Conn: &dbus.Conn{}}
		b, err := w.NewBasicServiceSet(context.Background(), "/bss")
		require.Nil(t, b)
		require.Error(t, err)
		require.Contains(t, err.Error(), "iwd basic service set interface not available")
	})

	t.Run("new_core_bss_nil", func(t *testing.T) {
		newIwdBSSFn = func(ctx context.Context, conn *dbus.Conn, path dbus.ObjectPath) (*iwdbus.BasicServiceSet, error) {
			return &iwdbus.BasicServiceSet{}, nil
		}
		newCoreBSSFn = func(raw *iwdbus.BasicServiceSet) *core.BasicServiceSet {
			return nil
		}

		w := &Wiring{Conn: &dbus.Conn{}}
		b, err := w.NewBasicServiceSet(context.Background(), "/bss")
		require.Nil(t, b)
		require.Error(t, err)
		require.Contains(t, err.Error(), "core basic service set interface not available")
	})
	t.Run("success", func(t *testing.T) {
		newIwdBSSFn = func(ctx context.Context, conn *dbus.Conn, path dbus.ObjectPath) (*iwdbus.BasicServiceSet, error) {
			return &iwdbus.BasicServiceSet{}, nil
		}
		newCoreBSSFn = origNewCore

		w := &Wiring{Conn: &dbus.Conn{}}
		b, err := w.NewBasicServiceSet(context.Background(), "/bss")
		require.NoError(t, err)
		require.NotNil(t, b)
	})
}
