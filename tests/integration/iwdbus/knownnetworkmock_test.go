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

// mockKnownNetworks is the set of known networks the mock exports, in the
// daemon's path-sorted enumeration order.
// Path-sorted: 4775... (GuestHotspot) precedes 4b6e... (KnownNet).
var mockKnownNetworks = []spiderw.KnownNetworkRef{
	{Path: "/net/connman/iwd/4775657374486f7473706f74_hotspot", Name: "GuestHotspot"},
	{Path: "/net/connman/iwd/4b6e6f776e4e6574_psk", Name: "KnownNet"},
}

const knownNetworkPath = "/net/connman/iwd/4b6e6f776e4e6574_psk"

func newRawMockKnownNetwork(t *testing.T, path string) *iwdbus.KnownNetwork {
	t.Helper()

	conn, err := dbus.SessionBus()
	require.NoError(t, err)

	k, err := iwdbus.NewKnownNetwork(context.Background(), conn, dbus.ObjectPath(path))
	require.NoError(t, err)

	t.Cleanup(func() { _ = conn.Close() })
	return k
}

func newPublicMockKnownNetwork(t *testing.T, ctx context.Context, client *spiderw.Client, name string) *spiderw.KnownNetwork {
	t.Helper()

	refs, err := client.Daemon().KnownNetworks(ctx)
	require.NoError(t, err)
	for _, ref := range refs {
		if ref.Name != name {
			continue
		}
		k, err := client.KnownNetwork(ctx, ref.Path)
		require.NoError(t, err)
		require.NotNil(t, k)
		return k
	}
	t.Fatalf("mock known network %q not found in refs: %#v", name, refs)
	return nil
}

// -----------------------------------------------------------------------------
// Public client against the mock
// -----------------------------------------------------------------------------

func TestKnownNetworkMock_DaemonKnownNetworks(t *testing.T) {
	iwdmock.StartMockNormal(t)

	ctx := context.Background()
	client := newMockClient(t, ctx)

	refs, err := client.Daemon().KnownNetworks(ctx)
	require.NoError(t, err)
	require.Equal(t, mockKnownNetworks, refs)
}

func TestKnownNetworkMock_Properties(t *testing.T) {
	iwdmock.StartMockNormal(t)

	ctx := context.Background()
	client := newMockClient(t, ctx)

	known := newPublicMockKnownNetwork(t, ctx, client, "KnownNet")
	props, err := known.Properties(ctx)
	require.NoError(t, err)
	require.Equal(t, "KnownNet", props.Name)
	require.Equal(t, spiderw.NetworkTypePSK, props.Type)
	require.False(t, props.Hidden)
	require.NotNil(t, props.LastConnectedTime)
	require.True(t, props.AutoConnect)

	hotspot := newPublicMockKnownNetwork(t, ctx, client, "GuestHotspot")
	hp, err := hotspot.Properties(ctx)
	require.NoError(t, err)
	require.Equal(t, spiderw.NetworkTypeHotspot, hp.Type)
	require.Nil(t, hp.LastConnectedTime)
	require.False(t, hp.AutoConnect)
}

// TestKnownNetworkMock_NetworkLinkage verifies a network's KnownNetwork path
// resolves to a live KnownNetwork handle.
func TestKnownNetworkMock_NetworkLinkage(t *testing.T) {
	iwdmock.StartMockNormal(t)

	ctx := context.Background()
	client := newMockClient(t, ctx)

	known := newPublicMockNetwork(t, ctx, client, "KnownNet")
	path, err := known.KnownNetwork(ctx)
	require.NoError(t, err)
	require.NotNil(t, path)

	kn, err := client.KnownNetwork(ctx, *path)
	require.NoError(t, err)
	name, err := kn.Name(ctx)
	require.NoError(t, err)
	require.Equal(t, "KnownNet", name)
}

func TestKnownNetworkMock_SetAutoConnect(t *testing.T) {
	iwdmock.StartMockNormal(t)

	ctx := context.Background()
	client := newMockClient(t, ctx)

	known := newPublicMockKnownNetwork(t, ctx, client, "KnownNet")
	require.NoError(t, known.SetAutoConnect(ctx, false))

	auto, err := known.AutoConnect(ctx)
	require.NoError(t, err)
	require.False(t, auto)
}

func TestKnownNetworkMock_Forget(t *testing.T) {
	iwdmock.StartMockNormal(t)

	ctx := context.Background()
	client := newMockClient(t, ctx)

	known := newPublicMockKnownNetwork(t, ctx, client, "KnownNet")
	require.NoError(t, known.Forget(ctx))
}

// TestKnownNetworkMock_SubscribeAutoConnectChanged exercises raw-iwdbus signal
// plumbing: setting AutoConnect emits a PropertiesChanged the subscription must
// deliver.
func TestKnownNetworkMock_SubscribeAutoConnectChanged(t *testing.T) {
	iwdmock.StartMockNormal(t)

	ctx := context.Background()
	k := newRawMockKnownNetwork(t, knownNetworkPath)

	received := make(chan bool, 2)
	unsubscribe, err := k.SubscribeAutoConnectChanged(ctx, func(auto bool) {
		received <- auto
	})
	require.NoError(t, err)

	require.NoError(t, k.SetAutoConnect(ctx, false))
	select {
	case got := <-received:
		require.False(t, got)
	case <-time.After(signalTimeout):
		t.Fatal("timed out waiting for autoconnect=false callback")
	}

	require.NoError(t, unsubscribe.Unsubscribe())
}

func TestKnownNetworkMock_AllKnownNetworks_Empty(t *testing.T) {
	iwdmock.StartMockWithoutKnownNetwork(t)

	ctx := context.Background()
	client := newMockClient(t, ctx)

	refs, err := client.Daemon().KnownNetworks(ctx)
	require.NoError(t, err)
	require.Empty(t, refs)

	known, err := client.AllKnownNetworks(ctx)
	require.NoError(t, err)
	require.Empty(t, known)
}

// -----------------------------------------------------------------------------
// CLI (`spiderw known-network …`) against the mock
// -----------------------------------------------------------------------------

func runSpiderKnownNetwork(t *testing.T, args ...string) (string, error) {
	t.Helper()
	return runSpider(t, append([]string{"known-network"}, args...)...)
}

// TestKnownNetworkMock_StatusJSON is the representative end-to-end CLI smoke for
// the known network: it drives `known-network status --json` through the full
// real-D-Bus stack and asserts the structured output for every exported known
// network.
func TestKnownNetworkMock_StatusJSON(t *testing.T) {
	iwdmock.StartMockNormal(t)

	list, out, err := runSpiderJSONArray(t, "known-network", "status")
	require.NoError(t, err, "output:\n%s", out)
	require.Len(t, list, len(mockKnownNetworks), "output:\n%s", out)

	byName := make(map[string]map[string]any, len(list))
	for _, entry := range list {
		byName[jsonGetString(t, entry, "Name")] = entry
	}

	known := byName["KnownNet"]
	require.NotNil(t, known, "KnownNet missing:\n%s", out)
	require.Equal(t, "psk", jsonGetString(t, known, "Type"))
	require.True(t, jsonGetBool(t, known, "AutoConnect"))
}
