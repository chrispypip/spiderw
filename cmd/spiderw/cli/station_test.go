//go:build unit

package cli

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/chrispypip/spiderw"
)

const (
	testStationPath = "/net/connman/iwd/phy0/wlan0"
	testStationNet  = "/net/connman/iwd/phy0/wlan0/known_psk"
	testStationAP   = "/net/connman/iwd/phy0/wlan0/aabbccddeeff"
)

func fakeWithStation() *fakeClient {
	st := &fakeStation{
		path: testStationPath,
		props: &spiderw.StationProperties{
			State:                spiderw.StationStateConnected,
			Scanning:             false,
			ConnectedNetwork:     new(testStationNet),
			ConnectedAccessPoint: new(testStationAP),
			Affinities:           []string{testStationAP},
		},
	}
	return &fakeClient{
		daemon:      &fakeDaemon{stations: []spiderw.StationRef{{Path: st.path}}},
		stations:    map[string]stationAPI{st.path: st},
		allStations: []stationAPI{st},
	}
}

func TestStationCmd_List_Human(t *testing.T) {
	t.Parallel()

	out, code := driveCLI(fakeWithStation(), nil, false, "station", "list")
	require.Equal(t, 0, code, out)
	require.Contains(t, out, testStationPath)
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
