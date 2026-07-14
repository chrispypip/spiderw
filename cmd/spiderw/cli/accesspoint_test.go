//go:build unit

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/chrispypip/spiderw"
)

const (
	testAPPath = "/net/connman/iwd/phy1/wlan1"
	testAPName = "wlan1"
)

func fakeWithAccessPoint() (*fakeClient, *fakeAccessPoint) {
	ssid := "HostedNet"
	freq := uint32(5180)
	group := "CCMP"
	ap := &fakeAccessPoint{
		path: testAPPath,
		name: testAPName,
		props: &spiderw.AccessPointProperties{
			Started:         true,
			Scanning:        false,
			SSID:            &ssid,
			Frequency:       &freq,
			PairwiseCiphers: []string{"CCMP"},
			GroupCipher:     &group,
		},
		ordered: []spiderw.AccessPointOrderedNetwork{
			{Name: "OpenNet", SignalStrength: -60, Type: spiderw.NetworkTypeOpen},
			{Name: "SecuredNet", SignalStrength: -72.5, Type: spiderw.NetworkTypePSK},
		},
	}
	return accessPointClient(ap), ap
}

func accessPointClient(ap *fakeAccessPoint) *fakeClient {
	return &fakeClient{
		daemon:          &fakeDaemon{accessPoints: []spiderw.AccessPointRef{{Path: ap.path, Name: ap.name}}},
		accessPoints:    map[string]accessPointAPI{ap.path: ap},
		allAccessPoints: []accessPointAPI{ap},
	}
}

func TestAccessPointCmd_List_Human(t *testing.T) {
	t.Parallel()

	fc, _ := fakeWithAccessPoint()
	out, code := driveCLI(fc, nil, false, "access-point", "list")
	require.Equal(t, 0, code, out)
	require.Contains(t, out, testAPPath)
	require.Contains(t, out, testAPName)
}

func TestAccessPointCmd_List_Empty(t *testing.T) {
	t.Parallel()

	fc := &fakeClient{daemon: &fakeDaemon{}}
	out, code := driveCLI(fc, nil, false, "access-point", "list")
	require.Equal(t, 0, code, out)
	require.Contains(t, out, "no access points")
}

func TestAccessPointCmd_Status_Human(t *testing.T) {
	t.Parallel()

	fc, _ := fakeWithAccessPoint()
	out, code := driveCLI(fc, nil, false, "access-point", "status")
	require.Equal(t, 0, code, out)
	for _, want := range []string{testAPName, testAPPath, "HostedNet", "5180", "CCMP"} {
		require.Contains(t, out, want)
	}
}

func TestAccessPointCmd_Status_JSON(t *testing.T) {
	t.Parallel()

	fc, _ := fakeWithAccessPoint()
	out, code := driveCLI(fc, nil, true, "access-point", "status")
	require.Equal(t, 0, code, out)

	var list []map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &list))
	require.Len(t, list, 1)
	require.Equal(t, testAPName, list[0]["Name"])
	require.Equal(t, "HostedNet", list[0]["SSID"])
	require.Equal(t, true, list[0]["Started"])
}

func TestAccessPointCmd_Status_Stopped_OmitsScanning(t *testing.T) {
	t.Parallel()

	ap := &fakeAccessPoint{
		path:  testAPPath,
		name:  testAPName,
		props: &spiderw.AccessPointProperties{Started: false},
	}
	out, code := driveCLI(accessPointClient(ap), nil, false, "access-point", "status")
	require.Equal(t, 0, code, out)
	require.Contains(t, out, "Started:  false")
	require.NotContains(t, out, "Scanning")
}

func TestAccessPointCmd_SingleStatus_ByName(t *testing.T) {
	t.Parallel()

	fc, _ := fakeWithAccessPoint()
	out, code := driveCLI(fc, nil, false, "access-point", testAPName, "status")
	require.Equal(t, 0, code, out)
	require.Contains(t, out, "HostedNet")
}

func TestAccessPointCmd_Start(t *testing.T) {
	t.Parallel()

	fc, ap := fakeWithAccessPoint()
	out, code := driveCLI(fc, nil, false, "access-point", testAPName, "start", "MyAP", "s3cretpass")
	require.Equal(t, 0, code, out)
	require.Contains(t, out, "access point started")
	require.Equal(t, "MyAP", ap.startedSSID)
	require.Equal(t, "s3cretpass", ap.startedPSK)
}

func TestAccessPointCmd_Start_BadArgs(t *testing.T) {
	t.Parallel()

	fc, _ := fakeWithAccessPoint()
	out, code := driveCLI(fc, nil, false, "access-point", testAPName, "start", "MyAP")
	require.Equal(t, 1, code)
	require.Contains(t, out, "usage:")
}

func TestAccessPointCmd_StartProfile(t *testing.T) {
	t.Parallel()

	fc, ap := fakeWithAccessPoint()
	out, code := driveCLI(fc, nil, false, "access-point", testAPName, "start-profile", "MyProfile")
	require.Equal(t, 0, code, out)
	require.Equal(t, "MyProfile", ap.profileSSID)
}

func TestAccessPointCmd_Stop(t *testing.T) {
	t.Parallel()

	fc, ap := fakeWithAccessPoint()
	out, code := driveCLI(fc, nil, false, "access-point", testAPName, "stop")
	require.Equal(t, 0, code, out)
	require.Contains(t, out, "access point stopped")
	require.True(t, ap.stopCalled)
}

func TestAccessPointCmd_Scan_WaitsThenListsNetworks(t *testing.T) {
	t.Parallel()

	fc, ap := fakeWithAccessPoint()
	out, code := driveCLI(fc, nil, false, "access-point", testAPName, "scan")
	require.Equal(t, 0, code, out)
	require.True(t, ap.scanCalled)
	// Wait mode announces that the scan started, then lists the results once
	// Scanning returns to false.
	require.Contains(t, out, "scan started")
	require.Contains(t, out, "OpenNet")
	require.Contains(t, out, "SecuredNet")
}

func TestAccessPointCmd_Scan_NoWait(t *testing.T) {
	t.Parallel()

	fc, ap := fakeWithAccessPoint()
	out, code := driveCLI(fc, nil, false, "access-point", testAPName, "scan", "--no-wait")
	require.Equal(t, 0, code, out)
	require.Contains(t, out, "scan started")
	require.True(t, ap.scanCalled)
}

func TestAccessPointCmd_Scan_Timeout(t *testing.T) {
	t.Parallel()

	ap := &fakeAccessPoint{path: testAPPath, name: testAPName, scanNeverCompletes: true}
	out, code := driveCLI(accessPointClient(ap), nil, false,
		"access-point", testAPName, "scan", "--timeout=50ms")
	require.Equal(t, 1, code)
	require.Contains(t, out, "timed out waiting for scan")
}

func TestAccessPointCmd_Scan_RejectsNonPositiveTimeout(t *testing.T) {
	t.Parallel()

	fc, _ := fakeWithAccessPoint()
	out, code := driveCLI(fc, nil, false,
		"access-point", testAPName, "scan", "--timeout=0")
	require.Equal(t, 1, code)
	require.Contains(t, out, "--timeout must be positive")
}

func TestAccessPointCmd_Scan_RejectsTimeoutWithNoWait(t *testing.T) {
	t.Parallel()

	fc, _ := fakeWithAccessPoint()
	out, code := driveCLI(fc, nil, false,
		"access-point", testAPName, "scan", "--no-wait", "--timeout=30s")
	require.Equal(t, 1, code)
	require.Contains(t, out, "--timeout has no effect with --no-wait")
}

func TestAccessPointCmd_Networks(t *testing.T) {
	t.Parallel()

	fc, _ := fakeWithAccessPoint()
	out, code := driveCLI(fc, nil, false, "access-point", testAPName, "networks")
	require.Equal(t, 0, code, out)
	for _, want := range []string{"OpenNet", "SecuredNet", "-60", "-72.5"} {
		require.Contains(t, out, want)
	}
}

func TestAccessPointCmd_Networks_JSON(t *testing.T) {
	t.Parallel()

	fc, _ := fakeWithAccessPoint()
	out, code := driveCLI(fc, nil, true, "access-point", testAPName, "networks")
	require.Equal(t, 0, code, out)
	var list []map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &list))
	require.Len(t, list, 2)
	require.Equal(t, "OpenNet", list[0]["Name"])
	require.InDelta(t, -60.0, list[0]["SignalDBm"], 0.001)
}

func TestAccessPointCmd_UnknownName(t *testing.T) {
	t.Parallel()

	fc, _ := fakeWithAccessPoint()
	out, code := driveCLI(fc, nil, false, "access-point", "nope", "status")
	require.Equal(t, 1, code)
	require.Contains(t, out, `access point "nope" not found`)
}

func TestAccessPointCmd_StartError(t *testing.T) {
	t.Parallel()

	ap := &fakeAccessPoint{path: testAPPath, name: testAPName, startErr: errors.New("already exists")}
	out, code := driveCLI(accessPointClient(ap), nil, false,
		"access-point", testAPName, "start", "MyAP", "s3cretpass")
	require.Equal(t, 1, code)
	require.Contains(t, out, "already exists")
}

func TestAccessPointCmd_UnknownSubcommand(t *testing.T) {
	t.Parallel()

	fc, _ := fakeWithAccessPoint()
	out, code := driveCLI(fc, nil, false, "access-point", testAPName, "bogus")
	require.Equal(t, 1, code)
	require.Contains(t, out, "unknown access-point command")
}

func TestAccessPointCmd_StartProfile_BadArgs(t *testing.T) {
	t.Parallel()

	fc, _ := fakeWithAccessPoint()
	out, code := driveCLI(fc, nil, false, "access-point", testAPName, "start-profile")
	require.Equal(t, 1, code)
	require.Contains(t, out, "usage:")
}

func TestAccessPointCmd_Stop_BadArgs(t *testing.T) {
	t.Parallel()

	fc, _ := fakeWithAccessPoint()
	out, code := driveCLI(fc, nil, false, "access-point", testAPName, "stop", "extra")
	require.Equal(t, 1, code)
	require.Contains(t, out, "usage:")
}

func TestAccessPointCmd_Networks_BadArgs(t *testing.T) {
	t.Parallel()

	fc, _ := fakeWithAccessPoint()
	out, code := driveCLI(fc, nil, false, "access-point", testAPName, "networks", "extra")
	require.Equal(t, 1, code)
	require.Contains(t, out, "usage:")
}

func TestAccessPointCmd_MissingCommand(t *testing.T) {
	t.Parallel()

	fc, _ := fakeWithAccessPoint()
	out, code := driveCLI(fc, nil, false, "access-point")
	require.Equal(t, 1, code)
	require.Contains(t, out, "missing access-point command")
	require.Contains(t, out, "Commands:")
}

func TestAccessPointCmd_RefMissingCommand(t *testing.T) {
	t.Parallel()

	fc, _ := fakeWithAccessPoint()
	out, code := driveCLI(fc, nil, false, "access-point", testAPName)
	require.Equal(t, 1, code)
	require.Contains(t, out, "missing access-point command for")
}

func TestAccessPointCmd_AmbiguousRef(t *testing.T) {
	t.Parallel()

	// Two access points share the name "wlan1", so a name reference is ambiguous.
	fc := &fakeClient{
		daemon: &fakeDaemon{accessPoints: []spiderw.AccessPointRef{
			{Path: "/net/connman/iwd/phy1/wlan1", Name: "wlan1"},
			{Path: "/net/connman/iwd/phy2/wlan1", Name: "wlan1"},
		}},
	}
	out, code := driveCLI(fc, nil, false, "access-point", "wlan1", "status")
	require.Equal(t, 1, code)
	require.Contains(t, out, "ambiguous")
}

func TestAccessPointCmd_Status_PropertiesError(t *testing.T) {
	t.Parallel()

	ap := &fakeAccessPoint{path: testAPPath, name: testAPName, err: errors.New("read failed")}
	out, code := driveCLI(accessPointClient(ap), nil, false, "access-point", "status")
	require.Equal(t, 1, code)
	require.Contains(t, out, "read failed")
}

func TestAccessPointCmd_Networks_Error(t *testing.T) {
	t.Parallel()

	ap := &fakeAccessPoint{path: testAPPath, name: testAPName, err: errors.New("scan read failed")}
	out, code := driveCLI(accessPointClient(ap), nil, false, "access-point", testAPName, "networks")
	require.Equal(t, 1, code)
	require.Contains(t, out, "scan read failed")
}

func TestAccessPointCmd_EmptyRef(t *testing.T) {
	t.Parallel()

	fc, _ := fakeWithAccessPoint()
	out, code := driveCLI(fc, nil, false, "access-point", "", "status")
	require.Equal(t, 1, code)
	require.Contains(t, out, "reference required")
}

func TestAccessPointCmd_StopError(t *testing.T) {
	t.Parallel()

	ap := &fakeAccessPoint{path: testAPPath, name: testAPName, stopErr: errors.New("stop failed")}
	out, code := driveCLI(accessPointClient(ap), nil, false, "access-point", testAPName, "stop")
	require.Equal(t, 1, code)
	require.Contains(t, out, "stop failed")
}

func TestAccessPointCmd_StartProfileError(t *testing.T) {
	t.Parallel()

	// StartProfile shares the fake's startErr; the `start` error path is covered
	// separately, so this pins the start-profile branch.
	ap := &fakeAccessPoint{path: testAPPath, name: testAPName, startErr: errors.New("no such profile")}
	out, code := driveCLI(accessPointClient(ap), nil, false,
		"access-point", testAPName, "start-profile", "MyProfile")
	require.Equal(t, 1, code)
	require.Contains(t, out, "no such profile")
}

func TestAccessPointCmd_ScanError(t *testing.T) {
	t.Parallel()

	// A scan on a stopped AP is rejected by iwd with NotAvailable, so this is a
	// path users actually reach. Both wait mode and --no-wait must surface it.
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"wait mode", []string{"access-point", testAPName, "scan"}},
		{"no-wait", []string{"access-point", testAPName, "scan", "--no-wait"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ap := &fakeAccessPoint{path: testAPPath, name: testAPName, scanErr: errors.New("operation not available")}
			out, code := driveCLI(accessPointClient(ap), nil, false, tc.args...)
			require.Equal(t, 1, code)
			require.Contains(t, out, "operation not available")
		})
	}
}

func TestAccessPointCmd_Scan_SubscribeError(t *testing.T) {
	t.Parallel()

	// Wait mode subscribes to Scanning before triggering the scan, so a failing
	// subscription must surface rather than the scan running unwatched.
	ap := &fakeAccessPoint{path: testAPPath, name: testAPName, err: errors.New("subscribe failed")}
	out, code := driveCLI(accessPointClient(ap), nil, false, "access-point", testAPName, "scan")
	require.Equal(t, 1, code)
	require.Contains(t, out, "subscribe failed")
	require.False(t, ap.scanCalled, "the scan must not start when the subscription fails")
}

func TestAccessPointCmd_Scan_BadFlag(t *testing.T) {
	t.Parallel()

	fc, _ := fakeWithAccessPoint()
	out, code := driveCLI(fc, nil, false, "access-point", testAPName, "scan", "--bogus")
	require.Equal(t, 1, code)
	require.Contains(t, out, "bogus")
}

func TestAccessPointCmd_Scan_RejectsPositionalArg(t *testing.T) {
	t.Parallel()

	fc, _ := fakeWithAccessPoint()
	out, code := driveCLI(fc, nil, false, "access-point", testAPName, "scan", "extra")
	require.Equal(t, 1, code)
	require.Contains(t, out, "usage:")
}

func TestAccessPointCmd_RejectsUnknownArguments(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		args []string
	}{
		{"list", []string{"access-point", "list", "extra"}},
		{"status", []string{"access-point", "status", "extra"}},
		{"single status", []string{"access-point", testAPName, "status", "extra"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fc, _ := fakeWithAccessPoint()
			out, code := driveCLI(fc, nil, false, tc.args...)
			require.Equal(t, 1, code)
			require.NotEmpty(t, out)
		})
	}
}

func TestAccessPointCmd_RefWithNoAccessPoints(t *testing.T) {
	t.Parallel()

	// Referencing an access point when the host has none is distinct from naming
	// one that does not exist ("not found").
	fc := &fakeClient{daemon: &fakeDaemon{}}
	out, code := driveCLI(fc, nil, false, "access-point", testAPName, "status")
	require.Equal(t, 1, code)
	require.Contains(t, out, "no access points available")
}

func TestAccessPointCmd_Status_Empty(t *testing.T) {
	t.Parallel()

	fc := &fakeClient{daemon: &fakeDaemon{}}
	out, code := driveCLI(fc, nil, false, "access-point", "status")
	require.Equal(t, 0, code, out)
	require.Contains(t, out, "no access points")
}

func TestAccessPointCmd_Networks_Empty(t *testing.T) {
	t.Parallel()

	// An AP that has not scanned yet reports no results rather than an empty list.
	ap := &fakeAccessPoint{path: testAPPath, name: testAPName}
	out, code := driveCLI(accessPointClient(ap), nil, false, "access-point", testAPName, "networks")
	require.Equal(t, 0, code, out)
	require.Contains(t, out, "no networks available")
}

// TestPrintAccessPointMonitorLines covers the access-point monitor output helpers
// directly (the monitor command blocks on an OS signal).
func TestPrintAccessPointMonitorLines(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex

	app, buf := appWithBuffer(false)
	require.NoError(t, printAccessPointStartedLine(app, testAPName, true, &mu))
	require.Equal(t, "started=true\n", buf.String())

	appScan, bufScan := appWithBuffer(false)
	require.NoError(t, printAccessPointScanningLine(appScan, testAPName, false, &mu))
	require.Equal(t, "scanning=false\n", bufScan.String())

	appJSON, bufJSON := appWithBuffer(true)
	require.NoError(t, printAccessPointStartedLine(appJSON, testAPName, true, &mu))
	require.Contains(t, bufJSON.String(), `"Started":true`)

	appScanJSON, bufScanJSON := appWithBuffer(true)
	require.NoError(t, printAccessPointScanningLine(appScanJSON, testAPName, true, &mu))
	require.Contains(t, bufScanJSON.String(), `"Scanning":true`)
}

func TestAccessPointCmd_Monitor_BadArgs(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{
		{"access-point", testAPName, "monitor"},
		{"access-point", testAPName, "monitor", "bogus"},
		{"access-point", testAPName, "monitor", "started", "extra"},
	} {
		fc, _ := fakeWithAccessPoint()
		out, code := driveCLI(fc, nil, false, args...)
		require.Equal(t, 1, code, out)
		require.Contains(t, out, "usage:")
	}
}

// TestStreamAccessPointProperty drives the monitor's non-blocking core: the
// current value prints, and each target wires its own subscription.
func TestStreamAccessPointProperty(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	for _, tc := range []struct {
		what        string
		wantSeed    string
		wantSubcall string
	}{
		{"started", "started=true", "SubscribeStartedChanged"},
		{"scanning", "scanning=false", "SubscribeScanningChanged"},
	} {
		t.Run(tc.what, func(t *testing.T) {
			t.Parallel()
			_, ap := fakeWithAccessPoint()
			started := false
			ap.startedEvent = &started

			app, buf := appWithBuffer(false)
			var mu sync.Mutex

			unsubscribe, err := streamAccessPointProperty(ctx, app, testAPName, tc.what, ap, &mu)
			require.NoError(t, err)
			require.NoError(t, unsubscribe.Unsubscribe())

			require.Contains(t, buf.String(), tc.wantSeed, "the current value must print first")
			require.Equal(t, tc.wantSubcall, ap.subscribed,
				"target %q must subscribe to its own property", tc.what)
		})
	}
}

func TestStreamAccessPointProperty_Errors(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	var mu sync.Mutex
	app, _ := appWithBuffer(false)

	ap := &fakeAccessPoint{path: testAPPath, name: testAPName, err: errors.New("read failed")}
	_, err := streamAccessPointProperty(ctx, app, testAPName, "started", ap, &mu)
	require.Error(t, err)
	require.Contains(t, err.Error(), "read failed")

	_, apOK := fakeWithAccessPoint()
	apOK.subscribeErr = errors.New("subscribe failed")
	_, err = streamAccessPointProperty(ctx, app, testAPName, "started", apOK, &mu)
	require.Error(t, err)
	require.Contains(t, err.Error(), "subscribe failed")

	_, apBad := fakeWithAccessPoint()
	_, err = streamAccessPointProperty(ctx, app, testAPName, "bogus", apBad, &mu)
	require.Error(t, err)
	require.Contains(t, err.Error(), "usage:")
}

func TestParseAccessPointMonitorTarget(t *testing.T) {
	t.Parallel()

	for _, what := range accessPointMonitorTargets {
		got, err := parseAccessPointMonitorTarget([]string{what})
		require.NoError(t, err)
		require.Equal(t, what, got)
	}
	for _, args := range [][]string{nil, {"bogus"}, {"started", "extra"}} {
		_, err := parseAccessPointMonitorTarget(args)
		require.Error(t, err)
	}
}

// TestAccessPointCmd_Status_EnumerationError covers the enumeration itself failing,
// as the sibling resources already do. This is distinct from a per-AP Properties()
// error: here the daemon never hands back a list at all.
func TestAccessPointCmd_Status_EnumerationError(t *testing.T) {
	t.Parallel()

	fc := &fakeClient{allAccessPointErr: errors.New("enumeration boom")}
	out, code := driveCLI(fc, nil, false, "access-point", "status")
	require.Equal(t, 1, code)
	require.Contains(t, out, "enumeration boom")
}
