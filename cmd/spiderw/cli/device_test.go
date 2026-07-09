//go:build unit

package cli

import (
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/chrispypip/spiderw"
)

func fakeWithDevice() *fakeClient {
	dev := &fakeDevice{
		path: "/net/connman/iwd/phy0/wlan0",
		props: &spiderw.DeviceProperties{
			Name:    "wlan0",
			Address: "aa:bb:cc:dd:ee:ff",
			Powered: true,
			Mode:    spiderw.ModeStation,
			Adapter: spiderw.AdapterRef{Path: "/net/connman/iwd/phy0", Name: "phy0"},
		},
	}
	return &fakeClient{
		daemon:     &fakeDaemon{devices: []spiderw.DeviceRef{{Path: dev.path, Name: "wlan0"}}},
		devices:    map[string]deviceAPI{dev.path: dev},
		allDevices: []deviceAPI{dev},
	}
}

func TestDeviceCmd_Status_Human(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithDevice(), nil, false, "device", "status")
	require.Equal(t, 0, code, out)
	for _, want := range []string{"wlan0", "aa:bb:cc:dd:ee:ff", "station", "/net/connman/iwd/phy0/wlan0", "/net/connman/iwd/phy0"} {
		require.Contains(t, out, want)
	}
}

func TestDeviceCmd_Status_JSON(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithDevice(), nil, true, "device", "status")
	require.Equal(t, 0, code, out)

	var list []map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &list))
	require.Len(t, list, 1)
	require.Equal(t, "wlan0", list[0]["Name"])
	require.Equal(t, "aa:bb:cc:dd:ee:ff", list[0]["Address"])
	require.Equal(t, "station", list[0]["Mode"])
	require.Equal(t, map[string]any{"Name": "phy0", "Path": "/net/connman/iwd/phy0"}, list[0]["Adapter"])
}

func TestDeviceCmd_ScopedStatus_JSON(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithDevice(), nil, true, "device", "wlan0", "status")
	require.Equal(t, 0, code, out)

	var list []map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &list))
	require.Len(t, list, 1)

	entry := list[0]
	require.Equal(t, "/net/connman/iwd/phy0/wlan0", entry["Path"])
	require.Equal(t, "wlan0", entry["Name"])
	require.Equal(t, "aa:bb:cc:dd:ee:ff", entry["Address"])
	require.Equal(t, true, entry["Powered"])
	require.Equal(t, "station", entry["Mode"])
	require.Equal(t, map[string]any{"Name": "phy0", "Path": "/net/connman/iwd/phy0"}, entry["Adapter"])
}

func TestDeviceCmd_ScopedStatus_UsageError(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithDevice(), nil, false, "device", "wlan0", "status", "extra")
	require.Equal(t, 1, code)
	require.Contains(t, out, "usage: spiderw device <device> status")
	require.NotContains(t, out, "Commands:")
}

func TestDeviceCmd_Status_BackendError(t *testing.T) {
	t.Parallel()

	fc := &fakeClient{allDeviceErr: errors.New("enumeration boom")}
	out, code := driveCLI(fc, nil, false, "device", "status")
	require.Equal(t, 1, code)
	require.Contains(t, out, "enumeration boom")
}

func TestDeviceCmd_Status_UsageError(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithDevice(), nil, false, "device", "status", "extra")
	require.Equal(t, 1, code)
	require.Contains(t, out, "unknown device status argument")
}

func TestDeviceCmd_List(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithDevice(), nil, false, "device", "list")
	require.Equal(t, 0, code, out)
	require.Contains(t, out, "wlan0")
	require.Contains(t, out, "/net/connman/iwd/phy0/wlan0")
}

func TestDeviceCmd_PoweredGetAndSet(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithDevice(), nil, false, "device", "wlan0", "powered")
	require.Equal(t, 0, code, out)
	require.Equal(t, "true\n", out)

	// Set re-reads after writing; the fake mutates its stored value.
	out, code = driveCLI(fakeWithDevice(), nil, false, "device", "wlan0", "powered", "off")
	require.Equal(t, 0, code, out)
	require.Equal(t, "false\n", out)
}

func TestDeviceCmd_ModeGetAndSet(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithDevice(), nil, false, "device", "wlan0", "mode")
	require.Equal(t, 0, code, out)
	require.Equal(t, "station\n", out)

	out, code = driveCLI(fakeWithDevice(), nil, false, "device", "wlan0", "mode", "ap")
	require.Equal(t, 0, code, out)
	require.Equal(t, "ap\n", out)
}

func TestDeviceCmd_StringAccessors(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithDevice(), nil, false, "device", "wlan0", "name")
	require.Equal(t, 0, code, out)
	require.Equal(t, "wlan0\n", out)

	out, code = driveCLI(fakeWithDevice(), nil, false, "device", "wlan0", "address")
	require.Equal(t, 0, code, out)
	require.Equal(t, "aa:bb:cc:dd:ee:ff\n", out)

	out, code = driveCLI(fakeWithDevice(), nil, false, "device", "wlan0", "adapter")
	require.Equal(t, 0, code, out)
	require.Equal(t, "/net/connman/iwd/phy0\n", out)
}

func TestDeviceCmd_RefNotFound(t *testing.T) {
	t.Parallel()

	// Daemon reports no devices, so resolving "wlan0" fails.
	fc := &fakeClient{daemon: &fakeDaemon{}}
	out, code := driveCLI(fc, nil, false, "device", "wlan0", "powered")
	require.Equal(t, 1, code)
	require.Contains(t, out, "no devices available")
}

func TestDeviceCmd_UnknownCommand(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithDevice(), nil, false, "device", "wlan0", "bogus")
	require.Equal(t, 1, code)
	require.Contains(t, out, "unknown device command")
	require.Contains(t, out, "Commands:")
	require.Contains(t, out, "<device> powered")
}

func TestDeviceCmd_MissingNestedCommandPrintsHelp(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithDevice(), nil, false, "device", "wlan0")
	require.Equal(t, 1, code)
	require.Contains(t, out, "Usage:")
	require.Contains(t, out, "spiderw device <command>")
	require.Contains(t, out, "Commands:")
	require.Contains(t, out, "<device> mode")
	require.Contains(t, out, "missing device command for \"wlan0\"")
}

func TestDeviceCmd_LeafUsageErrorStaysFocused(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithDevice(), nil, false, "device", "wlan0", "mode", "ap", "extra")
	require.Equal(t, 1, code)
	require.Contains(t, out, "usage: spiderw device <device> mode [station|ap|ad-hoc]")
	require.NotContains(t, out, "Commands:")
}

func TestDeviceCmd_ClientConstructionError(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(nil, errors.New("no session bus"), false, "device", "status")
	require.Equal(t, 1, code)
	require.Contains(t, out, "no session bus")
}

func TestDeviceCmd_ClosesClient(t *testing.T) {
	t.Parallel()

	fc := fakeWithDevice()
	_, code := driveCLI(fc, nil, false, "device", "status")
	require.Equal(t, 0, code)
	require.True(t, fc.closed, "command should close the client")
}

func TestDeviceListResult_String(t *testing.T) {
	t.Parallel()

	require.Equal(t, "no devices available", deviceListResult{}.String())

	out := deviceListResult{
		{Path: "/net/connman/iwd/phy0/wlan0", Name: "wlan0"},
		{Path: "/net/connman/iwd/phy1/wlan1", Name: ""},
	}.String()
	require.Equal(t, "wlan0\t/net/connman/iwd/phy0/wlan0\n/net/connman/iwd/phy1/wlan1", out)
}

func TestDeviceScalarResults_String(t *testing.T) {
	t.Parallel()

	require.Equal(t, "true", devicePoweredResult{Powered: true}.String())
	require.Equal(t, "false", devicePoweredResult{Powered: false}.String())
	require.Equal(t, "station", deviceModeResult{Mode: "station"}.String())
	require.Equal(t, "aa:bb:cc:dd:ee:ff", deviceStringResult{Value: "aa:bb:cc:dd:ee:ff"}.String())
}

func TestDeviceStatusResult_String(t *testing.T) {
	t.Parallel()

	require.Equal(t, "no devices available", deviceStatusResult{}.String())

	out := deviceStatusResult{
		{
			Path:    "/net/connman/iwd/phy0/wlan0",
			Name:    "wlan0",
			Address: "aa:bb:cc:dd:ee:ff",
			Powered: true,
			Mode:    "station",
			Adapter: nameRef{Path: "/net/connman/iwd/phy0", Name: "phy0"},
		},
	}.String()

	require.Contains(t, out, "Name:")
	require.Contains(t, out, "wlan0")
	require.Contains(t, out, "Path:")
	require.Contains(t, out, "/net/connman/iwd/phy0/wlan0")
	require.Contains(t, out, "Address:")
	require.Contains(t, out, "aa:bb:cc:dd:ee:ff")
	require.Contains(t, out, "Powered:")
	require.Contains(t, out, "true")
	require.Contains(t, out, "Mode:")
	require.Contains(t, out, "station")
	require.Contains(t, out, "Adapter:")
	require.Contains(t, out, "/net/connman/iwd/phy0")
	require.NotContains(t, out, "<nil>")
}

func TestDeviceStatusResult_String_MultipleEntriesSeparated(t *testing.T) {
	t.Parallel()

	out := deviceStatusResult{
		{Path: "/p0/w0", Name: "wlan0"},
		{Path: "/p1/w1", Name: "wlan1"},
	}.String()

	require.Equal(t, 2, strings.Count(out, "Name:"))
	require.Contains(t, out, "\n\n")
}

func TestDeviceStatusResult_String_UnnamedAndEmptyFields(t *testing.T) {
	t.Parallel()

	out := deviceStatusResult{
		{Path: "/p0/w0", Name: "", Address: "", Mode: "", Adapter: nameRef{}},
	}.String()

	require.Contains(t, out, "(unnamed)")
	// Empty Address/Mode/Adapter render as "-".
	require.Contains(t, out, "Address:")
	require.Contains(t, out, "-")
}

// TestPrintDeviceLines covers the monitor output helpers directly (the monitor
// command blocks on an OS signal and is not drivable in-process).
func TestPrintDeviceLines(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex

	t.Run("Powered_Human", func(t *testing.T) {
		app, buf := appWithBuffer(false)
		require.NoError(t, printDevicePoweredLine(app, "wlan0", false, &mu))
		require.Equal(t, "powered=false\n", buf.String())
	})
	t.Run("Powered_JSON", func(t *testing.T) {
		app, buf := appWithBuffer(true)
		require.NoError(t, printDevicePoweredLine(app, "wlan0", true, &mu))
		require.Contains(t, buf.String(), `"Powered":true`)
	})
	t.Run("Mode_Human", func(t *testing.T) {
		app, buf := appWithBuffer(false)
		require.NoError(t, printDeviceModeLine(app, "wlan0", "ap", &mu))
		require.Equal(t, "mode=ap\n", buf.String())
	})
	t.Run("Mode_JSON", func(t *testing.T) {
		app, buf := appWithBuffer(true)
		require.NoError(t, printDeviceModeLine(app, "wlan0", "station", &mu))
		require.Contains(t, buf.String(), `"Mode":"station"`)
	})
}
