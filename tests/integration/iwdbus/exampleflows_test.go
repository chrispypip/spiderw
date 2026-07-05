//go:build integration

package integration_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/chrispypip/spiderw"
	"github.com/chrispypip/spiderw/tests/testutil/iwdmock"
)

// TestExampleFlows runs the same call sequences as the godoc examples in
// example_test.go against the iwd mock, so the documented flows are verified at
// runtime (the examples themselves are compile-checked but not executed, since
// they target the system bus and real iwd). Each subtest mirrors one Example*,
// swapping SystemBus for SessionBus and log.Fatal for require assertions.
func TestExampleFlows(t *testing.T) {
	tmpDir := t.TempDir()
	iwdmock.StartMockNormal(t, tmpDir)
	ctx := context.Background()

	// ExampleNewClient
	t.Run("NewClient", func(t *testing.T) {
		v, err := newMockClient(t, ctx).Daemon().Version(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, v)
	})

	// ExampleDaemon_Info
	t.Run("Daemon_Info", func(t *testing.T) {
		info, err := newMockClient(t, ctx).Daemon().Info(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, info.Version)
	})

	// ExampleDaemon_Adapters
	t.Run("Daemon_Adapters", func(t *testing.T) {
		refs, err := newMockClient(t, ctx).Daemon().Adapters(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, refs)
	})

	// ExampleClient_Adapter
	t.Run("Client_Adapter", func(t *testing.T) {
		client := newMockClient(t, ctx)
		refs, err := client.Daemon().Adapters(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, refs)
		adapter, err := client.Adapter(ctx, refs[0].Path)
		require.NoError(t, err)
		_, err = adapter.Powered(ctx)
		require.NoError(t, err)
	})

	// ExampleClient_Device
	t.Run("Client_Device", func(t *testing.T) {
		client := newMockClient(t, ctx)
		refs, err := client.Daemon().Devices(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, refs)
		device, err := client.Device(ctx, refs[0].Path)
		require.NoError(t, err)
		_, err = device.Properties(ctx)
		require.NoError(t, err)
	})

	// ExampleClient_Station
	t.Run("Client_Station", func(t *testing.T) {
		client := newMockClient(t, ctx)
		stations, err := client.AllStations(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, stations)
		_, err = stations[0].Properties(ctx)
		require.NoError(t, err)
	})

	// ExampleStation_Scan
	t.Run("Station_Scan", func(t *testing.T) {
		client := newMockClient(t, ctx)
		stations, err := client.AllStations(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, stations)
		require.NoError(t, stations[0].Scan(ctx))
		_, err = stations[0].OrderedNetworks(ctx)
		require.NoError(t, err)
	})

	// ExampleStation_ConnectHiddenNetwork
	t.Run("Station_ConnectHiddenNetwork", func(t *testing.T) {
		client := newMockClient(t, ctx)
		agent, err := client.RegisterAgent(ctx, spiderw.AgentConfig{
			Passphrase: func(ctx context.Context, networkPath string) (string, error) { return mockSecuredPassphrase, nil },
		})
		require.NoError(t, err)
		defer func() { _ = agent.Unregister(ctx) }()

		stations, err := client.AllStations(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, stations)
		require.NoError(t, stations[0].ConnectHiddenNetwork(ctx, "HiddenSecured"))
	})

	// ExampleClient_BasicServiceSet
	t.Run("Client_BasicServiceSet", func(t *testing.T) {
		client := newMockClient(t, ctx)
		refs, err := client.Daemon().BasicServiceSets(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, refs)
		bss, err := client.BasicServiceSet(ctx, refs[0].Path)
		require.NoError(t, err)
		_, err = bss.Address(ctx)
		require.NoError(t, err)
	})

	// ExampleClient_AllBasicServiceSets
	t.Run("Client_AllBasicServiceSets", func(t *testing.T) {
		client := newMockClient(t, ctx)
		bsses, err := client.AllBasicServiceSets(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, bsses)
		for _, bss := range bsses {
			_, err := bss.Address(ctx)
			require.NoError(t, err)
		}
	})

	// ExampleClient_Network
	t.Run("Client_Network", func(t *testing.T) {
		client := newMockClient(t, ctx)
		refs, err := client.Daemon().Networks(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, refs)
		network, err := client.Network(ctx, refs[0].Path)
		require.NoError(t, err)
		_, err = network.Properties(ctx)
		require.NoError(t, err)
	})

	// ExampleNetwork_Connect (refs[0] is a known network, which connects without
	// an agent).
	t.Run("Network_Connect", func(t *testing.T) {
		client := newMockClient(t, ctx)
		refs, err := client.Daemon().Networks(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, refs)
		network, err := client.Network(ctx, refs[0].Path)
		require.NoError(t, err)
		require.NoError(t, network.Connect(ctx))
	})

	// ExampleClient_RegisterAgent
	t.Run("Client_RegisterAgent", func(t *testing.T) {
		client := newMockClient(t, ctx)
		agent, err := client.RegisterAgent(ctx, spiderw.AgentConfig{
			Passphrase: func(ctx context.Context, networkPath string) (string, error) {
				return mockSecuredPassphrase, nil
			},
		})
		require.NoError(t, err)
		defer func() { _ = agent.Unregister(ctx) }()

		refs, err := client.Daemon().Networks(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, refs)
		network, err := client.Network(ctx, refs[0].Path)
		require.NoError(t, err)
		require.NoError(t, network.Connect(ctx))
	})

	// ExampleNetwork_ExtendedServiceSet
	t.Run("Network_ExtendedServiceSet", func(t *testing.T) {
		client := newMockClient(t, ctx)
		refs, err := client.Daemon().Networks(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, refs)
		network, err := client.Network(ctx, refs[0].Path)
		require.NoError(t, err)
		paths, err := network.ExtendedServiceSet(ctx)
		require.NoError(t, err)
		for _, path := range paths {
			bss, err := client.BasicServiceSet(ctx, path)
			require.NoError(t, err)
			_, err = bss.Address(ctx)
			require.NoError(t, err)
		}
	})

	// ExampleClient_KnownNetwork
	t.Run("Client_KnownNetwork", func(t *testing.T) {
		client := newMockClient(t, ctx)
		refs, err := client.Daemon().KnownNetworks(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, refs)
		known, err := client.KnownNetwork(ctx, refs[0].Path)
		require.NoError(t, err)
		_, err = known.Properties(ctx)
		require.NoError(t, err)
	})

	// ExampleKnownNetwork_SetAutoConnect
	t.Run("KnownNetwork_SetAutoConnect", func(t *testing.T) {
		client := newMockClient(t, ctx)
		refs, err := client.Daemon().KnownNetworks(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, refs)
		known, err := client.KnownNetwork(ctx, refs[0].Path)
		require.NoError(t, err)
		require.NoError(t, known.SetAutoConnect(ctx, false))
	})

	// ExampleClient_AllAdapters
	t.Run("Client_AllAdapters", func(t *testing.T) {
		client := newMockClient(t, ctx)
		adapters, err := client.AllAdapters(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, adapters)
		for _, adapter := range adapters {
			_, err := adapter.Name(ctx)
			require.NoError(t, err)
			_, err = adapter.Powered(ctx)
			require.NoError(t, err)
		}
	})

	// ExampleAdapter_Properties (after fixing it to discover the adapter path).
	t.Run("Adapter_Properties", func(t *testing.T) {
		client := newMockClient(t, ctx)
		refs, err := client.Daemon().Adapters(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, refs)
		adapter, err := client.Adapter(ctx, refs[0].Path)
		require.NoError(t, err)
		_, err = adapter.Properties(ctx)
		require.NoError(t, err)
	})

	// ExampleAdapter_SupportsMode
	t.Run("Adapter_SupportsMode", func(t *testing.T) {
		client := newMockClient(t, ctx)
		refs, err := client.Daemon().Adapters(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, refs)
		adapter, err := client.Adapter(ctx, refs[0].Path)
		require.NoError(t, err)
		_, err = adapter.SupportsMode(ctx, spiderw.ModeStation)
		require.NoError(t, err)
	})

	// ExampleAdapter_SubscribePoweredChanged
	t.Run("Adapter_SubscribePoweredChanged", func(t *testing.T) {
		client := newMockClient(t, ctx)
		refs, err := client.Daemon().Adapters(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, refs)
		adapter, err := client.Adapter(ctx, refs[0].Path)
		require.NoError(t, err)
		unsubscribe, err := adapter.SubscribePoweredChanged(ctx, func(bool) {})
		require.NoError(t, err)
		require.NoError(t, unsubscribe.Unsubscribe())
	})

	// Example_errorHandling (the happy path against a healthy mock; the error
	// classification it shows is unit-tested elsewhere).
	t.Run("errorHandling", func(t *testing.T) {
		_, err := newMockClient(t, ctx).Daemon().Info(ctx)
		require.NoError(t, err)
	})
}
