//go:build integration

package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/require"

	"github.com/chrispypip/spiderw"
	"github.com/chrispypip/spiderw/internal/iwdbus"
	"github.com/chrispypip/spiderw/tests/testutil/iwdmock"
)

const devicePath = "/net/connman/iwd/phy0/wlan0"

// newTestDevice builds a raw iwdbus.Device bound to the mock device path.
func newTestDevice(t *testing.T) (*iwdbus.Device, *dbus.Conn) {
	t.Helper()

	conn, err := dbus.SessionBus()
	require.NoError(t, err)

	device, err := iwdbus.NewDevice(context.Background(), conn, devicePath)
	require.NoError(t, err)

	t.Cleanup(func() { _ = conn.Close() })

	return device, conn
}

// newPublicMockDevice resolves a public Device handle for the named mock device.
func newPublicMockDevice(t *testing.T, ctx context.Context, client *spiderw.Client, name string) *spiderw.Device {
	t.Helper()

	daemon := client.Daemon()
	require.NotNil(t, daemon)

	refs, err := daemon.Devices(ctx)
	require.NoError(t, err)

	for _, ref := range refs {
		if ref.Name != name {
			continue
		}

		device, err := client.Device(ctx, ref.Path)
		require.NoError(t, err)
		require.NotNil(t, device)
		return device
	}

	t.Fatalf("mock device %q not found in refs: %#v", name, refs)
	return nil
}

// -----------------------------------------------------------------------------
// Raw iwdbus.Device against the mock
// -----------------------------------------------------------------------------

func TestDeviceMock_Getters(t *testing.T) {
	tmpDir := t.TempDir()
	iwdmock.StartMockNormal(t, tmpDir)

	ctx := context.Background()
	device, _ := newTestDevice(t)

	name, err := device.GetName(ctx)
	require.NoError(t, err)
	require.Equal(t, "wlan0", name)

	addr, err := device.GetAddress(ctx)
	require.NoError(t, err)
	require.Equal(t, "aa:bb:cc:dd:ee:ff", addr)

	powered, err := device.GetPowered(ctx)
	require.NoError(t, err)
	require.True(t, powered)

	mode, err := device.GetMode(ctx)
	require.NoError(t, err)
	require.Equal(t, iwdbus.ModeStation, mode)

	adapter, err := device.GetAdapter(ctx)
	require.NoError(t, err)
	require.Equal(t, dbus.ObjectPath("/net/connman/iwd/phy0"), adapter)
}

func TestDeviceMock_GetProperties(t *testing.T) {
	tmpDir := t.TempDir()
	iwdmock.StartMockNormal(t, tmpDir)

	device, _ := newTestDevice(t)

	props, err := device.GetProperties(context.Background())
	require.NoError(t, err)
	require.Equal(t, "wlan0", props.Name)
	require.Equal(t, "aa:bb:cc:dd:ee:ff", props.Address)
	require.True(t, props.Powered)
	require.Equal(t, iwdbus.ModeStation, props.Mode)
	require.Equal(t, dbus.ObjectPath("/net/connman/iwd/phy0"), props.Adapter)
}

func TestDeviceMock_SetPowered(t *testing.T) {
	tmpDir := t.TempDir()
	iwdmock.StartMockNormal(t, tmpDir)

	ctx := context.Background()
	device, _ := newTestDevice(t)

	require.NoError(t, device.SetPowered(ctx, false))

	powered, err := device.GetPowered(ctx)
	require.NoError(t, err)
	require.False(t, powered)
}

func TestDeviceMock_SetMode(t *testing.T) {
	tmpDir := t.TempDir()
	iwdmock.StartMockNormal(t, tmpDir)

	ctx := context.Background()
	device, _ := newTestDevice(t)

	require.NoError(t, device.SetMode(ctx, iwdbus.ModeAP))

	mode, err := device.GetMode(ctx)
	require.NoError(t, err)
	require.Equal(t, iwdbus.ModeAP, mode)
}

func TestDeviceMock_SubscribePoweredChanged(t *testing.T) {
	tmpDir := t.TempDir()
	iwdmock.StartMockNormal(t, tmpDir)

	ctx := context.Background()
	device, _ := newTestDevice(t)

	received := make(chan bool, 2)
	unsubscribe, err := device.SubscribePoweredChanged(ctx, func(powered bool) {
		received <- powered
	})
	require.NoError(t, err)

	require.NoError(t, device.SetPowered(ctx, false))
	select {
	case got := <-received:
		require.False(t, got)
	case <-time.After(signalTimeout):
		t.Fatal("timed out waiting for powered=false callback")
	}

	require.NoError(t, unsubscribe.Unsubscribe())
}

func TestDeviceMock_SubscribeModeChanged(t *testing.T) {
	tmpDir := t.TempDir()
	iwdmock.StartMockNormal(t, tmpDir)

	ctx := context.Background()
	device, _ := newTestDevice(t)

	received := make(chan iwdbus.Mode, 2)
	_, err := device.SubscribeModeChanged(ctx, func(mode iwdbus.Mode) {
		received <- mode
	})
	require.NoError(t, err)

	require.NoError(t, device.SetMode(ctx, iwdbus.ModeAP))
	select {
	case got := <-received:
		require.Equal(t, iwdbus.ModeAP, got)
	case <-time.After(signalTimeout):
		t.Fatal("timed out waiting for mode=ap callback")
	}
}

func TestDeviceMock_Firehose(t *testing.T) {
	tmpDir := t.TempDir()
	iwdmock.StartMockNormal(t, tmpDir)

	device, _ := newTestDevice(t)

	var recv iwdbus.FirehoseSignal
	fired := make(chan struct{}, 1)

	err := device.Firehose(context.Background(), func(sig iwdbus.FirehoseSignal) {
		recv = sig
		fired <- struct{}{}
	})
	require.NoError(t, err)

	m, out, err := runSpiderDeviceJSON(t, "wlan0", "powered", "off")
	require.NoError(t, err, "output:\n%s", out)
	require.Equal(t, false, jsonGetBool(t, m, "Powered"))

	changed := map[string]dbus.Variant{"Powered": dbus.MakeVariant(false)}

	requireFired(t, fired)

	require.Equal(t, "org.freedesktop.DBus.Properties", recv.Interface)
	require.Equal(t, "PropertiesChanged", recv.Member)
	require.Len(t, recv.Body, 3)

	s, ok := recv.Body[0].(string)
	require.True(t, ok)

	v, ok := recv.Body[1].(map[string]dbus.Variant)
	require.True(t, ok)

	ss, ok := recv.Body[2].([]string)
	require.True(t, ok)

	require.Equal(t, iwdbus.IwdDeviceIface, s)
	require.Equal(t, changed, v)
	require.Equal(t, []string{}, ss)
}

// -----------------------------------------------------------------------------
// Public client against the mock
// -----------------------------------------------------------------------------

func TestDeviceMock_DaemonDevices(t *testing.T) {
	tmpDir := t.TempDir()
	iwdmock.StartMockNormal(t, tmpDir)

	ctx := context.Background()
	client, err := spiderw.NewClient(ctx, spiderw.SessionBus)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })

	refs, err := client.Daemon().Devices(ctx)
	require.NoError(t, err)
	require.Equal(t, []spiderw.DeviceRef{
		{Path: devicePath, Name: "wlan0"},
	}, refs)
}

func TestDeviceMock_Properties(t *testing.T) {
	tmpDir := t.TempDir()
	iwdmock.StartMockNormal(t, tmpDir)

	ctx := context.Background()
	client, err := spiderw.NewClient(ctx, spiderw.SessionBus)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })

	d := newPublicMockDevice(t, ctx, client, "wlan0")

	props, err := d.Properties(ctx)
	require.NoError(t, err)
	require.Equal(t, "wlan0", props.Name)
	require.Equal(t, "aa:bb:cc:dd:ee:ff", props.Address)
	require.True(t, props.Powered)
	require.Equal(t, spiderw.ModeStation, props.Mode)
	require.Equal(t, "/net/connman/iwd/phy0", props.Adapter)
}

func TestDeviceMock_AllDevices(t *testing.T) {
	tmpDir := t.TempDir()
	iwdmock.StartMockNormal(t, tmpDir)

	ctx := context.Background()
	client, err := spiderw.NewClient(ctx, spiderw.SessionBus)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })

	devices, err := client.AllDevices(ctx)
	require.NoError(t, err)
	require.Len(t, devices, 1)
	require.Equal(t, devicePath, devices[0].Path())

	name, err := devices[0].Name(ctx)
	require.NoError(t, err)
	require.Equal(t, "wlan0", name)
}

func TestDeviceMock_AllDevices_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	iwdmock.StartMockWithoutDevice(t, tmpDir)

	ctx := context.Background()
	client, err := spiderw.NewClient(ctx, spiderw.SessionBus)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })

	refs, err := client.Daemon().Devices(ctx)
	require.NoError(t, err)
	require.Empty(t, refs)

	devices, err := client.AllDevices(ctx)
	require.NoError(t, err)
	require.Empty(t, devices)
}

// -----------------------------------------------------------------------------
// CLI (`spiderw device …`) against the mock
// -----------------------------------------------------------------------------

func findDeviceStatusEntry(t *testing.T, list []map[string]any, path string) map[string]any {
	t.Helper()

	for _, entry := range list {
		if p, ok := entry["Path"].(string); ok && p == path {
			return entry
		}
	}

	t.Fatalf("device %q not found in status output: %#v", path, list)
	return nil
}

func runSpiderDevice(t *testing.T, args ...string) (string, error) {
	t.Helper()

	return runSpider(t, append([]string{"device"}, args...)...)
}

func runSpiderDeviceJSON(t *testing.T, args ...string) (map[string]any, string, error) {
	t.Helper()

	return runSpiderJSON(t, append([]string{"device"}, args...)...)
}

func TestDeviceMock_CLI_GetPowered(t *testing.T) {
	tmpDir := t.TempDir()
	iwdmock.StartMockNormal(t, tmpDir)

	m, out, err := runSpiderDeviceJSON(t, "wlan0", "powered")
	require.NoError(t, err, "output:\n%s", out)
	require.Equal(t, true, jsonGetBool(t, m, "Powered"))
}

func TestDeviceMock_CLI_SetPowered(t *testing.T) {
	tmpDir := t.TempDir()
	iwdmock.StartMockNormal(t, tmpDir)

	m, out, err := runSpiderDeviceJSON(t, "wlan0", "powered", "off")
	require.NoError(t, err, "output:\n%s", out)
	require.Equal(t, false, jsonGetBool(t, m, "Powered"))
}

func TestDeviceMock_CLI_GetName(t *testing.T) {
	tmpDir := t.TempDir()
	iwdmock.StartMockNormal(t, tmpDir)

	m, out, err := runSpiderDeviceJSON(t, "wlan0", "name")
	require.NoError(t, err, "output:\n%s", out)
	require.Equal(t, "wlan0", jsonGetString(t, m, "Value"))
}

func TestDeviceMock_CLI_GetAddress(t *testing.T) {
	tmpDir := t.TempDir()
	iwdmock.StartMockNormal(t, tmpDir)

	m, out, err := runSpiderDeviceJSON(t, "wlan0", "address")
	require.NoError(t, err, "output:\n%s", out)
	require.Equal(t, "aa:bb:cc:dd:ee:ff", jsonGetString(t, m, "Value"))
}

func TestDeviceMock_CLI_GetAdapter(t *testing.T) {
	tmpDir := t.TempDir()
	iwdmock.StartMockNormal(t, tmpDir)

	m, out, err := runSpiderDeviceJSON(t, "wlan0", "adapter")
	require.NoError(t, err, "output:\n%s", out)
	require.Equal(t, "/net/connman/iwd/phy0", jsonGetString(t, m, "Value"))
}

func TestDeviceMock_CLI_GetMode(t *testing.T) {
	tmpDir := t.TempDir()
	iwdmock.StartMockNormal(t, tmpDir)

	m, out, err := runSpiderDeviceJSON(t, "wlan0", "mode")
	require.NoError(t, err, "output:\n%s", out)
	require.Equal(t, "station", jsonGetString(t, m, "Mode"))
}

func TestDeviceMock_CLI_SetMode(t *testing.T) {
	tmpDir := t.TempDir()
	iwdmock.StartMockNormal(t, tmpDir)

	m, out, err := runSpiderDeviceJSON(t, "wlan0", "mode", "ap")
	require.NoError(t, err, "output:\n%s", out)
	require.Equal(t, "ap", jsonGetString(t, m, "Mode"))
}

func TestDeviceMock_CLI_UnknownCommand(t *testing.T) {
	tmpDir := t.TempDir()
	iwdmock.StartMockNormal(t, tmpDir)

	out, err := runSpiderDevice(t, "wlan0", "bogus")
	require.Error(t, err, "output:\n%s", out)
	mustContain(t, out, "unknown device command")
}

// TestDeviceMock_Status exercises `device status`, which drives
// Client.AllDevices: it constructs a handle per device and reports the full
// per-device snapshot (path, name, address, powered, mode, adapter).
func TestDeviceMock_Status(t *testing.T) {
	tmpDir := t.TempDir()
	iwdmock.StartMockNormal(t, tmpDir)

	out, err := runSpiderDevice(t, "status")
	require.NoError(t, err, "output:\n%s", out)

	mustContainAll(t, out, []string{
		"Name:",
		"wlan0",
		"Path:",
		devicePath,
		"Address:",
		"aa:bb:cc:dd:ee:ff",
		"Powered:",
		"true",
		"Mode:",
		"station",
		"Adapter:",
		"/net/connman/iwd/phy0",
	})
}

// TestDeviceMock_StatusJSON verifies the structured `device status --json`
// output, which is a JSON array of per-device snapshots.
func TestDeviceMock_StatusJSON(t *testing.T) {
	tmpDir := t.TempDir()
	iwdmock.StartMockNormal(t, tmpDir)

	list, out, err := runSpiderJSONArray(t, "device", "status")
	require.NoError(t, err, "output:\n%s", out)
	require.NotEmpty(t, list, "expected at least one device:\n%s", out)

	entry := findDeviceStatusEntry(t, list, devicePath)
	require.Equal(t, "wlan0", jsonGetString(t, entry, "Name"))
	require.Equal(t, "aa:bb:cc:dd:ee:ff", jsonGetString(t, entry, "Address"))
	require.Equal(t, true, jsonGetBool(t, entry, "Powered"))
	require.Equal(t, "station", jsonGetString(t, entry, "Mode"))
	require.Equal(t, "/net/connman/iwd/phy0", jsonGetString(t, entry, "Adapter"))
}

// TestDeviceMock_List exercises `device list`, which drives Daemon.Devices.
func TestDeviceMock_List(t *testing.T) {
	tmpDir := t.TempDir()
	iwdmock.StartMockNormal(t, tmpDir)

	out, err := runSpiderDevice(t, "list")
	require.NoError(t, err, "output:\n%s", out)

	mustContainAll(t, out, []string{"wlan0", devicePath})
}

// TestDeviceMock_StatusEmpty verifies status renders an empty enumeration
// without failing.
func TestDeviceMock_StatusEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	iwdmock.StartMockWithoutDevice(t, tmpDir)

	out, err := runSpiderDevice(t, "status")
	require.NoError(t, err, "output:\n%s", out)
	mustContain(t, out, "no devices available")
}
