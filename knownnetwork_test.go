//go:build unit

package spiderw

import (
	"context"
	"testing"

	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/require"

	"github.com/chrispypip/spiderw/internal/connect"
	"github.com/chrispypip/spiderw/internal/core"
)

func TestKnownNetwork(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("PropertiesAndType", func(t *testing.T) {
		t.Parallel()

		k := newKnownNetwork((&fakeCoreKnownNetwork{}).setProps(validCoreKnownNetworkProps()), "/net/connman/iwd/abc")
		require.NotNil(t, k)
		require.Equal(t, "/net/connman/iwd/abc", k.Path())

		secType, err := k.Type(ctx)
		require.NoError(t, err)
		require.Equal(t, NetworkTypePSK, secType)

		props, err := k.Properties(ctx)
		require.NoError(t, err)
		require.Equal(t, "HomeNet", props.Name)
		require.Equal(t, NetworkTypePSK, props.Type)
		require.NotNil(t, props.LastConnectedTime)
		require.True(t, props.AutoConnect)
	})

	t.Run("SetAutoConnect", func(t *testing.T) {
		t.Parallel()
		k := newKnownNetwork((&fakeCoreKnownNetwork{}).setProps(validCoreKnownNetworkProps()), "/path")
		require.NoError(t, k.SetAutoConnect(ctx, false))

		auto, err := k.AutoConnect(ctx)
		require.NoError(t, err)
		require.False(t, auto)
	})

	t.Run("ForgetErrorMatchesSentinel", func(t *testing.T) {
		t.Parallel()
		// A core error carrying an iwd sentinel stays matchable through the public
		// error chain.
		forgetErr := core.WrapKnownNetworkUnavailable("KnownNetwork.Forget", "forget failed", core.ErrBusy)
		k := newKnownNetwork((&fakeCoreKnownNetwork{}).setForgetErr(forgetErr), "/path")

		err := k.Forget(ctx)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrBusy)
	})

	t.Run("NilReceiver", func(t *testing.T) {
		t.Parallel()
		var k *KnownNetwork
		require.Equal(t, "", k.Path())
		_, err := k.Name(ctx)
		require.ErrorIs(t, err, ErrInternal)
	})

	t.Run("NewNilCore", func(t *testing.T) {
		t.Parallel()
		require.Nil(t, newKnownNetwork(nil, "/path"))
	})
}

func TestClientAllKnownNetworks(t *testing.T) {
	ctx := context.Background()

	newClient := func(refs []core.KnownNetworkRef, daemonErr error) *Client {
		fakeDaemon := &fakeCoreDaemon{}
		fakeDaemon.setKnownNetworks(refs)
		if daemonErr != nil {
			fakeDaemon.setErr(daemonErr)
		}
		factory := func(ctx context.Context, path string) (core.KnownNetworkIface, error) {
			return (&fakeCoreKnownNetwork{}).setProps(core.KnownNetworkProperties{Name: path}), nil
		}
		wire := &connect.Wiring{
			Conn:                &dbus.Conn{},
			Daemon:              fakeDaemon,
			Cleanup:             func() error { return nil },
			KnownNetworkFactory: factory,
		}
		return &Client{
			daemon:  newDaemon(fakeDaemon),
			wire:    wire,
			cleanup: wire.Cleanup,
		}
	}

	t.Run("Success", func(t *testing.T) {
		refs := []core.KnownNetworkRef{
			{Path: "/net/connman/iwd/abc", Name: "HomeNet"},
			{Path: "/net/connman/iwd/def", Name: "CafeNet"},
		}
		c := newClient(refs, nil)

		known, err := c.AllKnownNetworks(ctx)
		require.NoError(t, err)
		require.Len(t, known, len(refs))
		for i, k := range known {
			require.Equal(t, refs[i].Path, k.Path())
		}
	})

	t.Run("Empty", func(t *testing.T) {
		c := newClient(nil, nil)
		known, err := c.AllKnownNetworks(ctx)
		require.NoError(t, err)
		require.NotNil(t, known)
		require.Empty(t, known)
	})

	t.Run("Closed", func(t *testing.T) {
		c := newClient([]core.KnownNetworkRef{{Path: "/net/connman/iwd/abc", Name: "HomeNet"}}, nil)
		require.NoError(t, c.Close())
		known, err := c.AllKnownNetworks(ctx)
		require.Nil(t, known)
		require.ErrorIs(t, err, ErrInvalidState)
	})

	t.Run("NilReceiver", func(t *testing.T) {
		var c *Client
		known, err := c.AllKnownNetworks(ctx)
		require.Nil(t, known)
		require.ErrorIs(t, err, ErrInternal)
	})
}
