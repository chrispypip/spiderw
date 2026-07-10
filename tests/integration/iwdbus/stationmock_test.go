//go:build integration

package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/chrispypip/spiderw"
	"github.com/chrispypip/spiderw/tests/testutil/iwdmock"
)

const (
	// These mirror the literals the iwd mock seeds for the station-mode device
	// (see tools/test-mocks/iwdmock/internal/mock/station.go).
	stationConnectedNetworkPath     = "/net/connman/iwd/0/3/4b6e6f776e4e6574_psk"
	stationConnectedAccessPointPath = "/net/connman/iwd/0/3/4b6e6f776e4e6574_psk/deadbeefcafe"
)

// TestStationMock_Reads verifies every read-only Station property resolves over
// real D-Bus (introspection + Properties.GetAll), including the optional
// experimental ones (ConnectedAccessPoint, Affinities).
func TestStationMock_Reads(t *testing.T) {
	iwdmock.StartMockNormal(t)
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
	require.Equal(t, stationConnectedNetworkPath, props.ConnectedNetwork.Path)
	require.NotNil(t, props.ConnectedAccessPoint)
	require.Equal(t, stationConnectedAccessPointPath, props.ConnectedAccessPoint.Path)
	require.Len(t, props.Affinities, 1)
	require.Equal(t, stationConnectedAccessPointPath, props.Affinities[0].Path)

	// The bundle resolves each path to its friendly identifier.
	require.Equal(t, "KnownNet", props.ConnectedNetwork.Name)
	require.Equal(t, "de:ad:be:ef:ca:fe", props.ConnectedAccessPoint.Address)
	require.Equal(t, "de:ad:be:ef:ca:fe", props.Affinities[0].Address)
}

// TestStationMock_AllStations verifies station enumeration via the real
// ObjectManager returns exactly the station-mode device (wlan0), not the
// AP-mode device (wlan1).
func TestStationMock_AllStations(t *testing.T) {
	iwdmock.StartMockNormal(t)
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
	iwdmock.StartMockNormal(t)
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
	iwdmock.StartMockWithoutStation(t)
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
func TestStationMock_CLI_Status(t *testing.T) {
	iwdmock.StartMockNormal(t)

	out, err := runSpider(t, "station", "status")
	require.NoError(t, err, out)
	require.Contains(t, out, "connected")
	require.Contains(t, out, devicePath)
	require.Contains(t, out, "KnownNet")          // resolved SSID
	require.Contains(t, out, "de:ad:be:ef:ca:fe") // resolved BSS MAC
}

// TestStationMock_CLIList drives `station list` against the mock.
func TestStationMock_CLI_List(t *testing.T) {
	iwdmock.StartMockNormal(t)

	out, err := runSpider(t, "station", "list")
	require.NoError(t, err, out)
	require.Contains(t, out, devicePath)
}

// TestStationMock_ScanLiveTransition is the headline S2 test: it drives a real
// scan and observes the Scanning property transition true->false over live
// PropertiesChanged signals -- the first end-to-end exercise of a subscription.
func TestStationMock_ScanLiveTransition(t *testing.T) {
	iwdmock.StartMockNormal(t)
	ctx := context.Background()
	client := newMockClient(t, ctx)

	station, err := client.Station(ctx, devicePath)
	require.NoError(t, err)

	events := make(chan bool, 8)
	unsubscribe, err := station.SubscribeScanningChanged(ctx, func(scanning bool) {
		events <- scanning
	})
	require.NoError(t, err)
	defer func() { _ = unsubscribe.Unsubscribe() }()

	require.NoError(t, station.Scan(ctx))

	waitFor := func(want bool) {
		select {
		case got := <-events:
			require.Equal(t, want, got)
		case <-time.After(3 * time.Second):
			t.Fatalf("timed out waiting for Scanning=%v", want)
		}
	}
	waitFor(true)  // scan started
	waitFor(false) // scan finished
}

// TestStationMock_OrderedNetworks reads the seeded scan results and confirms the
// signal strength is converted from iwd's 100*dBm to dBm.
func TestStationMock_OrderedNetworks(t *testing.T) {
	iwdmock.StartMockNormal(t)
	ctx := context.Background()
	client := newMockClient(t, ctx)

	station, err := client.Station(ctx, devicePath)
	require.NoError(t, err)

	nets, err := station.OrderedNetworks(ctx)
	require.NoError(t, err)
	require.Len(t, nets, 3)
	require.Equal(t, stationConnectedNetworkPath, nets[0].Path)
	require.Equal(t, "KnownNet", nets[0].Name)      // resolved SSID
	require.Equal(t, -60.0, nets[0].SignalStrength) // mock seeds -6000 (100*dBm)
}

// TestStationMock_CLIAffinitiesByMAC drives `affinities set <mac>`: the MAC
// resolves device-wide to its BSS object path, and `affinities` then renders it.
func TestStationMock_CLI_AffinitiesByMAC(t *testing.T) {
	iwdmock.StartMockNormal(t)

	out, err := runSpider(t, "station", devicePath, "affinities", "set", "77:88:99:aa:bb:cc")
	require.NoError(t, err, out)
	require.Contains(t, out, "77:88:99:aa:bb:cc")

	out, err = runSpider(t, "station", devicePath, "affinities")
	require.NoError(t, err, out)
	require.Contains(t, out, "77:88:99:aa:bb:cc")

	out, err = runSpider(t, "station", devicePath, "affinities", "clear")
	require.NoError(t, err, out)
	require.Contains(t, out, "no affinities set")
}

// TestStationMock_SetAffinities round-trips: write affinities, then read them
// back through the Affinities getter.
func TestStationMock_SetAffinities(t *testing.T) {
	iwdmock.StartMockNormal(t)
	ctx := context.Background()
	client := newMockClient(t, ctx)

	station, err := client.Station(ctx, devicePath)
	require.NoError(t, err)

	// Set multiple affinities and read the full list back.
	want := []string{
		"/net/connman/iwd/0/3/4f70656e4e6574_open/112233445566",
		"/net/connman/iwd/0/3/4f70656e4e6574_open/778899aabbcc",
	}
	require.NoError(t, station.SetAffinities(ctx, want))

	got, err := station.Affinities(ctx)
	require.NoError(t, err)
	require.Equal(t, want, got)

	// An empty slice clears all affinities.
	require.NoError(t, station.SetAffinities(ctx, nil))
	got, err = station.Affinities(ctx)
	require.NoError(t, err)
	require.Empty(t, got)
}

// TestStationMock_CLIScan drives `station <path> scan` (wait mode) against the
// mock: it waits for the scan to finish, then lists the networks.
func TestStationMock_CLI_Scan(t *testing.T) {
	iwdmock.StartMockNormal(t)

	out, err := runSpider(t, "station", devicePath, "scan")
	require.NoError(t, err, out)
	require.Contains(t, out, "KnownNet") // resolved SSID
	require.Contains(t, out, "dBm")
}

// TestStationMock_CLINetworks drives `station <path> networks` against the mock.
func TestStationMock_CLI_Networks(t *testing.T) {
	iwdmock.StartMockNormal(t)

	out, err := runSpider(t, "station", devicePath, "networks")
	require.NoError(t, err, out)
	require.Contains(t, out, "KnownNet") // resolved SSID
	require.Contains(t, out, "-60 dBm")
}

// TestStationMock_ResolvesName confirms a station carries the co-located device
// Name ("wlan0"), both from enumeration and a single lookup, and that the CLI
// renders it.
func TestStationMock_ResolvesName(t *testing.T) {
	iwdmock.StartMockNormal(t)
	ctx := context.Background()
	client := newMockClient(t, ctx)

	stations, err := client.AllStations(ctx)
	require.NoError(t, err)
	require.Len(t, stations, 1)
	require.Equal(t, "wlan0", stations[0].Name())

	st, err := client.Station(ctx, devicePath)
	require.NoError(t, err)
	require.Equal(t, "wlan0", st.Name())

	out, err := runSpider(t, "station", "list")
	require.NoError(t, err, out)
	require.Contains(t, out, "wlan0")
}

// TestStationMock_DisconnectLiveTransition drives a real disconnect and observes
// the State property transition connected->disconnected over live signals.
func TestStationMock_DisconnectLiveTransition(t *testing.T) {
	iwdmock.StartMockNormal(t)
	ctx := context.Background()
	client := newMockClient(t, ctx)

	station, err := client.Station(ctx, devicePath)
	require.NoError(t, err)

	states := make(chan spiderw.StationState, 8)
	unsubscribe, err := station.SubscribeStateChanged(ctx, func(s spiderw.StationState) {
		states <- s
	})
	require.NoError(t, err)
	defer func() { _ = unsubscribe.Unsubscribe() }()

	require.NoError(t, station.Disconnect(ctx))

	select {
	case got := <-states:
		require.Equal(t, spiderw.StationStateDisconnected, got)
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for State=disconnected")
	}
}

// TestStationMock_ConnectHidden covers the open, not-found, and secured (with and
// without an agent) hidden-connect paths.
func TestStationMock_ConnectHidden(t *testing.T) {
	t.Run("Open", func(t *testing.T) {
		iwdmock.StartMockNormal(t)
		ctx := context.Background()
		station, err := newMockClient(t, ctx).Station(ctx, devicePath)
		require.NoError(t, err)
		require.NoError(t, station.ConnectHiddenNetwork(ctx, "HiddenOpen"))
	})

	t.Run("NotFound", func(t *testing.T) {
		iwdmock.StartMockNormal(t)
		ctx := context.Background()
		station, err := newMockClient(t, ctx).Station(ctx, devicePath)
		require.NoError(t, err)
		err = station.ConnectHiddenNetwork(ctx, "NoSuchNet")
		require.Error(t, err)
		require.ErrorIs(t, err, spiderw.ErrNotFound)
	})

	t.Run("SecuredNoAgent", func(t *testing.T) {
		iwdmock.StartMockNormal(t)
		ctx := context.Background()
		station, err := newMockClient(t, ctx).Station(ctx, devicePath)
		require.NoError(t, err)
		err = station.ConnectHiddenNetwork(ctx, "HiddenSecured")
		require.Error(t, err)
		require.ErrorIs(t, err, spiderw.ErrNoAgent)
	})

	t.Run("SecuredWithAgent", func(t *testing.T) {
		iwdmock.StartMockNormal(t)
		ctx := context.Background()
		client := newMockClient(t, ctx)

		agent, err := client.RegisterAgent(ctx, spiderw.AgentConfig{
			Passphrase: func(ctx context.Context, networkPath string) (string, error) {
				return mockSecuredPassphrase, nil
			},
		})
		require.NoError(t, err)
		defer func() { _ = agent.Unregister(ctx) }()

		station, err := client.Station(ctx, devicePath)
		require.NoError(t, err)
		require.NoError(t, station.ConnectHiddenNetwork(ctx, "HiddenSecured"))
	})
}

// TestStationMock_HiddenAccessPoints reads the seeded hidden-AP list, confirming
// the signal (100*dBm -> dBm) and type conversions.
func TestStationMock_HiddenAccessPoints(t *testing.T) {
	iwdmock.StartMockNormal(t)
	ctx := context.Background()
	station, err := newMockClient(t, ctx).Station(ctx, devicePath)
	require.NoError(t, err)

	aps, err := station.HiddenAccessPoints(ctx)
	require.NoError(t, err)
	require.Len(t, aps, 2)
	require.Equal(t, "de:ad:be:ef:00:01", aps[0].Address)
	require.Equal(t, -65.0, aps[0].SignalStrength)
	require.Equal(t, spiderw.NetworkTypePSK, aps[0].Type)
	require.Equal(t, spiderw.NetworkTypeOpen, aps[1].Type)
}

// TestStationMock_CLIDisconnectHiddenAPs drives the disconnect / connect-hidden /
// hidden-aps CLI verbs against the mock.
func TestStationMock_CLI_DisconnectHiddenAPs(t *testing.T) {
	t.Run("disconnect", func(t *testing.T) {
		iwdmock.StartMockNormal(t)
		out, err := runSpider(t, "station", devicePath, "disconnect")
		require.NoError(t, err, out)
		require.Contains(t, out, "disconnected")
	})

	t.Run("connect-hidden secured", func(t *testing.T) {
		iwdmock.StartMockNormal(t)
		out, err := runSpider(t, "station", devicePath, "connect-hidden", "HiddenSecured", "--passphrase="+mockSecuredPassphrase)
		require.NoError(t, err, out)
		require.Contains(t, out, "connected to HiddenSecured")
	})

	t.Run("hidden-aps", func(t *testing.T) {
		iwdmock.StartMockNormal(t)
		out, err := runSpider(t, "station", devicePath, "hidden-aps")
		require.NoError(t, err, out)
		require.Contains(t, out, "de:ad:be:ef:00:01")
		require.Contains(t, out, "psk")
	})
}
