//go:build integration

package integration_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/chrispypip/spiderw"
	"github.com/chrispypip/spiderw/tests/testutil/iwdmock"
)

const (
	// These mirror the literals the iwd mock seeds for the station-mode device
	// (see tools/test-mocks/iwdmock/internal/mock/station.go).
	stationConnectedNetworkPath     = "/net/connman/iwd/phy0/wlan0/known_psk"
	stationConnectedAccessPointPath = "/net/connman/iwd/phy0/wlan0/aabbccddeeff"
)

// TestStationMock_Reads verifies every read-only Station property resolves over
// real D-Bus (introspection + Properties.GetAll), including the optional
// experimental ones (ConnectedAccessPoint, Affinities).
func TestStationMock_Reads(t *testing.T) {
	tmpDir := t.TempDir()
	iwdmock.StartMockNormal(t, tmpDir)
	ctx := context.Background()
	client := newMockClient(t, ctx)

	station, err := client.Station(ctx, devicePath)
	require.NoError(t, err)

	state, err := station.State(ctx)
	require.NoError(t, err)
	require.Equal(t, spiderw.StationStateConnected, state)

	scanning, err := station.Scanning(ctx)
	require.NoError(t, err)
	require.False(t, scanning)

	connectedNet, err := station.ConnectedNetwork(ctx)
	require.NoError(t, err)
	require.NotNil(t, connectedNet)
	require.Equal(t, stationConnectedNetworkPath, *connectedNet)

	ap, err := station.ConnectedAccessPoint(ctx)
	require.NoError(t, err)
	require.NotNil(t, ap)
	require.Equal(t, stationConnectedAccessPointPath, *ap)

	affinities, err := station.Affinities(ctx)
	require.NoError(t, err)
	require.Equal(t, []string{stationConnectedAccessPointPath}, affinities)

	// Properties reads all of the above in a single GetAll.
	props, err := station.Properties(ctx)
	require.NoError(t, err)
	require.Equal(t, spiderw.StationStateConnected, props.State)
	require.False(t, props.Scanning)
	require.NotNil(t, props.ConnectedNetwork)
	require.Equal(t, stationConnectedNetworkPath, *props.ConnectedNetwork)
	require.NotNil(t, props.ConnectedAccessPoint)
	require.Equal(t, stationConnectedAccessPointPath, *props.ConnectedAccessPoint)
	require.Equal(t, []string{stationConnectedAccessPointPath}, props.Affinities)
}

// TestStationMock_AllStations verifies station enumeration via the real
// ObjectManager returns exactly the station-mode device (wlan0), not the
// AP-mode device (wlan1).
func TestStationMock_AllStations(t *testing.T) {
	tmpDir := t.TempDir()
	iwdmock.StartMockNormal(t, tmpDir)
	ctx := context.Background()
	client := newMockClient(t, ctx)

	stations, err := client.AllStations(ctx)
	require.NoError(t, err)
	require.Len(t, stations, 1)
	require.Equal(t, devicePath, stations[0].Path())

	// The enumerated handle is live.
	state, err := stations[0].State(ctx)
	require.NoError(t, err)
	require.Equal(t, spiderw.StationStateConnected, state)
}

// TestStationMock_SubscribeRegisters verifies the property-change subscriptions
// wire up over real D-Bus. No S1 operation mutates station state (Scan/Connect/
// Disconnect are later sub-slices), so this asserts clean registration and
// teardown; the fires-and-maps behavior is covered by the unit suites.
func TestStationMock_SubscribeRegisters(t *testing.T) {
	tmpDir := t.TempDir()
	iwdmock.StartMockNormal(t, tmpDir)
	ctx := context.Background()
	client := newMockClient(t, ctx)

	station, err := client.Station(ctx, devicePath)
	require.NoError(t, err)

	unsubState, err := station.SubscribeStateChanged(ctx, func(spiderw.StationState) {})
	require.NoError(t, err)
	require.NotNil(t, unsubState)

	unsubScanning, err := station.SubscribeScanningChanged(ctx, func(bool) {})
	require.NoError(t, err)
	require.NotNil(t, unsubScanning)

	require.NoError(t, unsubState.Unsubscribe())
	require.NoError(t, unsubScanning.Unsubscribe())
}

// TestStationMock_Unavailable verifies that when the Station interface is not
// exported (--omit-station), Client.Station fails cleanly and station
// enumeration is empty, even though the device object still exists.
func TestStationMock_Unavailable(t *testing.T) {
	tmpDir := t.TempDir()
	iwdmock.StartMockWithoutStation(t, tmpDir)
	ctx := context.Background()
	client := newMockClient(t, ctx)

	station, err := client.Station(ctx, devicePath)
	require.Error(t, err)
	require.Nil(t, station)

	stations, err := client.AllStations(ctx)
	require.NoError(t, err)
	require.Empty(t, stations)

	// The device itself is still present, confirming only the Station interface
	// was omitted.
	device, err := client.Device(ctx, devicePath)
	require.NoError(t, err)
	name, err := device.Name(ctx)
	require.NoError(t, err)
	require.Equal(t, "wlan0", name)
}

// TestStationMock_CLIStatus drives the `station status` CLI command in-process
// against the mock, confirming the read path renders end to end.
func TestStationMock_CLIStatus(t *testing.T) {
	tmpDir := t.TempDir()
	iwdmock.StartMockNormal(t, tmpDir)

	out, err := runSpider(t, "station", "status")
	require.NoError(t, err, out)
	require.Contains(t, out, "connected")
	require.Contains(t, out, devicePath)
	require.Contains(t, out, stationConnectedNetworkPath)
	require.Contains(t, out, stationConnectedAccessPointPath)
}

// TestStationMock_CLIList drives `station list` against the mock.
func TestStationMock_CLIList(t *testing.T) {
	tmpDir := t.TempDir()
	iwdmock.StartMockNormal(t, tmpDir)

	out, err := runSpider(t, "station", "list")
	require.NoError(t, err, out)
	require.Contains(t, out, devicePath)
}
