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

func TestWiring_NewSimpleConfiguration_GuardsInputs(t *testing.T) {
	t.Run("nil wiring", func(t *testing.T) {
		var w *Wiring
		c, err := w.NewSimpleConfiguration(context.Background(), "/phy0/wlan0")
		require.Nil(t, c)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid state")
		require.Contains(t, err.Error(), "wiring cannot be nil")
	})

	t.Run("empty path", func(t *testing.T) {
		w := &Wiring{}
		c, err := w.NewSimpleConfiguration(context.Background(), "")
		require.Nil(t, c)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid argument")
		require.Contains(t, err.Error(), "simple configuration path cannot be empty")
	})

	t.Run("relative path", func(t *testing.T) {
		w := &Wiring{}
		c, err := w.NewSimpleConfiguration(context.Background(), "phy0/wlan0")
		require.Nil(t, c)
		require.Error(t, err)
		require.Contains(t, err.Error(), "simple configuration path must be absolute")
	})

	t.Run("nil connection without factory", func(t *testing.T) {
		w := &Wiring{}
		c, err := w.NewSimpleConfiguration(context.Background(), "/phy0/wlan0")
		require.Nil(t, c)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid state")
		require.Contains(t, err.Error(), "D-Bus conn cannot be nil")
	})
}

func TestWiring_NewSimpleConfiguration_UsesFactoryWhenPresent(t *testing.T) {
	want := &core.SimpleConfiguration{}
	w := &Wiring{
		SimpleConfigurationFactory: func(ctx context.Context, path string) (core.SimpleConfigurationIface, error) {
			require.Equal(t, "/phy0/wlan0", path)
			return want, nil
		},
	}
	got, err := w.NewSimpleConfiguration(context.Background(), "/phy0/wlan0")
	require.NoError(t, err)
	require.Same(t, want, got)
}

func TestWiring_NewSimpleConfiguration_FailurePaths(t *testing.T) {
	origNewIwd := newIwdSimpleConfigurationFn
	origNewCore := newCoreSimpleConfigurationFn
	t.Cleanup(func() {
		newIwdSimpleConfigurationFn = origNewIwd
		newCoreSimpleConfigurationFn = origNewCore
	})

	t.Run("new_iwd_config_error", func(t *testing.T) {
		wantErr := errors.New("wsc init failed")
		newIwdSimpleConfigurationFn = func(ctx context.Context, conn *dbus.Conn, path dbus.ObjectPath) (*iwdbus.SimpleConfiguration, error) {
			return nil, wantErr
		}

		w := &Wiring{Conn: &dbus.Conn{}}
		c, err := w.NewSimpleConfiguration(context.Background(), "/phy0/wlan0")
		require.Nil(t, c)
		require.ErrorIs(t, err, wantErr)
		require.Contains(t, err.Error(), "simple configuration unavailable")
	})

	t.Run("new_iwd_config_nil", func(t *testing.T) {
		newIwdSimpleConfigurationFn = func(ctx context.Context, conn *dbus.Conn, path dbus.ObjectPath) (*iwdbus.SimpleConfiguration, error) {
			return nil, nil
		}
		newCoreSimpleConfigurationFn = origNewCore

		w := &Wiring{Conn: &dbus.Conn{}}
		c, err := w.NewSimpleConfiguration(context.Background(), "/phy0/wlan0")
		require.Nil(t, c)
		require.Error(t, err)
		require.Contains(t, err.Error(), "iwd simple configuration interface not available")
	})

	t.Run("new_core_config_nil", func(t *testing.T) {
		newIwdSimpleConfigurationFn = func(ctx context.Context, conn *dbus.Conn, path dbus.ObjectPath) (*iwdbus.SimpleConfiguration, error) {
			return &iwdbus.SimpleConfiguration{}, nil
		}
		newCoreSimpleConfigurationFn = func(raw *iwdbus.SimpleConfiguration) *core.SimpleConfiguration {
			return nil
		}

		w := &Wiring{Conn: &dbus.Conn{}}
		c, err := w.NewSimpleConfiguration(context.Background(), "/phy0/wlan0")
		require.Nil(t, c)
		require.Error(t, err)
		require.Contains(t, err.Error(), "core simple configuration interface not available")
	})

	t.Run("success", func(t *testing.T) {
		newIwdSimpleConfigurationFn = func(ctx context.Context, conn *dbus.Conn, path dbus.ObjectPath) (*iwdbus.SimpleConfiguration, error) {
			return &iwdbus.SimpleConfiguration{}, nil
		}
		newCoreSimpleConfigurationFn = origNewCore

		w := &Wiring{Conn: &dbus.Conn{}}
		c, err := w.NewSimpleConfiguration(context.Background(), "/phy0/wlan0")
		require.NoError(t, err)
		require.NotNil(t, c)
	})
}
