//go:build unit

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/chrispypip/spiderw"
)

const (
	testStationPath = "/net/connman/iwd/phy0/wlan0"
	testStationName = "wlan0"
	testStationNet  = "/net/connman/iwd/phy0/wlan0/known_psk"
	testStationAP   = "/net/connman/iwd/phy0/wlan0/aabbccddeeff"
)

func fakeWithStation() *fakeClient {
	return stationClient(&fakeStation{
		path: testStationPath,
		name: testStationName,
		props: &spiderw.StationProperties{
			State:                spiderw.StationStateConnected,
			Scanning:             false,
			ConnectedNetwork:     new(testStationNet),
			ConnectedAccessPoint: new(testStationAP),
			Affinities:           []string{testStationAP},
		},
		ordered: []spiderw.OrderedNetwork{
			{Network: testStationNet, SignalStrength: -60},
			{Network: "/net/connman/iwd/phy0/wlan0/open", SignalStrength: -72.5},
		},
	})
}

func stationClient(st *fakeStation) *fakeClient {
	return &fakeClient{
		daemon:      &fakeDaemon{stations: []spiderw.StationRef{{Path: st.path, Name: st.name}}},
		stations:    map[string]stationAPI{st.path: st},
		allStations: []stationAPI{st},
	}
}

func TestStationCmd_List_Human(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithStation(), nil, false, "station", "list")
	require.Equal(t, 0, code, out)
	require.Contains(t, out, testStationPath)
	require.Contains(t, out, testStationName)
}

func TestStationCmd_Status_ShowsName(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithStation(), nil, true, "station", "status")
	require.Equal(t, 0, code, out)
	var list []map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &list))
	require.Len(t, list, 1)
	require.Equal(t, testStationName, list[0]["Name"])
}

func TestStationCmd_ResolvesByName(t *testing.T) {
	t.Parallel()

	st := &fakeStation{path: testStationPath, name: testStationName}
	out, code := driveCLI(stationClient(st), nil, false,
		"station", testStationName, "disconnect")
	require.Equal(t, 0, code, out)
	require.True(t, st.disconnectCalled)
}

func TestStationCmd_UnknownName(t *testing.T) {
	t.Parallel()

	st := &fakeStation{path: testStationPath, name: testStationName}
	out, code := driveCLI(stationClient(st), nil, false,
		"station", "nope", "disconnect")
	require.Equal(t, 1, code)
	require.Contains(t, out, `station "nope" not found`)
}

func TestStationCmd_List_Empty(t *testing.T) {
	t.Parallel()

	fc := &fakeClient{daemon: &fakeDaemon{}}
	out, code := driveCLI(fc, nil, false, "station", "list")
	require.Equal(t, 0, code, out)
	require.Contains(t, out, "no stations available")
}

func TestStationCmd_Status_Human(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithStation(), nil, false, "station", "status")
	require.Equal(t, 0, code, out)
	for _, want := range []string{"connected", "Scanning", testStationPath, testStationNet, testStationAP} {
		require.Contains(t, out, want)
	}
}

func TestStationCmd_Status_JSON(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithStation(), nil, true, "station", "status")
	require.Equal(t, 0, code, out)

	var list []map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &list))
	require.Len(t, list, 1)
	entry := list[0]
	require.Equal(t, testStationPath, entry["Path"])
	require.Equal(t, "connected", entry["State"])
	require.Equal(t, false, entry["Scanning"])
	require.Equal(t, testStationNet, entry["ConnectedNetwork"])
	require.Equal(t, testStationAP, entry["ConnectedAccessPoint"])
	require.Equal(t, []any{testStationAP}, entry["Affinities"])
}

func TestStationCmd_ScopedStatus_JSON(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithStation(), nil, true, "station", testStationPath, "status")
	require.Equal(t, 0, code, out)

	var list []map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &list))
	require.Len(t, list, 1)
	require.Equal(t, testStationPath, list[0]["Path"])
	require.Equal(t, "connected", list[0]["State"])
}

func TestStationCmd_Disconnected_ShowsDashes(t *testing.T) {
	t.Parallel()

	st := &fakeStation{
		path: testStationPath,
		props: &spiderw.StationProperties{
			State:    spiderw.StationStateDisconnected,
			Scanning: false,
			// ConnectedNetwork / ConnectedAccessPoint nil, Affinities nil.
		},
	}
	fc := &fakeClient{allStations: []stationAPI{st}}

	out, code := driveCLI(fc, nil, false, "station", "status")
	require.Equal(t, 0, code, out)
	require.Contains(t, out, "disconnected")
	require.Contains(t, out, "ConnectedNetwork:")
	require.Contains(t, out, "-")
}

func TestStationCmd_ScopedStatus_NotFound(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithStation(), nil, false, "station", "/nope", "status")
	require.Equal(t, 1, code)
	require.Contains(t, out, "not found")
}

func TestStationCmd_ScopedStatus_UsageError(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithStation(), nil, false, "station", testStationPath, "status", "extra")
	require.Equal(t, 1, code)
	require.Contains(t, out, "usage: spiderw station <station> status")
	require.NotContains(t, out, "Commands:")
}

func TestStationCmd_Status_BackendError(t *testing.T) {
	t.Parallel()

	fc := &fakeClient{allStationErr: errors.New("enumeration boom")}
	out, code := driveCLI(fc, nil, false, "station", "status")
	require.Equal(t, 1, code)
	require.Contains(t, out, "enumeration boom")
}

func TestStationCmd_UnknownVerb(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithStation(), nil, false, "station", testStationPath, "bogus")
	require.Equal(t, 1, code)
	require.Contains(t, out, "unknown station command")
	require.Contains(t, out, "Commands:")
}

func TestStationCmd_MissingCommand(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithStation(), nil, false, "station")
	require.Equal(t, 1, code)
	require.Contains(t, out, "missing station command")
	require.Contains(t, out, "Commands:")
}

func TestStationCmd_Scan_WaitsThenListsNetworks(t *testing.T) {
	t.Parallel()

	st := &fakeStation{
		path:    testStationPath,
		ordered: []spiderw.OrderedNetwork{{Network: testStationNet, SignalStrength: -60}},
	}
	out, code := driveCLI(stationClient(st), nil, false, "station", testStationPath, "scan")
	require.Equal(t, 0, code, out)
	require.True(t, st.scanCalled)
	// The fake's SubscribeScanningChanged fires true then false, so wait mode
	// completes and prints the ordered networks.
	require.Contains(t, out, testStationNet)
	require.Contains(t, out, "-60 dBm")
}

func TestStationCmd_Scan_NoWait(t *testing.T) {
	t.Parallel()

	st := &fakeStation{path: testStationPath}
	out, code := driveCLI(stationClient(st), nil, false, "station", testStationPath, "scan", "--no-wait")
	require.Equal(t, 0, code, out)
	require.True(t, st.scanCalled)
	require.Contains(t, out, "scan started")
}

func TestStationCmd_Scan_Error(t *testing.T) {
	t.Parallel()

	st := &fakeStation{path: testStationPath, scanErr: errors.New("scan boom")}
	out, code := driveCLI(stationClient(st), nil, false, "station", testStationPath, "scan", "--no-wait")
	require.Equal(t, 1, code)
	require.Contains(t, out, "scan boom")
}

func TestStationCmd_Scan_Timeout(t *testing.T) {
	t.Parallel()

	// The scan starts but never reports completion, so wait mode hits --timeout.
	st := &fakeStation{path: testStationPath, scanNeverCompletes: true}
	out, code := driveCLI(stationClient(st), nil, false,
		"station", testStationPath, "scan", "--timeout=50ms")
	require.Equal(t, 1, code)
	require.Contains(t, out, "timed out waiting for scan to finish")
}

func TestStationCmd_Scan_RejectsNonPositiveTimeout(t *testing.T) {
	t.Parallel()

	st := &fakeStation{path: testStationPath}
	out, code := driveCLI(stationClient(st), nil, false,
		"station", testStationPath, "scan", "--timeout=0")
	require.Equal(t, 1, code)
	require.Contains(t, out, "--timeout must be positive")
}

func TestStationCmd_Scan_RejectsTimeoutWithNoWait(t *testing.T) {
	t.Parallel()

	st := &fakeStation{path: testStationPath}
	out, code := driveCLI(stationClient(st), nil, false,
		"station", testStationPath, "scan", "--no-wait", "--timeout=30s")
	require.Equal(t, 1, code)
	require.Contains(t, out, "--timeout has no effect with --no-wait")
	require.False(t, st.scanCalled, "must reject before triggering the scan")
}

func TestStationCmd_Scan_HonorsContextCancellation(t *testing.T) {
	t.Parallel()

	// A cancelled context aborts the wait rather than blocking until --timeout.
	st := &fakeStation{path: testStationPath, scanNeverCompletes: true}
	var buf bytes.Buffer
	app := &App{
		Stdout: &buf,
		Stderr: &buf,
		NewClient: func(ctx context.Context, bus spiderw.Bus) (clientAPI, error) {
			return stationClient(st), nil
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := runStationScan(app, ctx, testStationPath, nil)
	require.ErrorIs(t, err, context.Canceled)
}

func TestStationCmd_Networks_JSON(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithStation(), nil, true, "station", testStationPath, "networks")
	require.Equal(t, 0, code, out)

	var list []map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &list))
	require.Len(t, list, 2)
	require.Equal(t, testStationNet, list[0]["Network"])
	require.Equal(t, -60.0, list[0]["SignalDBm"])
	require.Equal(t, -72.5, list[1]["SignalDBm"])
}

func TestStationCmd_Affinities_Get(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithStation(), nil, false, "station", testStationPath, "affinities")
	require.Equal(t, 0, code, out)
	require.Contains(t, out, testStationAP)
}

func TestStationCmd_Affinities_Set(t *testing.T) {
	t.Parallel()

	st := &fakeStation{path: testStationPath}
	out, code := driveCLI(stationClient(st), nil, false,
		"station", testStationPath, "affinities", "set",
		"/net/connman/iwd/phy0/wlan0/aaa", "/net/connman/iwd/phy0/wlan0/bbb")
	require.Equal(t, 0, code, out)
	require.Equal(t, []string{
		"/net/connman/iwd/phy0/wlan0/aaa",
		"/net/connman/iwd/phy0/wlan0/bbb",
	}, st.setAffinitiesTo)
}

func TestStationCmd_Affinities_Set_Missing(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithStation(), nil, false, "station", testStationPath, "affinities", "set")
	require.Equal(t, 1, code)
	require.Contains(t, out, "usage: spiderw station <station> affinities set")
}

func TestStationCmd_Disconnect(t *testing.T) {
	t.Parallel()

	st := &fakeStation{path: testStationPath}
	out, code := driveCLI(stationClient(st), nil, false, "station", testStationPath, "disconnect")
	require.Equal(t, 0, code, out)
	require.True(t, st.disconnectCalled)
	require.Contains(t, out, "disconnected")
}

func TestStationCmd_Disconnect_Error(t *testing.T) {
	t.Parallel()

	st := &fakeStation{path: testStationPath, disconnectErr: errors.New("not connected")}
	out, code := driveCLI(stationClient(st), nil, false, "station", testStationPath, "disconnect")
	require.Equal(t, 1, code)
	require.Contains(t, out, "not connected")
}

func TestStationCmd_ConnectHidden(t *testing.T) {
	t.Parallel()

	st := &fakeStation{path: testStationPath}
	out, code := driveCLI(stationClient(st), nil, false,
		"station", testStationPath, "connect-hidden", "MyHidden", "--passphrase=secret")
	require.Equal(t, 0, code, out)
	require.Equal(t, "MyHidden", st.connectHiddenName)
	require.Contains(t, out, "connected to MyHidden")
}

func TestStationCmd_ConnectHidden_MissingSSID(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithStation(), nil, false, "station", testStationPath, "connect-hidden")
	require.Equal(t, 1, code)
	require.Contains(t, out, "usage: spiderw station <station> connect-hidden")
}

func TestStationCmd_ConnectHidden_Error(t *testing.T) {
	t.Parallel()

	st := &fakeStation{path: testStationPath, connectHiddenErr: errors.New("no agent")}
	out, code := driveCLI(stationClient(st), nil, false,
		"station", testStationPath, "connect-hidden", "MyHidden", "--passphrase=x")
	require.Equal(t, 1, code)
	require.Contains(t, out, "no agent")
}

func TestStationCmd_HiddenAPs_JSON(t *testing.T) {
	t.Parallel()

	st := &fakeStation{
		path: testStationPath,
		hiddenAPs: []spiderw.HiddenAccessPoint{
			{Address: "aa:bb:cc:dd:ee:ff", SignalStrength: -60, Type: spiderw.NetworkTypePSK},
			{Address: "11:22:33:44:55:66", SignalStrength: -72.5, Type: spiderw.NetworkTypeOpen},
		},
	}
	out, code := driveCLI(stationClient(st), nil, true, "station", testStationPath, "hidden-aps")
	require.Equal(t, 0, code, out)

	var list []map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &list))
	require.Len(t, list, 2)
	require.Equal(t, "aa:bb:cc:dd:ee:ff", list[0]["Address"])
	require.Equal(t, -60.0, list[0]["SignalDBm"])
	require.Equal(t, "psk", list[0]["Type"])
	require.Equal(t, "open", list[1]["Type"])
}

func TestStationCmd_HiddenAPs_Empty(t *testing.T) {
	t.Parallel()

	st := &fakeStation{path: testStationPath}
	out, code := driveCLI(stationClient(st), nil, false, "station", testStationPath, "hidden-aps")
	require.Equal(t, 0, code, out)
	require.Contains(t, out, "no hidden access points available")
}
