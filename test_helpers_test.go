//go:build unit || race || stress

package spiderw

import (
	"context"
	"testing"

	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/require"

	"github.com/chrispypip/spiderw/internal/connect"
	"github.com/chrispypip/spiderw/internal/core"
)

type fakeCore struct {
	daemon  *fakeCoreDaemon
	adapter *fakeCoreAdapter
	device  *fakeCoreDevice
}

// newClientWithDaemon is a test-only constructor that wires a Client
// to a supplied daemon backend, bypassing real DBus usage.
//
// This function intentionally lives in *_test.go so it cannot be used
// by production code.
func newClientWithCore(fc *fakeCore) (*Client, error) {
	w := &connect.Wiring{
		Conn:             &dbus.Conn{},
		Daemon:           fc.daemon,
		ResolverOverride: connect.NoResolver{},
		Cleanup: func() error {
			return nil
		},
		AdapterFactory: func(ctx context.Context, path string) (core.AdapterIface, error) {
			return fc.adapter, nil
		},
		DeviceFactory: func(ctx context.Context, path string) (core.DeviceIface, error) {
			return fc.device, nil
		},
	}
	return newClientFromWiring(w)
}

// newTestClient returns a Client wired to a fake iwdbus backend.
// It mirrors the approach used in internal/core tests and ensures
// public API tests never touch the real system bus.
func newTestClient(t *testing.T) *Client {
	t.Helper()

	fakeDaemon := &fakeCoreDaemon{}
	fakeDaemon.setInfo(&core.DaemonInfo{
		Version:                     "1.0",
		StateDirectory:              "/var/lib/iwd",
		NetworkConfigurationEnabled: true,
	})
	fakeDaemon.setAdapters([]core.AdapterRef{
		{
			Path: "/phy0",
			Name: "phy0",
		},
	})
	fakeDaemon.setDevices([]core.DeviceRef{
		{
			Path: "/net/connman/iwd/phy0/wlan0",
			Name: "wlan0",
		},
	})
	mockModel := "MockModel"
	mockVendor := "MockVendor"
	fakeAdapter := &fakeCoreAdapter{}
	fakeAdapter.powered.Store(true)
	fakeAdapter.name.Store("phy0")
	fakeAdapter.model.Store(&mockModel)
	fakeAdapter.vendor.Store(&mockVendor)
	fakeAdapter.modes.Store([]core.Mode{core.ModeStation, core.ModeAP})

	fakeDevice := &fakeCoreDevice{}
	fakeDevice.name.Store("wlan0")
	fakeDevice.address.Store("aa:bb:cc:dd:ee:ff")
	fakeDevice.powered.Store(true)
	fakeDevice.mode.Store(core.ModeStation)
	fakeDevice.adapter.Store("/net/connman/iwd/phy0")

	fake := &fakeCore{
		daemon:  fakeDaemon,
		adapter: fakeAdapter,
		device:  fakeDevice,
	}

	c, err := newClientWithCore(fake)
	require.NoError(t, err)

	return c
}

func newTestAdapter(t *testing.T) *Adapter {
	t.Helper()
	client := newTestClient(t)
	refs, err := client.Daemon().Adapters(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, refs)
	a, err := client.Adapter(context.Background(), refs[0].Path)
	require.NoError(t, err)
	return a
}

func newTestDevice(t *testing.T) *Device {
	t.Helper()
	client := newTestClient(t)
	refs, err := client.Daemon().Devices(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, refs)
	d, err := client.Device(context.Background(), refs[0].Path)
	require.NoError(t, err)
	return d
}
