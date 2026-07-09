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

func TestParseBoolArg(t *testing.T) {
	t.Parallel()

	for _, in := range []string{"true", "1", "yes", "y", "on", "enable", "enabled", "  TRUE  ", "On"} {
		got, err := parseBoolArg(in)
		require.NoError(t, err, in)
		require.True(t, got, in)
	}
	for _, in := range []string{"false", "0", "no", "n", "off", "disable", "disabled", "  FALSE ", "Off"} {
		got, err := parseBoolArg(in)
		require.NoError(t, err, in)
		require.False(t, got, in)
	}
	for _, in := range []string{"maybe", "", "2", "yep"} {
		_, err := parseBoolArg(in)
		require.Error(t, err, in)
		require.Contains(t, err.Error(), "invalid boolean value")
	}
}

func fakeWithKnownNetwork() *fakeClient {
	known := &fakeKnownNetwork{
		path: "/net/connman/iwd/known_networks/1",
		props: &spiderw.KnownNetworkProperties{
			Name:              "KnownNet",
			Type:              spiderw.NetworkTypePSK,
			Hidden:            false,
			LastConnectedTime: new("2024-01-02T03:04:05Z"),
			AutoConnect:       true,
		},
	}
	hotspot := &fakeKnownNetwork{
		path: "/net/connman/iwd/known_networks/2",
		props: &spiderw.KnownNetworkProperties{
			Name:        "GuestHotspot",
			Type:        spiderw.NetworkTypeHotspot,
			AutoConnect: false,
		},
	}
	return &fakeClient{
		daemon: &fakeDaemon{knownNetworks: []spiderw.KnownNetworkRef{
			{Path: known.path, Name: "KnownNet"},
			{Path: hotspot.path, Name: "GuestHotspot"},
		}},
		knownNetworks: map[string]knownNetworkAPI{known.path: known, hotspot.path: hotspot},
		allKnownNets:  []knownNetworkAPI{known, hotspot},
	}
}

func TestKnownNetworkCmd_Status_JSON(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithKnownNetwork(), nil, true, "known-network", "status")
	require.Equal(t, 0, code, out)

	var entries []knownNetworkStatusEntry
	require.NoError(t, json.Unmarshal([]byte(out), &entries))
	require.Len(t, entries, 2)
	require.Equal(t, "KnownNet", entries[0].Name)
	require.Equal(t, "psk", entries[0].Type)
	require.NotNil(t, entries[0].LastConnectedTime)
	require.Equal(t, "hotspot", entries[1].Type)
	require.Nil(t, entries[1].LastConnectedTime)
}

func TestKnownNetworkCmd_Status_Human(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithKnownNetwork(), nil, false, "known-network", "status")
	require.Equal(t, 0, code, out)
	require.Contains(t, out, "KnownNet")
	require.Contains(t, out, "GuestHotspot")
	require.Contains(t, out, "psk")
}

func TestKnownNetworkCmd_SingleStatus(t *testing.T) {
	t.Parallel()

	t.Run("Human", func(t *testing.T) {
		out, code := driveCLI(fakeWithKnownNetwork(), nil, false, "known-network", "KnownNet", "status")
		require.Equal(t, 0, code, out)
		require.Contains(t, out, "KnownNet")
		require.Contains(t, out, "psk")
	})

	t.Run("JSON", func(t *testing.T) {
		out, code := driveCLI(fakeWithKnownNetwork(), nil, true, "known-network", "KnownNet", "status")
		require.Equal(t, 0, code, out)
		var entries []knownNetworkStatusEntry
		require.NoError(t, json.Unmarshal([]byte(out), &entries))
		require.Len(t, entries, 1)
		require.Equal(t, "KnownNet", entries[0].Name)
	})
}

func TestKnownNetworkCmd_Hidden(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithKnownNetwork(), nil, false, "known-network", "KnownNet", "hidden")
	require.Equal(t, 0, code, out)
	require.Equal(t, "false\n", out)
}

func TestKnownNetworkCmd_List(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithKnownNetwork(), nil, false, "known-network", "list")
	require.Equal(t, 0, code, out)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	require.Len(t, lines, 2)
	require.Contains(t, out, "KnownNet")
	require.Contains(t, out, "GuestHotspot")
}

func TestKnownNetworkCmd_Type(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithKnownNetwork(), nil, false, "known-network", "GuestHotspot", "type")
	require.Equal(t, 0, code, out)
	require.Equal(t, "hotspot", strings.TrimSpace(out))
}

func TestKnownNetworkCmd_AutoConnect_Get(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithKnownNetwork(), nil, false, "known-network", "KnownNet", "autoconnect")
	require.Equal(t, 0, code, out)
	require.Equal(t, "true", strings.TrimSpace(out))
}

func TestKnownNetworkCmd_AutoConnect_Set(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithKnownNetwork(), nil, false, "known-network", "KnownNet", "autoconnect", "off")
	require.Equal(t, 0, code, out)
	require.Equal(t, "false", strings.TrimSpace(out))
}

func TestKnownNetworkCmd_AutoConnect_InvalidValue(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithKnownNetwork(), nil, false, "known-network", "KnownNet", "autoconnect", "maybe")
	require.Equal(t, 1, code)
	require.Contains(t, out, "invalid value for autoconnect")
}

func TestKnownNetworkCmd_LastConnected(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithKnownNetwork(), nil, false, "known-network", "KnownNet", "last-connected")
	require.Equal(t, 0, code, out)
	require.Equal(t, "2024-01-02T03:04:05Z", strings.TrimSpace(out))
}

func TestKnownNetworkCmd_Forget(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithKnownNetwork(), nil, false, "known-network", "KnownNet", "forget")
	require.Equal(t, 0, code, out)
	require.Contains(t, out, "forgotten")
}

func TestKnownNetworkCmd_Forget_Error(t *testing.T) {
	t.Parallel()

	known := &fakeKnownNetwork{
		path:      "/net/connman/iwd/known_networks/1",
		props:     &spiderw.KnownNetworkProperties{Name: "KnownNet", Type: spiderw.NetworkTypePSK},
		forgetErr: errors.New("forget boom"),
	}
	fc := &fakeClient{
		daemon:        &fakeDaemon{knownNetworks: []spiderw.KnownNetworkRef{{Path: known.path, Name: "KnownNet"}}},
		knownNetworks: map[string]knownNetworkAPI{known.path: known},
	}
	out, code := driveCLI(fc, nil, false, "known-network", "KnownNet", "forget")
	require.Equal(t, 1, code)
	require.Contains(t, out, "forget boom")
}

func TestKnownNetworkCmd_EnumerationError(t *testing.T) {
	t.Parallel()

	fc := &fakeClient{allKnownErr: errors.New("enumeration boom")}
	out, code := driveCLI(fc, nil, false, "known-network", "status")
	require.Equal(t, 1, code)
	require.Contains(t, out, "enumeration boom")
}

func TestKnownNetworkCmd_UnknownSubcommand(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithKnownNetwork(), nil, false, "known-network", "KnownNet", "powered")
	require.Equal(t, 1, code)
	require.Contains(t, out, "unknown known-network command")
}

// TestPrintKnownNetworkAutoConnectLine covers the monitor output helper directly
// (the monitor command blocks on an OS signal and is not driveable in-process).
func TestPrintKnownNetworkAutoConnectLine(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex

	app, buf := appWithBuffer(false)
	require.NoError(t, printKnownNetworkAutoConnectLine(app, "HomeNet", true, &mu))
	require.Equal(t, "autoconnect=true\n", buf.String())

	appJSON, bufJSON := appWithBuffer(true)
	require.NoError(t, printKnownNetworkAutoConnectLine(appJSON, "HomeNet", false, &mu))
	require.Contains(t, bufJSON.String(), `"AutoConnect":false`)
}
