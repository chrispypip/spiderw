//go:build unit

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/chrispypip/spiderw"
)

const (
	testStationPath = "/net/connman/iwd/phy0/wlan0"
	testStationName = "wlan0"
	testStationNet  = "/net/connman/iwd/phy0/wlan0/known_psk"
	testStationAP   = "/net/connman/iwd/phy0/wlan0/aabbccddeeff"
	testStationMAC  = "aa:bb:cc:dd:ee:ff"
)

func fakeWithStation() *fakeClient {
	return stationClient(&fakeStation{
		path: testStationPath,
		name: testStationName,
		props: &spiderw.StationProperties{
			State:                spiderw.StationStateConnected,
			Scanning:             false,
			ConnectedNetwork:     &spiderw.NetworkRef{Path: testStationNet, Name: "KnownNet"},
			ConnectedAccessPoint: &spiderw.BasicServiceSetRef{Path: testStationAP, Address: testStationMAC},
			Affinities:           []spiderw.BasicServiceSetRef{{Path: testStationAP, Address: testStationMAC}},
		},
		ordered: []spiderw.OrderedNetwork{
			{NetworkRef: spiderw.NetworkRef{Path: testStationNet, Name: "KnownNet"}, SignalStrength: -60},
			{NetworkRef: spiderw.NetworkRef{Path: "/net/connman/iwd/phy0/wlan0/open", Name: "OpenNet"}, SignalStrength: -72.5},
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
	// Human output shows the resolved SSID and BSS MAC, not raw paths.
	for _, want := range []string{"connected", "Scanning", testStationPath, "KnownNet", testStationMAC} {
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
	require.Equal(t, map[string]any{"Name": "KnownNet", "Path": testStationNet}, entry["ConnectedNetwork"])
	require.Equal(t, map[string]any{"Address": testStationMAC, "Path": testStationAP}, entry["ConnectedAccessPoint"])
	require.Equal(t, []any{map[string]any{"Address": testStationMAC, "Path": testStationAP}}, entry["Affinities"])
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
		path: testStationPath,
		ordered: []spiderw.OrderedNetwork{
			{NetworkRef: spiderw.NetworkRef{Path: testStationNet, Name: "KnownNet"}, SignalStrength: -60},
		},
	}
	out, code := driveCLI(stationClient(st), nil, false, "station", testStationPath, "scan")
	require.Equal(t, 0, code, out)
	require.True(t, st.scanCalled)
	// Wait mode announces that the scan started, then prints the ordered networks
	// by resolved SSID once the fake's SubscribeScanningChanged fires true→false.
	require.Contains(t, out, "scan started")
	require.Contains(t, out, "KnownNet")
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
	require.Equal(t, "KnownNet", list[0]["Name"])
	require.Equal(t, testStationNet, list[0]["Path"])
	require.Equal(t, -60.0, list[0]["SignalDBm"])
	require.Equal(t, -72.5, list[1]["SignalDBm"])
}

func TestStationCmd_Affinities_Get(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithStation(), nil, false, "station", testStationPath, "affinities")
	require.Equal(t, 0, code, out)
	// Affinities render as resolved BSS MACs.
	require.Contains(t, out, testStationMAC)
}

func TestStationCmd_Affinities_SetByMAC(t *testing.T) {
	t.Parallel()

	st := &fakeStation{path: testStationPath, name: testStationName}
	fc := stationClient(st)
	fc.daemon = &fakeDaemon{
		stations: []spiderw.StationRef{{Path: st.path, Name: st.name}},
		bsses: []spiderw.BasicServiceSetRef{
			{Path: "/net/connman/iwd/phy0/wlan0/aabbccddeeff", Address: "AA:BB:CC:DD:EE:FF"},
		},
	}
	// A MAC (case-insensitive) resolves device-wide to its BSS object path.
	out, code := driveCLI(fc, nil, false,
		"station", testStationPath, "affinities", "set", "aa:bb:cc:dd:ee:ff")
	require.Equal(t, 0, code, out)
	require.Equal(t, []string{"/net/connman/iwd/phy0/wlan0/aabbccddeeff"}, st.setAffinitiesTo)
}

func TestStationCmd_Affinities_Clear(t *testing.T) {
	t.Parallel()

	st := &fakeStation{path: testStationPath, name: testStationName}
	out, code := driveCLI(stationClient(st), nil, false,
		"station", testStationPath, "affinities", "clear")
	require.Equal(t, 0, code, out)
	require.True(t, st.setAffinitiesCalled)
	require.Empty(t, st.setAffinitiesTo, "clear sends an empty list")
	require.Contains(t, out, "no affinities set")
}

func TestStationCmd_Affinities_ClearRejectsArgs(t *testing.T) {
	t.Parallel()

	st := &fakeStation{path: testStationPath, name: testStationName}
	out, code := driveCLI(stationClient(st), nil, false,
		"station", testStationPath, "affinities", "clear", "extra")
	require.Equal(t, 1, code)
	require.Contains(t, out, "usage: spiderw station <station> affinities clear")
	require.False(t, st.setAffinitiesCalled)
}

func TestStationCmd_Affinities_SetUnknownMAC(t *testing.T) {
	t.Parallel()

	st := &fakeStation{path: testStationPath, name: testStationName}
	out, code := driveCLI(stationClient(st), nil, false,
		"station", testStationPath, "affinities", "set", "00:00:00:00:00:00")
	require.Equal(t, 1, code)
	require.Contains(t, out, "no basic service set found with address")
	require.Nil(t, st.setAffinitiesTo)
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

// TestPrintSignalLevelLine covers the monitor output helper directly (the
// monitor-signal command blocks on an OS signal and is not drivable in-process).
func TestPrintSignalLevelLine(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex

	thresholds := []int{-60, -70, -80}

	app, buf := appWithBuffer(false)
	require.NoError(t, printSignalLevelLine(app, "wlan0", 2, thresholds, &mu))
	require.Equal(t, "level=2 (-80 to -70 dBm)\n", buf.String())

	appJSON, bufJSON := appWithBuffer(true)
	require.NoError(t, printSignalLevelLine(appJSON, "wlan0", 0, thresholds, &mu))
	var got stationSignalLevelResult
	require.NoError(t, json.Unmarshal(bufJSON.Bytes(), &got))
	require.Equal(t, stationSignalLevelResult{Station: "wlan0", Level: 0, Range: ">= -60 dBm"}, got)
}

func TestParseSignalThresholds(t *testing.T) {
	t.Parallel()

	got, err := parseSignalThresholds([]string{"-60", " -70 ", "-80"})
	require.NoError(t, err)
	require.Equal(t, []int{-60, -70, -80}, got)

	_, err = parseSignalThresholds([]string{"-60", "notanumber"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid signal threshold")
}

type fakeWSC struct {
	pushErr   error
	genPin    string
	genErr    error
	startErr  error
	cancelErr error

	startedPin string
	calls      []string
}

func (f *fakeWSC) PushButton(ctx context.Context) error {
	f.calls = append(f.calls, "PushButton")
	return f.pushErr
}

func (f *fakeWSC) GeneratePin(ctx context.Context) (string, error) {
	f.calls = append(f.calls, "GeneratePin")
	return f.genPin, f.genErr
}

func (f *fakeWSC) StartPin(ctx context.Context, pin string) error {
	f.calls = append(f.calls, "StartPin")
	f.startedPin = pin
	return f.startErr
}

func (f *fakeWSC) Cancel(ctx context.Context) error {
	f.calls = append(f.calls, "Cancel")
	return f.cancelErr
}

func TestRunWSCOp(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("PushButtonSuccess", func(t *testing.T) {
		t.Parallel()
		app, buf := appWithBuffer(false)
		f := &fakeWSC{}
		require.NoError(t, runWSCOp(app, ctx, "wlan0", f, "push-button", nil))
		require.Equal(t, []string{"PushButton"}, f.calls)
		out := buf.String()
		// The start prompt precedes enrollment, then the connected result.
		require.Contains(t, out, "press the WPS button")
		require.Contains(t, out, "connected via WSC")
	})

	t.Run("PushButtonError", func(t *testing.T) {
		t.Parallel()
		app, _ := appWithBuffer(false)
		f := &fakeWSC{pushErr: errors.New("overlap")}
		err := runWSCOp(app, ctx, "wlan0", f, "push-button", nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "overlap")
	})

	t.Run("PushButtonRejectsArgs", func(t *testing.T) {
		t.Parallel()
		app, _ := appWithBuffer(false)
		f := &fakeWSC{}
		err := runWSCOp(app, ctx, "wlan0", f, "push-button", []string{"extra"})
		require.Error(t, err)
		require.Empty(t, f.calls)
	})

	t.Run("PinWithArg", func(t *testing.T) {
		t.Parallel()
		app, buf := appWithBuffer(false)
		f := &fakeWSC{}
		require.NoError(t, runWSCOp(app, ctx, "wlan0", f, "pin", []string{"1234-5670"}))
		// A supplied PIN skips generation.
		require.Equal(t, []string{"StartPin"}, f.calls)
		require.Equal(t, "1234-5670", f.startedPin)
		out := buf.String()
		// The supplied PIN is echoed before enrollment, mirroring the
		// generated-PIN case.
		require.Contains(t, out, "1234-5670")
		require.Contains(t, out, "connected via WSC")
	})

	t.Run("PinGenerated", func(t *testing.T) {
		t.Parallel()
		app, buf := appWithBuffer(false)
		f := &fakeWSC{genPin: "12345670"}
		require.NoError(t, runWSCOp(app, ctx, "wlan0", f, "pin", nil))
		require.Equal(t, []string{"GeneratePin", "StartPin"}, f.calls)
		require.Equal(t, "12345670", f.startedPin, "the generated PIN is the one started")
		out := buf.String()
		require.Contains(t, out, "12345670", "the generated PIN must be printed for the user")
		require.Contains(t, out, "connected via WSC")
	})

	t.Run("PinGenerateError", func(t *testing.T) {
		t.Parallel()
		app, _ := appWithBuffer(false)
		f := &fakeWSC{genErr: errors.New("no pin")}
		err := runWSCOp(app, ctx, "wlan0", f, "pin", nil)
		require.Error(t, err)
		require.Equal(t, []string{"GeneratePin"}, f.calls, "StartPin must not run when generation fails")
	})

	t.Run("PinRejectsExtraArgs", func(t *testing.T) {
		t.Parallel()
		app, _ := appWithBuffer(false)
		f := &fakeWSC{}
		err := runWSCOp(app, ctx, "wlan0", f, "pin", []string{"12345670", "extra"})
		require.Error(t, err)
		require.Empty(t, f.calls)
	})

	t.Run("PinStartError", func(t *testing.T) {
		t.Parallel()
		// The likeliest real WSC failure: enrollment itself fails (wrong PIN, walk
		// time expired, no AP responding). The PIN is still printed first, so the
		// user can see which one was tried.
		app, buf := appWithBuffer(false)
		f := &fakeWSC{startErr: errors.New("walk time expired")}
		err := runWSCOp(app, ctx, "wlan0", f, "pin", []string{"12345670"})
		require.Error(t, err)
		require.Contains(t, err.Error(), "walk time expired")
		require.Equal(t, []string{"StartPin"}, f.calls)
		require.Contains(t, buf.String(), "12345670")
		require.NotContains(t, buf.String(), "connected via WSC", "a failed enrollment must not report success")
	})

	t.Run("Cancel", func(t *testing.T) {
		t.Parallel()
		app, buf := appWithBuffer(false)
		f := &fakeWSC{}
		require.NoError(t, runWSCOp(app, ctx, "wlan0", f, "cancel", nil))
		require.Equal(t, []string{"Cancel"}, f.calls)
		require.Contains(t, buf.String(), "canceled")
	})

	t.Run("CancelError", func(t *testing.T) {
		t.Parallel()
		app, buf := appWithBuffer(false)
		f := &fakeWSC{cancelErr: errors.New("nothing to cancel")}
		err := runWSCOp(app, ctx, "wlan0", f, "cancel", nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "nothing to cancel")
		require.NotContains(t, buf.String(), "canceled", "a failed cancel must not report success")
	})

	t.Run("CancelRejectsArgs", func(t *testing.T) {
		t.Parallel()
		app, _ := appWithBuffer(false)
		f := &fakeWSC{}
		err := runWSCOp(app, ctx, "wlan0", f, "cancel", []string{"extra"})
		require.Error(t, err)
		require.Empty(t, f.calls)
	})

	t.Run("UnknownSubcommand", func(t *testing.T) {
		t.Parallel()
		app, _ := appWithBuffer(false)
		f := &fakeWSC{}
		err := runWSCOp(app, ctx, "wlan0", f, "bogus", nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unknown wsc subcommand")
		require.Empty(t, f.calls)
	})

	t.Run("JSONOutput", func(t *testing.T) {
		t.Parallel()
		app, buf := appWithBuffer(true)
		f := &fakeWSC{}
		require.NoError(t, runWSCOp(app, ctx, "wlan0", f, "push-button", nil))
		require.Contains(t, buf.String(), `"Station": "wlan0"`)
		require.Contains(t, buf.String(), `"Action": "push-button"`)
	})

	t.Run("PinJSONHasStationAndPin", func(t *testing.T) {
		t.Parallel()
		app, buf := appWithBuffer(true)
		f := &fakeWSC{}
		require.NoError(t, runWSCOp(app, ctx, "wlan0", f, "pin", []string{"12345678"}))
		out := buf.String()
		// The pre-enrollment JSON object carries the station and PIN.
		require.Contains(t, out, `"Station": "wlan0"`)
		require.Contains(t, out, `"Pin": "12345678"`)
	})
}

func TestStationCmd_WSC_UnknownSubcommand(t *testing.T) {
	t.Parallel()
	out, code := driveCLI(fakeWithStation(), nil, false, "station", testStationPath, "wsc", "bogus")
	require.NotEqual(t, 0, code)
	require.Contains(t, out, "unknown wsc subcommand")
}

func TestStationCmd_WSC_MissingSubcommand(t *testing.T) {
	t.Parallel()
	out, code := driveCLI(fakeWithStation(), nil, false, "station", testStationPath, "wsc")
	require.NotEqual(t, 0, code)
	require.Contains(t, out, "usage")
}

func TestStationCmd_WSC_HandleUnavailable(t *testing.T) {
	t.Parallel()
	st := &fakeStation{path: testStationPath, name: testStationName, wscErr: errors.New("wsc not available")}
	out, code := driveCLI(stationClient(st), nil, false, "station", testStationPath, "wsc", "push-button")
	require.NotEqual(t, 0, code)
	require.Contains(t, out, "wsc not available")
}
