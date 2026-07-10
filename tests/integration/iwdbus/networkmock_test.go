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

const openNetworkPath = "/net/connman/iwd/0/3/4f70656e4e6574_open"

// newRawMockNetwork builds a raw iwdbus.Network bound to a mock network path, for
// signal-plumbing tests.
func newRawMockNetwork(t *testing.T, path string) *iwdbus.Network {
	t.Helper()

	conn, err := dbus.SessionBus()
	require.NoError(t, err)

	n, err := iwdbus.NewNetwork(context.Background(), conn, dbus.ObjectPath(path))
	require.NoError(t, err)

	t.Cleanup(func() { _ = conn.Close() })
	return n
}

func runSpiderNetwork(t *testing.T, args ...string) (string, error) {
	t.Helper()
	return runSpider(t, append([]string{"network"}, args...)...)
}

// mockNetworks is the set of networks the mock exports under the device, in the
// daemon's path-sorted enumeration order.
var mockNetworks = []spiderw.NetworkRef{
	{Path: "/net/connman/iwd/0/3/4b6e6f776e4e6574_psk", Name: "KnownNet"},
	{Path: "/net/connman/iwd/0/3/4f70656e4e6574_open", Name: "OpenNet"},
	{Path: "/net/connman/iwd/0/3/536563757265644e6574_psk", Name: "SecuredNet"},
}

func newPublicMockNetwork(t *testing.T, ctx context.Context, client *spiderw.Client, name string) *spiderw.Network {
	t.Helper()

	refs, err := client.Daemon().Networks(ctx)
	require.NoError(t, err)
	for _, ref := range refs {
		if ref.Name != name {
			continue
		}
		network, err := client.Network(ctx, ref.Path)
		require.NoError(t, err)
		require.NotNil(t, network)
		return network
	}
	t.Fatalf("mock network %q not found in refs: %#v", name, refs)
	return nil
}

// -----------------------------------------------------------------------------
// Public client against the mock
// -----------------------------------------------------------------------------

func TestNetworkMock_DaemonNetworks(t *testing.T) {
	iwdmock.StartMockNormal(t)

	ctx := context.Background()
	client := newMockClient(t, ctx)

	refs, err := client.Daemon().Networks(ctx)
	require.NoError(t, err)
	require.Equal(t, mockNetworks, refs)
}

func TestNetworkMock_Properties(t *testing.T) {
	iwdmock.StartMockNormal(t)

	ctx := context.Background()
	client := newMockClient(t, ctx)

	open := newPublicMockNetwork(t, ctx, client, "OpenNet")
	props, err := open.Properties(ctx)
	require.NoError(t, err)
	require.Equal(t, "OpenNet", props.Name)
	require.Equal(t, spiderw.NetworkTypeOpen, props.Type)
	require.Equal(t, "/net/connman/iwd/0/3", props.Device.Path)
	require.Nil(t, props.KnownNetwork)
	require.Len(t, props.ExtendedServiceSet, 2)

	known := newPublicMockNetwork(t, ctx, client, "KnownNet")
	kp, err := known.Properties(ctx)
	require.NoError(t, err)
	require.Equal(t, spiderw.NetworkTypePSK, kp.Type)
	require.NotNil(t, kp.KnownNetwork)
}

// TestNetworkMock_ExtendedServiceSet verifies the BSS membership story: a
// network's ExtendedServiceSet paths resolve to live BasicServiceSet handles.
func TestNetworkMock_ExtendedServiceSet(t *testing.T) {
	iwdmock.StartMockNormal(t)

	ctx := context.Background()
	client := newMockClient(t, ctx)

	open := newPublicMockNetwork(t, ctx, client, "OpenNet")
	ess, err := open.ExtendedServiceSet(ctx)
	require.NoError(t, err)
	require.Len(t, ess, 2)

	addresses := make([]string, 0, len(ess))
	for _, path := range ess {
		bss, err := client.BasicServiceSet(ctx, path)
		require.NoError(t, err)
		addr, err := bss.Address(ctx)
		require.NoError(t, err)
		addresses = append(addresses, addr)
	}
	require.ElementsMatch(t, []string{"11:22:33:44:55:66", "77:88:99:aa:bb:cc"}, addresses)
}

func TestNetworkMock_Connect_Open(t *testing.T) {
	iwdmock.StartMockNormal(t)

	ctx := context.Background()
	client := newMockClient(t, ctx)

	open := newPublicMockNetwork(t, ctx, client, "OpenNet")
	require.NoError(t, open.Connect(ctx))

	connected, err := open.Connected(ctx)
	require.NoError(t, err)
	require.True(t, connected)
}

func TestNetworkMock_Connect_KnownPSK(t *testing.T) {
	iwdmock.StartMockNormal(t)

	ctx := context.Background()
	client := newMockClient(t, ctx)

	// A known (provisioned) secured network connects without an agent.
	known := newPublicMockNetwork(t, ctx, client, "KnownNet")
	require.NoError(t, known.Connect(ctx))
}

func TestNetworkMock_Connect_SecuredNoAgent(t *testing.T) {
	iwdmock.StartMockNormal(t)

	ctx := context.Background()
	client := newMockClient(t, ctx)

	// An unknown secured network is rejected because no agent is registered.
	secured := newPublicMockNetwork(t, ctx, client, "SecuredNet")
	err := secured.Connect(ctx)
	require.Error(t, err)
	require.ErrorIs(t, err, spiderw.ErrNoAgent)
}

// TestNetworkMock_SubscribeConnectedChanged exercises raw-iwdbus signal plumbing:
// a Connect on the open network emits a Connected PropertiesChanged that the
// subscription must deliver.
func TestNetworkMock_SubscribeConnectedChanged(t *testing.T) {
	iwdmock.StartMockNormal(t)

	ctx := context.Background()
	n := newRawMockNetwork(t, openNetworkPath)

	received := make(chan bool, 2)
	unsubscribe, err := n.SubscribeConnectedChanged(ctx, func(connected bool) {
		received <- connected
	})
	require.NoError(t, err)

	require.NoError(t, n.Connect(ctx))
	select {
	case got := <-received:
		require.True(t, got)
	case <-time.After(signalTimeout):
		t.Fatal("timed out waiting for connected=true callback")
	}

	require.NoError(t, unsubscribe.Unsubscribe())
}

func TestNetworkMock_AllNetworks_Empty(t *testing.T) {
	iwdmock.StartMockWithoutNetwork(t)

	ctx := context.Background()
	client := newMockClient(t, ctx)

	refs, err := client.Daemon().Networks(ctx)
	require.NoError(t, err)
	require.Empty(t, refs)

	networks, err := client.AllNetworks(ctx)
	require.NoError(t, err)
	require.Empty(t, networks)
}

// -----------------------------------------------------------------------------
// CLI (`spiderw network …`) against the mock
// -----------------------------------------------------------------------------

// TestNetworkMock_StatusJSON is the representative end-to-end CLI smoke for the
// network: it drives `network status --json` through the full real-D-Bus stack
// (Client.AllNetworks + per-network Properties) and asserts the structured
// output for every exported network.
func TestNetworkMock_StatusJSON(t *testing.T) {
	iwdmock.StartMockNormal(t)

	list, out, err := runSpiderJSONArray(t, "network", "status")
	require.NoError(t, err, "output:\n%s", out)
	require.Len(t, list, len(mockNetworks), "output:\n%s", out)

	byName := make(map[string]map[string]any, len(list))
	for _, entry := range list {
		byName[jsonGetString(t, entry, "Name")] = entry
	}

	open := byName["OpenNet"]
	require.NotNil(t, open, "OpenNet missing:\n%s", out)
	require.Equal(t, "open", jsonGetString(t, open, "Type"))
}
