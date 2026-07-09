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

func TestWiring_NewDevice_GuardsInputs(t *testing.T) {
	t.Run("nil wiring", func(t *testing.T) {
		var w *Wiring
		d, err := w.NewDevice(context.Background(), "/phy0/wlan0")
		require.Nil(t, d)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid state")
		require.Contains(t, err.Error(), "wiring cannot be nil")
	})

	t.Run("empty path", func(t *testing.T) {
		w := &Wiring{}
		d, err := w.NewDevice(context.Background(), "")
		require.Nil(t, d)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid argument")
		require.Contains(t, err.Error(), "device path cannot be empty")
	})

	t.Run("relative path", func(t *testing.T) {
		w := &Wiring{}
		d, err := w.NewDevice(context.Background(), "phy0/wlan0")
		require.Nil(t, d)
		require.Error(t, err)
		require.Contains(t, err.Error(), "device path must be absolute")
	})

	t.Run("nil connection without factory", func(t *testing.T) {
		w := &Wiring{}
		d, err := w.NewDevice(context.Background(), "/phy0/wlan0")
		require.Nil(t, d)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid state")
		require.Contains(t, err.Error(), "D-Bus conn cannot be nil")
	})
}

func TestWiring_NewDevice_UsesFactoryWhenPresent(t *testing.T) {
	want := &core.Device{}
	w := &Wiring{
		DeviceFactory: func(ctx context.Context, path string) (core.DeviceIface, error) {
			require.Equal(t, "/phy0/wlan0", path)
			return want, nil
		},
	}
	got, err := w.NewDevice(context.Background(), "/phy0/wlan0")
	require.NoError(t, err)
	require.Same(t, want, got)
}

func TestWiring_NewDevice_FailurePaths(t *testing.T) {
	origNewIwdDevice := newIwdDeviceFn
	origNewCoreDevice := newCoreDeviceFn
	t.Cleanup(func() {
		newIwdDeviceFn = origNewIwdDevice
		newCoreDeviceFn = origNewCoreDevice
	})

	t.Run("new_iwd_device_error", func(t *testing.T) {
		wantErr := errors.New("device init failed")
		newIwdDeviceFn = func(ctx context.Context, conn *dbus.Conn, path dbus.ObjectPath) (*iwdbus.Device, error) {
			return nil, wantErr
		}

		w := &Wiring{Conn: &dbus.Conn{}}
		d, err := w.NewDevice(context.Background(), "/phy0/wlan0")
		require.Nil(t, d)
		require.ErrorIs(t, err, wantErr)
	})

	t.Run("new_iwd_device_nil", func(t *testing.T) {
		newIwdDeviceFn = func(ctx context.Context, conn *dbus.Conn, path dbus.ObjectPath) (*iwdbus.Device, error) {
			return nil, nil
		}
		newCoreDeviceFn = origNewCoreDevice

		w := &Wiring{Conn: &dbus.Conn{}}
		d, err := w.NewDevice(context.Background(), "/phy0/wlan0")
		require.Nil(t, d)
		require.Error(t, err)
		require.Contains(t, err.Error(), "iwd device interface not available")
	})

	t.Run("new_core_device_nil", func(t *testing.T) {
		newIwdDeviceFn = func(ctx context.Context, conn *dbus.Conn, path dbus.ObjectPath) (*iwdbus.Device, error) {
			return &iwdbus.Device{}, nil
		}
		newCoreDeviceFn = func(raw *iwdbus.Device) *core.Device {
			return nil
		}

		w := &Wiring{Conn: &dbus.Conn{}}
		d, err := w.NewDevice(context.Background(), "/phy0/wlan0")
		require.Nil(t, d)
		require.Error(t, err)
		require.Contains(t, err.Error(), "core device interface not available")
	})

	t.Run("success", func(t *testing.T) {
		newIwdDeviceFn = func(ctx context.Context, conn *dbus.Conn, path dbus.ObjectPath) (*iwdbus.Device, error) {
			return &iwdbus.Device{}, nil
		}
		newCoreDeviceFn = origNewCoreDevice

		w := &Wiring{Conn: &dbus.Conn{}}
		d, err := w.NewDevice(context.Background(), "/phy0/wlan0")
		require.NoError(t, err)
		require.NotNil(t, d)
	})
}
