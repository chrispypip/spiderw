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

func TestWiring_NewStation_GuardsInputs(t *testing.T) {
	t.Run("nil wiring", func(t *testing.T) {
		var w *Wiring
		s, err := w.NewStation(context.Background(), "/phy0/wlan0")
		require.Nil(t, s)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid state")
		require.Contains(t, err.Error(), "wiring cannot be nil")
	})

	t.Run("empty path", func(t *testing.T) {
		w := &Wiring{}
		s, err := w.NewStation(context.Background(), "")
		require.Nil(t, s)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid argument")
		require.Contains(t, err.Error(), "station path cannot be empty")
	})

	t.Run("relative path", func(t *testing.T) {
		w := &Wiring{}
		s, err := w.NewStation(context.Background(), "phy0/wlan0")
		require.Nil(t, s)
		require.Error(t, err)
		require.Contains(t, err.Error(), "station path must be absolute")
	})

	t.Run("nil connection without factory", func(t *testing.T) {
		w := &Wiring{}
		s, err := w.NewStation(context.Background(), "/phy0/wlan0")
		require.Nil(t, s)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid state")
		require.Contains(t, err.Error(), "D-Bus conn cannot be nil")
	})
}

func TestWiring_NewStation_UsesFactoryWhenPresent(t *testing.T) {
	want := &core.Station{}
	w := &Wiring{
		StationFactory: func(ctx context.Context, path string) (core.StationIface, error) {
			require.Equal(t, "/phy0/wlan0", path)
			return want, nil
		},
	}
	got, err := w.NewStation(context.Background(), "/phy0/wlan0")
	require.NoError(t, err)
	require.Same(t, want, got)
}

func TestWiring_NewStation_FailurePaths(t *testing.T) {
	origNewIwdStation := newIwdStationFn
	origNewCoreStation := newCoreStationFn
	t.Cleanup(func() {
		newIwdStationFn = origNewIwdStation
		newCoreStationFn = origNewCoreStation
	})

	t.Run("new_iwd_station_error", func(t *testing.T) {
		wantErr := errors.New("station init failed")
		newIwdStationFn = func(ctx context.Context, conn *dbus.Conn, path dbus.ObjectPath) (*iwdbus.Station, error) {
			return nil, wantErr
		}

		w := &Wiring{Conn: &dbus.Conn{}}
		s, err := w.NewStation(context.Background(), "/phy0/wlan0")
		require.Nil(t, s)
		require.ErrorIs(t, err, wantErr)
	})

	t.Run("new_iwd_station_nil", func(t *testing.T) {
		newIwdStationFn = func(ctx context.Context, conn *dbus.Conn, path dbus.ObjectPath) (*iwdbus.Station, error) {
			return nil, nil
		}
		newCoreStationFn = origNewCoreStation

		w := &Wiring{Conn: &dbus.Conn{}}
		s, err := w.NewStation(context.Background(), "/phy0/wlan0")
		require.Nil(t, s)
		require.Error(t, err)
		require.Contains(t, err.Error(), "iwd station interface not available")
	})

	t.Run("new_core_station_nil", func(t *testing.T) {
		newIwdStationFn = func(ctx context.Context, conn *dbus.Conn, path dbus.ObjectPath) (*iwdbus.Station, error) {
			return &iwdbus.Station{}, nil
		}
		newCoreStationFn = func(raw *iwdbus.Station) *core.Station {
			return nil
		}

		w := &Wiring{Conn: &dbus.Conn{}}
		s, err := w.NewStation(context.Background(), "/phy0/wlan0")
		require.Nil(t, s)
		require.Error(t, err)
		require.Contains(t, err.Error(), "core station interface not available")
	})

	t.Run("success", func(t *testing.T) {
		newIwdStationFn = func(ctx context.Context, conn *dbus.Conn, path dbus.ObjectPath) (*iwdbus.Station, error) {
			return &iwdbus.Station{}, nil
		}
		newCoreStationFn = origNewCoreStation

		w := &Wiring{Conn: &dbus.Conn{}}
		s, err := w.NewStation(context.Background(), "/phy0/wlan0")
		require.NoError(t, err)
		require.NotNil(t, s)
	})
}
