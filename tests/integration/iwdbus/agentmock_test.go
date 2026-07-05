//go:build integration

package integration_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/chrispypip/spiderw"
	"github.com/chrispypip/spiderw/tests/testutil/iwdmock"
)

const (
	securedNetworkPath = "/net/connman/iwd/phy0/wlan0/secured_psk"
	// mockSecuredPassphrase mirrors the literal the iwd mock expects for the
	// secured, not-yet-known network (see tools/test-mocks/iwdmock).
	mockSecuredPassphrase = "mock-secret-passphrase"
)

// TestAgentMock_SecuredConnect drives the full callback loop: register an agent,
// connect the secured network, and confirm the mock called back for the
// passphrase and the network became connected.
func TestAgentMock_SecuredConnect(t *testing.T) {
	tmpDir := t.TempDir()
	iwdmock.StartMockNormal(t, tmpDir)
	ctx := context.Background()
	client := newMockClient(t, ctx)

	var calls atomic.Int32
	var gotPath atomic.Value
	agent, err := client.RegisterAgent(ctx, spiderw.AgentConfig{
		Passphrase: func(ctx context.Context, networkPath string) (string, error) {
			calls.Add(1)
			gotPath.Store(networkPath)
			return mockSecuredPassphrase, nil
		},
	})
	require.NoError(t, err)

	net, err := client.Network(ctx, securedNetworkPath)
	require.NoError(t, err)
	require.NoError(t, net.Connect(ctx))

	require.Equal(t, int32(1), calls.Load())
	require.Equal(t, securedNetworkPath, gotPath.Load())

	require.Eventually(t, func() bool {
		connected, _ := net.Connected(ctx)
		return connected
	}, 2*time.Second, 20*time.Millisecond)

	require.NoError(t, agent.Unregister(ctx))
}

// TestAgentMock_WrongPassphrase confirms a bad passphrase yields a connect
// failure rather than a connection.
func TestAgentMock_WrongPassphrase(t *testing.T) {
	tmpDir := t.TempDir()
	iwdmock.StartMockNormal(t, tmpDir)
	ctx := context.Background()
	client := newMockClient(t, ctx)

	_, err := client.RegisterAgent(ctx, spiderw.AgentConfig{
		Passphrase: func(ctx context.Context, networkPath string) (string, error) { return "wrong", nil },
	})
	require.NoError(t, err)

	net, err := client.Network(ctx, securedNetworkPath)
	require.NoError(t, err)
	require.Error(t, net.Connect(ctx))
}

// TestAgentMock_Decline confirms a declining callback (returns an error) maps to
// a connect failure.
func TestAgentMock_Decline(t *testing.T) {
	tmpDir := t.TempDir()
	iwdmock.StartMockNormal(t, tmpDir)
	ctx := context.Background()
	client := newMockClient(t, ctx)

	_, err := client.RegisterAgent(ctx, spiderw.AgentConfig{
		Passphrase: func(ctx context.Context, networkPath string) (string, error) {
			return "", context.Canceled
		},
	})
	require.NoError(t, err)

	net, err := client.Network(ctx, securedNetworkPath)
	require.NoError(t, err)
	require.Error(t, net.Connect(ctx))
}

// TestAgentMock_NoAgent confirms that connecting a secured, unknown network
// without a registered agent surfaces ErrNoAgent.
func TestAgentMock_NoAgent(t *testing.T) {
	tmpDir := t.TempDir()
	iwdmock.StartMockNormal(t, tmpDir)
	ctx := context.Background()
	client := newMockClient(t, ctx)

	net, err := client.Network(ctx, securedNetworkPath)
	require.NoError(t, err)
	err = net.Connect(ctx)
	require.Error(t, err)
	require.ErrorIs(t, err, spiderw.ErrNoAgent)
}

// TestAgentMock_DoubleRegisterRejected confirms the client rejects a second
// agent while one is registered, and that unregistering frees the slot.
func TestAgentMock_DoubleRegisterRejected(t *testing.T) {
	tmpDir := t.TempDir()
	iwdmock.StartMockNormal(t, tmpDir)
	ctx := context.Background()
	client := newMockClient(t, ctx)

	cfg := spiderw.AgentConfig{
		Passphrase: func(ctx context.Context, networkPath string) (string, error) { return mockSecuredPassphrase, nil },
	}
	agent, err := client.RegisterAgent(ctx, cfg)
	require.NoError(t, err)

	_, err = client.RegisterAgent(ctx, cfg)
	require.ErrorIs(t, err, spiderw.ErrInvalidState)

	// After unregistering, a fresh agent can be registered.
	require.NoError(t, agent.Unregister(ctx))
	agent2, err := client.RegisterAgent(ctx, cfg)
	require.NoError(t, err)
	require.NoError(t, agent2.Unregister(ctx))
}

// TestAgentMock_AlreadyExistsAcrossClients confirms iwd's AlreadyExists rejection
// is surfaced when a second connection registers an agent while the first holds
// the slot.
func TestAgentMock_AlreadyExistsAcrossClients(t *testing.T) {
	tmpDir := t.TempDir()
	iwdmock.StartMockNormal(t, tmpDir)
	ctx := context.Background()

	cfg := spiderw.AgentConfig{
		Passphrase: func(ctx context.Context, networkPath string) (string, error) { return mockSecuredPassphrase, nil },
	}

	clientA := newMockClient(t, ctx)
	_, err := clientA.RegisterAgent(ctx, cfg)
	require.NoError(t, err)

	clientB := newMockClient(t, ctx)
	_, err = clientB.RegisterAgent(ctx, cfg)
	require.Error(t, err)
	require.ErrorIs(t, err, spiderw.ErrAlreadyExists)
}

// TestAgentMock_ManagerUnavailable confirms that registration fails when the mock
// omits the AgentManager interface entirely.
func TestAgentMock_ManagerUnavailable(t *testing.T) {
	tmpDir := t.TempDir()
	iwdmock.StartMockWithoutAgent(t, tmpDir)
	ctx := context.Background()
	client := newMockClient(t, ctx)

	_, err := client.RegisterAgent(ctx, spiderw.AgentConfig{
		Passphrase: func(ctx context.Context, networkPath string) (string, error) { return mockSecuredPassphrase, nil },
	})
	require.Error(t, err)
	require.ErrorIs(t, err, spiderw.ErrUnavailable)
}

// TestAgentMock_CLISecuredConnect drives the secured connect through the CLI with
// a passphrase flag.
func TestAgentMock_CLISecuredConnect(t *testing.T) {
	tmpDir := t.TempDir()
	iwdmock.StartMockNormal(t, tmpDir)

	out, err := runSpiderNetwork(t, "SecuredNet", "connect", "--passphrase="+mockSecuredPassphrase)
	require.NoError(t, err, out)
	require.Contains(t, out, "true")
}

// TestAgentMock_CLISecuredConnectStdin drives the secured connect through the CLI
// reading the passphrase from stdin.
func TestAgentMock_CLISecuredConnectWrong(t *testing.T) {
	tmpDir := t.TempDir()
	iwdmock.StartMockNormal(t, tmpDir)

	out, err := runSpiderNetwork(t, "SecuredNet", "connect", "--passphrase=nope")
	require.Error(t, err, out)
}
