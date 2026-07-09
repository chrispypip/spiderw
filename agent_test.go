//go:build unit

package spiderw

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/chrispypip/spiderw/internal/core"
)

func TestAgent_Public(t *testing.T) {
	t.Parallel()

	t.Run("RegisterAgent", func(t *testing.T) {
		t.Parallel()
		t.Run("Success", testAgentPublic_Register_Success)
		t.Run("NoCallbacks", testAgentPublic_Register_NoCallbacks)
		t.Run("AlreadyRegistered", testAgentPublic_Register_AlreadyRegistered)
		t.Run("Closed", testAgentPublic_Register_Closed)
		t.Run("FactoryError", testAgentPublic_Register_FactoryError)
		t.Run("ReRegisterAfterUnregister", testAgentPublic_Register_ReRegisterAfterUnregister)
	})

	t.Run("Unregister", func(t *testing.T) {
		t.Parallel()
		t.Run("CallsCore", testAgentPublic_Unregister_CallsCore)
		t.Run("BackendError", testAgentPublic_Unregister_BackendError)
		t.Run("NilReceiver", testAgentPublic_Unregister_NilReceiver)
	})

	t.Run("Close", func(t *testing.T) {
		t.Parallel()
		t.Run("UnregistersAgent", testAgentPublic_Close_UnregistersAgent)
	})
}

func factoryReturning(fake core.AgentIface) func(ctx context.Context, cc core.CredentialCallbacks) (core.AgentIface, error) {
	return func(ctx context.Context, cc core.CredentialCallbacks) (core.AgentIface, error) {
		return fake, nil
	}
}

func testAgentPublic_Register_Success(t *testing.T) {
	t.Parallel()
	fake := &fakeCoreAgent{}
	c := newAgentTestClient(t, factoryReturning(fake))
	defer func() { _ = c.Close() }()

	agent, err := c.RegisterAgent(context.Background(), validAgentConfig())
	require.NoError(t, err)
	require.NotNil(t, agent)
}

func testAgentPublic_Register_NoCallbacks(t *testing.T) {
	t.Parallel()
	c := newAgentTestClient(t, factoryReturning(&fakeCoreAgent{}))
	defer func() { _ = c.Close() }()

	agent, err := c.RegisterAgent(context.Background(), AgentConfig{OnCancel: func(string) {}})
	require.Nil(t, agent)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidArgument)
	var pe *Error
	require.True(t, errors.As(err, &pe))
	require.Equal(t, ResourceAgent, pe.Resource)
}

func testAgentPublic_Register_AlreadyRegistered(t *testing.T) {
	t.Parallel()
	c := newAgentTestClient(t, factoryReturning(&fakeCoreAgent{}))
	defer func() { _ = c.Close() }()

	_, err := c.RegisterAgent(context.Background(), validAgentConfig())
	require.NoError(t, err)

	agent, err := c.RegisterAgent(context.Background(), validAgentConfig())
	require.Nil(t, agent)
	require.ErrorIs(t, err, ErrInvalidState)
	var pe *Error
	require.True(t, errors.As(err, &pe))
	require.Equal(t, ResourceAgent, pe.Resource)
}

func testAgentPublic_Register_Closed(t *testing.T) {
	t.Parallel()
	c := newAgentTestClient(t, factoryReturning(&fakeCoreAgent{}))
	require.NoError(t, c.Close())

	agent, err := c.RegisterAgent(context.Background(), validAgentConfig())
	require.Nil(t, agent)
	require.ErrorIs(t, err, ErrInvalidState)
}

func testAgentPublic_Register_FactoryError(t *testing.T) {
	t.Parallel()
	wantErr := core.WrapAgentUnavailable("NewAgent", "boom", core.ErrCore)
	c := newAgentTestClient(t, func(ctx context.Context, cc core.CredentialCallbacks) (core.AgentIface, error) {
		return nil, wantErr
	})
	defer func() { _ = c.Close() }()

	agent, err := c.RegisterAgent(context.Background(), validAgentConfig())
	require.Nil(t, agent)
	require.Error(t, err)

	// A failed registration must not occupy the single agent slot.
	fake := &fakeCoreAgent{}
	c.wire.AgentFactory = factoryReturning(fake)
	retry, err := c.RegisterAgent(context.Background(), validAgentConfig())
	require.NoError(t, err)
	require.NotNil(t, retry)
}

func testAgentPublic_Register_ReRegisterAfterUnregister(t *testing.T) {
	t.Parallel()
	c := newAgentTestClient(t, factoryReturning(&fakeCoreAgent{}))
	defer func() { _ = c.Close() }()

	agent, err := c.RegisterAgent(context.Background(), validAgentConfig())
	require.NoError(t, err)
	require.NoError(t, agent.Unregister(context.Background()))

	// The slot is freed, so a fresh agent can be registered.
	c.wire.AgentFactory = factoryReturning(&fakeCoreAgent{})
	again, err := c.RegisterAgent(context.Background(), validAgentConfig())
	require.NoError(t, err)
	require.NotNil(t, again)
}

func testAgentPublic_Unregister_CallsCore(t *testing.T) {
	t.Parallel()
	fake := &fakeCoreAgent{}
	c := newAgentTestClient(t, factoryReturning(fake))
	defer func() { _ = c.Close() }()

	agent, err := c.RegisterAgent(context.Background(), validAgentConfig())
	require.NoError(t, err)
	require.NoError(t, agent.Unregister(context.Background()))
	require.Equal(t, 1, fake.calls())
}

func testAgentPublic_Unregister_BackendError(t *testing.T) {
	t.Parallel()
	fake := &fakeCoreAgent{unregisterErr: core.WrapAgentUnavailable("op", "boom", errors.New("x"))}
	c := newAgentTestClient(t, factoryReturning(fake))
	defer func() { _ = c.Close() }()

	agent, err := c.RegisterAgent(context.Background(), validAgentConfig())
	require.NoError(t, err)

	err = agent.Unregister(context.Background())
	require.Error(t, err)
	var pe *Error
	require.ErrorAs(t, err, &pe)
	require.Equal(t, ResourceAgent, pe.Resource)
}

func testAgentPublic_Unregister_NilReceiver(t *testing.T) {
	t.Parallel()
	var a *Agent
	err := a.Unregister(context.Background())
	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidState)
}

func testAgentPublic_Close_UnregistersAgent(t *testing.T) {
	t.Parallel()
	fake := &fakeCoreAgent{}
	c := newAgentTestClient(t, factoryReturning(fake))

	_, err := c.RegisterAgent(context.Background(), validAgentConfig())
	require.NoError(t, err)

	require.NoError(t, c.Close())
	require.Equal(t, 1, fake.calls())
}
