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
	securedNetworkPath = "/net/connman/iwd/0/3/536563757265644e6574_psk"
	// mockSecuredPassphrase mirrors the literal the iwd mock expects for the
	// secured, not-yet-known network (see tools/test-mocks/iwdmock).
	mockSecuredPassphrase = "mock-secret-passphrase"

	// enterpriseNetworkPath is the mock's unknown 802.1x (EAP) network; the
	// username/password below are the literals its mock agent expects.
	enterpriseNetworkPath = "/net/connman/iwd/0/3/506561704e6574_8021x"
	mockEapUser           = "mock-eap-user"
	mockEapPassword       = "mock-eap-password"

	// The mock's other two 802.1x networks: TtlsNet drives RequestUserPassword
	// (iwd supplies the identity), TlsNet drives RequestPrivateKeyPassphrase.
	ttlsNetworkPath = "/net/connman/iwd/0/3/54746c734e6574_8021x"
	tlsNetworkPath  = "/net/connman/iwd/0/3/546c734e6574_8021x"
	mockEapIdentity = "mock-eap-identity"
	mockEapTtlsPass = "mock-ttls-password"
	mockEapKeyPass  = "mock-tls-key-pass"
)

// TestAgentMock_SecuredConnect drives the full callback loop: register an agent,
// connect the secured network, and confirm the mock called back for the
// passphrase and the network became connected.
func TestAgentMock_SecuredConnect(t *testing.T) {
	iwdmock.StartMockNormal(t)
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
	iwdmock.StartMockNormal(t)
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
	iwdmock.StartMockNormal(t)
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
	iwdmock.StartMockNormal(t)
	ctx := context.Background()
	client := newMockClient(t, ctx)

	net, err := client.Network(ctx, securedNetworkPath)
	require.NoError(t, err)
	err = net.Connect(ctx)
	require.Error(t, err)
	require.ErrorIs(t, err, spiderw.ErrNoAgent)
}

// TestAgentMock_EnterpriseConnect drives the 802.1x callback loop: register an
// agent, connect the unknown 802.1x network, and confirm the mock called back
// for the username and password and the network became connected.
func TestAgentMock_EnterpriseConnect(t *testing.T) {
	iwdmock.StartMockNormal(t)
	ctx := context.Background()
	client := newMockClient(t, ctx)

	var calls atomic.Int32
	var gotPath atomic.Value
	agent, err := client.RegisterAgent(ctx, spiderw.AgentConfig{
		UserNameAndPassword: func(ctx context.Context, networkPath string) (string, string, error) {
			calls.Add(1)
			gotPath.Store(networkPath)
			return mockEapUser, mockEapPassword, nil
		},
	})
	require.NoError(t, err)

	net, err := client.Network(ctx, enterpriseNetworkPath)
	require.NoError(t, err)
	require.NoError(t, net.Connect(ctx))

	require.Equal(t, int32(1), calls.Load())
	require.Equal(t, enterpriseNetworkPath, gotPath.Load())

	require.Eventually(t, func() bool {
		connected, _ := net.Connected(ctx)
		return connected
	}, 2*time.Second, 20*time.Millisecond)

	require.NoError(t, agent.Unregister(ctx))
}

// TestAgentMock_EnterpriseWrongCredentials confirms wrong 802.1x credentials
// yield a connect failure rather than a connection.
func TestAgentMock_EnterpriseWrongCredentials(t *testing.T) {
	iwdmock.StartMockNormal(t)
	ctx := context.Background()
	client := newMockClient(t, ctx)

	_, err := client.RegisterAgent(ctx, spiderw.AgentConfig{
		UserNameAndPassword: func(ctx context.Context, networkPath string) (string, string, error) {
			return mockEapUser, "wrong-password", nil
		},
	})
	require.NoError(t, err)

	net, err := client.Network(ctx, enterpriseNetworkPath)
	require.NoError(t, err)
	require.Error(t, net.Connect(ctx))
}

// TestAgentMock_EnterpriseNoAgent confirms connecting the 802.1x network without
// a registered agent surfaces ErrNoAgent (the agent is required for EAP creds).
func TestAgentMock_EnterpriseNoAgent(t *testing.T) {
	iwdmock.StartMockNormal(t)
	ctx := context.Background()
	client := newMockClient(t, ctx)

	net, err := client.Network(ctx, enterpriseNetworkPath)
	require.NoError(t, err)
	err = net.Connect(ctx)
	require.ErrorIs(t, err, spiderw.ErrNoAgent)
}

// TestAgentMock_EnterpriseUserPassword drives the password-only 802.1x path: iwd
// supplies the configured identity and the mock calls RequestUserPassword.
func TestAgentMock_EnterpriseUserPassword(t *testing.T) {
	iwdmock.StartMockNormal(t)
	ctx := context.Background()
	client := newMockClient(t, ctx)

	var calls atomic.Int32
	var gotUser atomic.Value
	agent, err := client.RegisterAgent(ctx, spiderw.AgentConfig{
		UserPassword: func(ctx context.Context, networkPath, user string) (string, error) {
			calls.Add(1)
			gotUser.Store(user)
			return mockEapTtlsPass, nil
		},
	})
	require.NoError(t, err)

	net, err := client.Network(ctx, ttlsNetworkPath)
	require.NoError(t, err)
	require.NoError(t, net.Connect(ctx))

	require.Equal(t, int32(1), calls.Load())
	require.Equal(t, mockEapIdentity, gotUser.Load()) // iwd supplies the identity

	require.Eventually(t, func() bool {
		connected, _ := net.Connected(ctx)
		return connected
	}, 2*time.Second, 20*time.Millisecond)

	require.NoError(t, agent.Unregister(ctx))
}

// TestAgentMock_EnterpriseUserPasswordWrong confirms a wrong password yields a
// connect failure.
func TestAgentMock_EnterpriseUserPasswordWrong(t *testing.T) {
	iwdmock.StartMockNormal(t)
	ctx := context.Background()
	client := newMockClient(t, ctx)

	_, err := client.RegisterAgent(ctx, spiderw.AgentConfig{
		UserPassword: func(ctx context.Context, networkPath, user string) (string, error) {
			return "wrong", nil
		},
	})
	require.NoError(t, err)

	net, err := client.Network(ctx, ttlsNetworkPath)
	require.NoError(t, err)
	require.Error(t, net.Connect(ctx))
}

// TestAgentMock_EnterpriseUserPasswordNoAgent confirms the password-only network
// surfaces ErrNoAgent without a registered agent.
func TestAgentMock_EnterpriseUserPasswordNoAgent(t *testing.T) {
	iwdmock.StartMockNormal(t)
	ctx := context.Background()
	client := newMockClient(t, ctx)

	net, err := client.Network(ctx, ttlsNetworkPath)
	require.NoError(t, err)
	require.ErrorIs(t, net.Connect(ctx), spiderw.ErrNoAgent)
}

// TestAgentMock_EnterprisePrivateKey drives the EAP-TLS path: the mock calls
// RequestPrivateKeyPassphrase for the encrypted client key.
func TestAgentMock_EnterprisePrivateKey(t *testing.T) {
	iwdmock.StartMockNormal(t)
	ctx := context.Background()
	client := newMockClient(t, ctx)

	var calls atomic.Int32
	var gotPath atomic.Value
	agent, err := client.RegisterAgent(ctx, spiderw.AgentConfig{
		PrivateKeyPassphrase: func(ctx context.Context, networkPath string) (string, error) {
			calls.Add(1)
			gotPath.Store(networkPath)
			return mockEapKeyPass, nil
		},
	})
	require.NoError(t, err)

	net, err := client.Network(ctx, tlsNetworkPath)
	require.NoError(t, err)
	require.NoError(t, net.Connect(ctx))

	require.Equal(t, int32(1), calls.Load())
	require.Equal(t, tlsNetworkPath, gotPath.Load())

	require.Eventually(t, func() bool {
		connected, _ := net.Connected(ctx)
		return connected
	}, 2*time.Second, 20*time.Millisecond)

	require.NoError(t, agent.Unregister(ctx))
}

// TestAgentMock_EnterprisePrivateKeyWrong confirms a wrong key passphrase yields
// a connect failure.
func TestAgentMock_EnterprisePrivateKeyWrong(t *testing.T) {
	iwdmock.StartMockNormal(t)
	ctx := context.Background()
	client := newMockClient(t, ctx)

	_, err := client.RegisterAgent(ctx, spiderw.AgentConfig{
		PrivateKeyPassphrase: func(ctx context.Context, networkPath string) (string, error) {
			return "wrong", nil
		},
	})
	require.NoError(t, err)

	net, err := client.Network(ctx, tlsNetworkPath)
	require.NoError(t, err)
	require.Error(t, net.Connect(ctx))
}

// TestAgentMock_EnterprisePrivateKeyNoAgent confirms the EAP-TLS network surfaces
// ErrNoAgent without a registered agent.
func TestAgentMock_EnterprisePrivateKeyNoAgent(t *testing.T) {
	iwdmock.StartMockNormal(t)
	ctx := context.Background()
	client := newMockClient(t, ctx)

	net, err := client.Network(ctx, tlsNetworkPath)
	require.NoError(t, err)
	require.ErrorIs(t, net.Connect(ctx), spiderw.ErrNoAgent)
}

// TestAgentMock_DoubleRegisterRejected confirms the client rejects a second
// agent while one is registered, and that unregistering frees the slot.
func TestAgentMock_DoubleRegisterRejected(t *testing.T) {
	iwdmock.StartMockNormal(t)
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
	iwdmock.StartMockNormal(t)
	ctx := context.Background()

	cfg := spiderw.AgentConfig{
		Passphrase: func(ctx context.Context, networkPath string) (string, error) { return mockSecuredPassphrase, nil },
	}

	client1 := newMockClient(t, ctx)
	_, err := client1.RegisterAgent(ctx, cfg)
	require.NoError(t, err)

	clientB := newMockClient(t, ctx)
	_, err = clientB.RegisterAgent(ctx, cfg)
	require.Error(t, err)
	require.ErrorIs(t, err, spiderw.ErrAlreadyExists)
}

// TestAgentMock_ManagerUnavailable confirms that registration fails when the mock
// omits the AgentManager interface entirely.
func TestAgentMock_ManagerUnavailable(t *testing.T) {
	iwdmock.StartMockWithoutAgent(t)
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
func TestAgentMock_CLI_SecuredConnect(t *testing.T) {
	iwdmock.StartMockNormal(t)

	out, err := runSpiderNetwork(t, "SecuredNet", "connect", "--passphrase="+mockSecuredPassphrase)
	require.NoError(t, err, out)
	require.Contains(t, out, "true")
}

// TestAgentMock_CLISecuredConnectStdin drives the secured connect through the CLI
// reading the passphrase from stdin.
func TestAgentMock_CLI_SecuredConnectWrong(t *testing.T) {
	iwdmock.StartMockNormal(t)

	out, err := runSpiderNetwork(t, "SecuredNet", "connect", "--passphrase=nope")
	require.Error(t, err, out)
}
