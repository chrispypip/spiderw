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

// TestPrintStationMonitorLines covers the station monitor output helpers directly
// (the monitor command itself blocks on an OS signal and is not drivable
// in-process). The optional-path lines are the interesting ones: iwd's null path
// arrives as nil, and "disconnected" must read as a word, not an empty value.
func TestPrintStationMonitorLines(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	path := "/net/connman/iwd/0/3/ssid_psk"

	t.Run("state", func(t *testing.T) {
		t.Parallel()
		app, buf := appWithBuffer(false)
		require.NoError(t, printStationStateLine(app, "wlan0", spiderw.StationStateConnected, &mu))
		require.Equal(t, "state=connected\n", buf.String())

		appJSON, bufJSON := appWithBuffer(true)
		require.NoError(t, printStationStateLine(appJSON, "wlan0", spiderw.StationStateRoaming, &mu))
		require.Contains(t, bufJSON.String(), `"State":"roaming"`)
	})

	t.Run("scanning", func(t *testing.T) {
		t.Parallel()
		app, buf := appWithBuffer(false)
		require.NoError(t, printStationScanningLine(app, "wlan0", true, &mu))
		require.Equal(t, "scanning=true\n", buf.String())

		appJSON, bufJSON := appWithBuffer(true)
		require.NoError(t, printStationScanningLine(appJSON, "wlan0", true, &mu))
		require.Contains(t, bufJSON.String(), `"Scanning":true`)
	})

	t.Run("network", func(t *testing.T) {
		t.Parallel()
		app, buf := appWithBuffer(false)
		require.NoError(t, printStationConnectedNetworkLine(app, "wlan0", &nameRef{Name: "MySSID", Path: path}, &mu))
		require.Equal(t, "network=MySSID\n", buf.String(), "the SSID is shown, not the path")

		appOff, bufOff := appWithBuffer(false)
		require.NoError(t, printStationConnectedNetworkLine(appOff, "wlan0", nil, &mu))
		require.Equal(t, "network=none (disconnected)\n", bufOff.String())

		appJSON, bufJSON := appWithBuffer(true)
		require.NoError(t, printStationConnectedNetworkLine(appJSON, "wlan0", nil, &mu))
		require.Contains(t, bufJSON.String(), `"ConnectedNetwork":null`)
	})

	t.Run("access-point", func(t *testing.T) {
		t.Parallel()
		app, buf := appWithBuffer(false)
		require.NoError(t, printStationConnectedAPLine(app, "wlan0", &addrRef{Address: testStationMAC, Path: path}, &mu))
		require.Equal(t, "access-point="+testStationMAC+"\n", buf.String(), "the MAC is shown, not the path")

		appOff, bufOff := appWithBuffer(false)
		require.NoError(t, printStationConnectedAPLine(appOff, "wlan0", nil, &mu))
		require.Equal(t, "access-point=none (not associated)\n", bufOff.String())

		appJSON, bufJSON := appWithBuffer(true)
		require.NoError(t, printStationConnectedAPLine(appJSON, "wlan0", &addrRef{Address: testStationMAC, Path: path}, &mu))
		require.Contains(t, bufJSON.String(), `"Address":"`+testStationMAC+`"`)
		require.Contains(t, bufJSON.String(), `"Path":"`+path+`"`, "JSON keeps the path alongside the MAC")
	})

	t.Run("affinities", func(t *testing.T) {
		t.Parallel()
		app, buf := appWithBuffer(false)
		require.NoError(t, printStationAffinitiesLine(app, "wlan0", []addrRef{{Address: "aa:bb", Path: "/a"}, {Address: "cc:dd", Path: "/b"}}, &mu))
		require.Equal(t, "affinities=aa:bb, cc:dd\n", buf.String(), "MACs, not paths")

		appOff, bufOff := appWithBuffer(false)
		require.NoError(t, printStationAffinitiesLine(appOff, "wlan0", nil, &mu))
		require.Equal(t, "affinities=none\n", bufOff.String(), "a cleared list must read as none, not blank")

		appJSON, bufJSON := appWithBuffer(true)
		require.NoError(t, printStationAffinitiesLine(appJSON, "wlan0", []addrRef{{Address: "aa:bb", Path: "/a"}}, &mu))
		require.Contains(t, bufJSON.String(), `"Address":"aa:bb"`)
	})
}

func TestStationCmd_Monitor_BadArgs(t *testing.T) {
	t.Parallel()

	// Argument validation runs before the command blocks on a signal, so these are
	// drivable in-process.
	for _, args := range [][]string{
		{"station", testStationPath, "monitor"},
		{"station", testStationPath, "monitor", "bogus"},
		{"station", testStationPath, "monitor", "state", "extra"},
	} {
		out, code := driveCLI(fakeWithStation(), nil, false, args...)
		require.Equal(t, 1, code, out)
		require.Contains(t, out, "usage:")
	}
}

// TestStreamStationProperty drives the monitor's non-blocking core directly: it
// prints the current value and wires the matching subscription. The `subscribed`
// assertion is the point — it proves each target reaches its own Subscribe method,
// so swapping two branches of the switch (network <-> access-point, an easy
// copy-paste slip) fails here instead of shipping.
func TestStreamStationProperty(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	netPath := "/net/connman/iwd/phy0/wlan0/evented_psk"
	apPath := "/net/connman/iwd/phy0/wlan0/112233445566"

	// A fully-populated station whose subscribe events carry values distinct from
	// its Properties(), so the streamed line cannot be confused with the seed line.
	newFake := func() *fakeStation {
		st := &fakeStation{
			path:  testStationPath,
			name:  testStationName,
			props: fakeWithStation().allStations[0].(*fakeStation).props,
		}
		state := spiderw.StationStateRoaming
		st.stateEvent = &state
		st.connNetEvent = &cliOptStringEvent{v: &netPath}
		st.connAPEvent = &cliOptStringEvent{v: &apPath}
		st.affinityEvent = &cliStringSliceEvent{v: []string{apPath}}
		return st
	}

	// The seed line renders the ref Properties() already resolved (SSID / MAC). The
	// streamed line goes through the resolver, which here has no client, so it falls
	// back to the raw path — proving the fallback never prints blank.
	for _, tc := range []struct {
		what        string
		wantSeed    string
		wantEvent   string
		wantSubcall string
	}{
		{"state", "state=connected", "state=roaming", "SubscribeStateChanged"},
		{"scanning", "scanning=false", "scanning=", "SubscribeScanningChanged"},
		{"network", "network=KnownNet", "network=" + netPath, "SubscribeConnectedNetworkChanged"},
		{"access-point", "access-point=" + testStationMAC, "access-point=" + apPath, "SubscribeConnectedAccessPointChanged"},
		{"affinities", "affinities=" + testStationMAC, "affinities=" + apPath, "SubscribeAffinitiesChanged"},
	} {
		t.Run(tc.what, func(t *testing.T) {
			t.Parallel()
			st := newFake()
			app, buf := appWithBuffer(false)
			var mu sync.Mutex

			unsubscribe, err := streamStationProperty(ctx, app, testStationName, tc.what, st, monitorResolver{}, &mu)
			require.NoError(t, err)
			require.NotNil(t, unsubscribe)
			require.NoError(t, unsubscribe.Unsubscribe())

			out := buf.String()
			require.Contains(t, out, tc.wantSeed, "the current value must print first")
			require.Contains(t, out, tc.wantEvent, "a subsequent change must print")
			require.Equal(t, tc.wantSubcall, st.subscribed,
				"target %q must subscribe to its own property", tc.what)
		})
	}
}

func TestStreamStationProperty_Errors(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	var mu sync.Mutex

	t.Run("properties error", func(t *testing.T) {
		t.Parallel()
		st := &fakeStation{path: testStationPath, err: errors.New("read failed")}
		app, _ := appWithBuffer(false)
		_, err := streamStationProperty(ctx, app, testStationName, "state", st, monitorResolver{}, &mu)
		require.Error(t, err)
		require.Contains(t, err.Error(), "read failed")
	})

	t.Run("subscribe error", func(t *testing.T) {
		t.Parallel()
		// Properties succeeds, the subscription does not.
		st := &fakeStation{path: testStationPath, subscribeErr: errors.New("subscribe failed")}
		st.props = fakeWithStation().allStations[0].(*fakeStation).props
		app, _ := appWithBuffer(false)
		_, err := streamStationProperty(ctx, app, testStationName, "state", st, monitorResolver{}, &mu)
		require.Error(t, err)
		require.Contains(t, err.Error(), "subscribe failed")
	})

	t.Run("unknown target", func(t *testing.T) {
		t.Parallel()
		st := &fakeStation{path: testStationPath}
		st.props = fakeWithStation().allStations[0].(*fakeStation).props
		app, _ := appWithBuffer(false)
		_, err := streamStationProperty(ctx, app, testStationName, "bogus", st, monitorResolver{}, &mu)
		require.Error(t, err)
		require.Contains(t, err.Error(), "usage:")
	})
}

func TestParseStationMonitorTarget(t *testing.T) {
	t.Parallel()

	for _, what := range stationMonitorTargets {
		got, err := parseStationMonitorTarget([]string{what})
		require.NoError(t, err)
		require.Equal(t, what, got)
	}
	for _, args := range [][]string{nil, {}, {"bogus"}, {"state", "extra"}} {
		_, err := parseStationMonitorTarget(args)
		require.Error(t, err)
	}
}

// TestStreamStationProperty_ResolvesNames proves the streamed value — which the
// subscription delivers as a bare object path — is rendered as the SSID (and the
// BSS as a MAC), matching what `station status` shows. Without the resolver the
// user would see /net/connman/iwd/0/3/<hex-ssid>_psk instead of the SSID.
func TestStreamStationProperty_ResolvesNames(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	fc := fakeWithStation()
	st := fc.allStations[0].(*fakeStation)
	netPath := "/net/connman/iwd/phy0/wlan0/open"
	st.connNetEvent = &cliOptStringEvent{v: &netPath}

	// The client can resolve that path to a network with an SSID.
	fc.networks = map[string]networkAPI{
		netPath: &fakeNetwork{path: netPath, props: &spiderw.NetworkProperties{Name: "OpenNet"}},
	}

	app, buf := appWithBuffer(false)
	var mu sync.Mutex

	unsubscribe, err := streamStationProperty(ctx, app, testStationName, "network", st, monitorResolver{client: fc}, &mu)
	require.NoError(t, err)
	require.NoError(t, unsubscribe.Unsubscribe())

	require.Contains(t, buf.String(), "network=OpenNet", "the streamed path must render as its SSID")
	require.NotContains(t, buf.String(), netPath, "the raw path must not leak into human output")
}

// TestStreamStationProperty_DisconnectClearsLine covers the hardware bug end to
// end at the CLI: iwd invalidates ConnectedNetwork on a disconnect rather than
// sending a value, and the monitor must print the disconnected line rather than
// staying silent.
func TestStreamStationProperty_DisconnectClearsLine(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	fc := fakeWithStation()
	st := fc.allStations[0].(*fakeStation)
	st.connNetEvent = &cliOptStringEvent{v: nil} // the invalidation, delivered as nil

	app, buf := appWithBuffer(false)
	var mu sync.Mutex

	unsubscribe, err := streamStationProperty(ctx, app, testStationName, "network", st, monitorResolver{client: fc}, &mu)
	require.NoError(t, err)
	require.NoError(t, unsubscribe.Unsubscribe())

	require.Contains(t, buf.String(), "network=none (disconnected)")
}

// TestStationCmd_Affinities_SetError covers the failure path for `affinities set`.
// iwd rejects SetAffinities on hardware that does not support it (a Raspberry Pi's
// brcmfmac does), so this is a path users hit routinely — and it had no test.
func TestStationCmd_Affinities_SetError(t *testing.T) {
	t.Parallel()

	st := &fakeStation{
		path:      testStationPath,
		name:      testStationName,
		props:     &spiderw.StationProperties{State: spiderw.StationStateConnected},
		setAffErr: errors.New("not supported"),
	}
	// Pass the BSS object path, so the command reaches SetAffinities without
	// needing a MAC-to-path resolution first.
	out, code := driveCLI(stationClient(st), nil, false,
		"station", testStationName, "affinities", "set", testStationAP)
	require.Equal(t, 1, code, out)
	require.Contains(t, out, "not supported")
}

// TestStationCmd_Affinities_ClearError covers the same failure on `affinities clear`,
// which is a separate SetAffinities call.
func TestStationCmd_Affinities_ClearError(t *testing.T) {
	t.Parallel()

	st := &fakeStation{
		path:      testStationPath,
		name:      testStationName,
		props:     &spiderw.StationProperties{State: spiderw.StationStateConnected},
		setAffErr: errors.New("not supported"),
	}
	out, code := driveCLI(stationClient(st), nil, false,
		"station", testStationName, "affinities", "clear")
	require.Equal(t, 1, code, out)
	require.Contains(t, out, "not supported")
}

// TestStationCmd_MonitorSignal_BadArgs covers `monitor-signal`'s argument guard,
// which runs before the command blocks on an OS signal. The command had no test
// invocation at all.
func TestStationCmd_MonitorSignal_BadArgs(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{
		{"station", testStationName, "monitor-signal"},               // no thresholds
		{"station", testStationName, "monitor-signal", "abc"},        // not a number
		{"station", testStationName, "monitor-signal", "-70", "-60"}, // not descending
	} {
		out, code := driveCLI(fakeWithStation(), nil, false, args...)
		require.Equal(t, 1, code, out)
		require.NotEmpty(t, out)
	}
}

// TestParseSignalThresholds_RejectsNonDescending pins the ordering contract.
// signalBandRange maps a band index back to the dBm range it covers, which is only
// meaningful when the thresholds descend. The help documented "highest first" but
// nothing enforced it, so an ascending list was accepted and then rendered as
// nonsense ranges.
func TestParseSignalThresholds_RejectsNonDescending(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{
		{"-70", "-60"},        // ascending
		{"-60", "-60"},        // equal is not strictly descending
		{"-60", "-80", "-70"}, // out of order in the middle
	} {
		_, err := parseSignalThresholds(args)
		require.Error(t, err, args)
		require.Contains(t, err.Error(), "descending")
	}

	// A single threshold, and a properly descending list, remain valid.
	_, err := parseSignalThresholds([]string{"-60"})
	require.NoError(t, err)
	_, err = parseSignalThresholds([]string{"-60", "-70", "-80"})
	require.NoError(t, err)
}
