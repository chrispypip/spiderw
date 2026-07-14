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

type fakeConnectSignalLevelAgent struct{}

func (*fakeConnectSignalLevelAgent) Unregister(ctx context.Context) error { return nil }

func validConnectSignalConfig() core.SignalLevelConfig {
	return core.SignalLevelConfig{Thresholds: []int{-60, -70}, Changed: func(int) {}}
}

func TestWiring_NewSignalLevelAgent_GuardsInputs(t *testing.T) {
	t.Run("nil wiring", func(t *testing.T) {
		var w *Wiring
		a, err := w.NewSignalLevelAgent(context.Background(), "/net/connman/iwd/0/3", validConnectSignalConfig())
		require.Nil(t, a)
		require.Error(t, err)
		require.Contains(t, err.Error(), "wiring cannot be nil")
	})

	t.Run("nil connection without factory", func(t *testing.T) {
		w := &Wiring{}
		a, err := w.NewSignalLevelAgent(context.Background(), "/net/connman/iwd/0/3", validConnectSignalConfig())
		require.Nil(t, a)
		require.Error(t, err)
		require.Contains(t, err.Error(), "D-Bus conn cannot be nil")
	})

	t.Run("relative station path", func(t *testing.T) {
		w := &Wiring{Conn: &dbus.Conn{}}
		a, err := w.NewSignalLevelAgent(context.Background(), "relative", validConnectSignalConfig())
		require.Nil(t, a)
		require.Error(t, err)
		require.Contains(t, err.Error(), "station path must be absolute")
	})
}

func TestWiring_NewSignalLevelAgent_UsesFactoryWhenPresent(t *testing.T) {
	want := &fakeConnectSignalLevelAgent{}
	w := &Wiring{
		SignalLevelAgentFactory: func(ctx context.Context, stationPath string, cfg core.SignalLevelConfig) (core.SignalLevelAgentIface, error) {
			require.Equal(t, "/net/connman/iwd/0/3", stationPath)
			require.NotNil(t, cfg.Changed)
			return want, nil
		},
	}
	got, err := w.NewSignalLevelAgent(context.Background(), "/net/connman/iwd/0/3", validConnectSignalConfig())
	require.NoError(t, err)
	require.Same(t, want, got)
}

func TestWiring_NewSignalLevelAgent_FailurePaths(t *testing.T) {
	origNewStation := newIwdStationFn
	origExport := exportSignalLevelAgentFn
	t.Cleanup(func() {
		newIwdStationFn = origNewStation
		exportSignalLevelAgentFn = origExport
	})

	t.Run("station_error", func(t *testing.T) {
		wantErr := errors.New("station boom")
		newIwdStationFn = func(ctx context.Context, conn *dbus.Conn, path dbus.ObjectPath) (*iwdbus.Station, error) {
			return nil, wantErr
		}
		exportSignalLevelAgentFn = origExport

		w := &Wiring{Conn: &dbus.Conn{}}
		a, err := w.NewSignalLevelAgent(context.Background(), "/net/connman/iwd/0/3", validConnectSignalConfig())
		require.Nil(t, a)
		require.ErrorIs(t, err, wantErr)
		require.Contains(t, err.Error(), "station unavailable")
	})

	t.Run("station_nil", func(t *testing.T) {
		// A nil (but errorless) iwd station means the Station interface is not
		// present; it must be reported as unavailable, not dereferenced.
		newIwdStationFn = func(ctx context.Context, conn *dbus.Conn, path dbus.ObjectPath) (*iwdbus.Station, error) {
			return nil, nil
		}
		exportSignalLevelAgentFn = origExport

		w := &Wiring{Conn: &dbus.Conn{}}
		a, err := w.NewSignalLevelAgent(context.Background(), "/net/connman/iwd/0/3", validConnectSignalConfig())
		require.Nil(t, a)
		require.Error(t, err)
		require.Contains(t, err.Error(), "iwd station interface not available")
	})

	t.Run("export_error", func(t *testing.T) {
		newIwdStationFn = func(ctx context.Context, conn *dbus.Conn, path dbus.ObjectPath) (*iwdbus.Station, error) {
			return &iwdbus.Station{}, nil
		}
		wantErr := errors.New("export failed")
		exportSignalLevelAgentFn = func(*dbus.Conn, dbus.ObjectPath, iwdbus.SignalLevelAgentHandler) (func() error, error) {
			return nil, wantErr
		}

		w := &Wiring{Conn: &dbus.Conn{}}
		a, err := w.NewSignalLevelAgent(context.Background(), "/net/connman/iwd/0/3", validConnectSignalConfig())
		require.Nil(t, a)
		require.ErrorIs(t, err, wantErr)
		require.Contains(t, err.Error(), "failed exporting signal level agent object")
	})

	t.Run("register_error_unexports", func(t *testing.T) {
		// A zero *iwdbus.Station is uninitialized, so RegisterSignalLevelAgent
		// fails - exercising the post-export cleanup path.
		newIwdStationFn = func(ctx context.Context, conn *dbus.Conn, path dbus.ObjectPath) (*iwdbus.Station, error) {
			return &iwdbus.Station{}, nil
		}
		var unexported bool
		exportSignalLevelAgentFn = func(*dbus.Conn, dbus.ObjectPath, iwdbus.SignalLevelAgentHandler) (func() error, error) {
			return func() error { unexported = true; return nil }, nil
		}

		w := &Wiring{Conn: &dbus.Conn{}}
		a, err := w.NewSignalLevelAgent(context.Background(), "/net/connman/iwd/0/3", validConnectSignalConfig())
		require.Nil(t, a)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed registering signal level agent")
		require.True(t, unexported, "export must be undone when registration fails")
	})
}
