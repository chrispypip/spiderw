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

func TestNetwork(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("PropertiesAndType", func(t *testing.T) {
		t.Parallel()

		n := newNetwork((&fakeCoreNetwork{}).setProps(core.NetworkProperties{
			Name:               "OpenNet",
			Device:             "/net/connman/iwd/phy0/wlan0",
			Type:               core.NetworkTypeOpen,
			ExtendedServiceSet: []string{"/net/connman/iwd/phy0/wlan0/aabbccddeeff"},
		}), "/net/connman/iwd/phy0/wlan0/open")
		require.NotNil(t, n)
		require.Equal(t, "/net/connman/iwd/phy0/wlan0/open", n.Path())

		secType, err := n.Type(ctx)
		require.NoError(t, err)
		require.Equal(t, NetworkTypeOpen, secType)

		props, err := n.Properties(ctx)
		require.NoError(t, err)
		require.Equal(t, "OpenNet", props.Name)
		require.Equal(t, NetworkTypeOpen, props.Type)
		require.Equal(t, []string{"/net/connman/iwd/phy0/wlan0/aabbccddeeff"}, props.ExtendedServiceSet)
	})

	t.Run("ConnectErrorSentinels", func(t *testing.T) {
		t.Parallel()

		// A core error carrying an iwd sentinel must remain matchable through the
		// public error chain via the re-exported public sentinel.
		cases := []struct {
			name     string
			coreErr  error
			publicIs error
		}{
			{name: "NoAgent", coreErr: core.ErrNoAgent, publicIs: ErrNoAgent},
			{name: "Busy", coreErr: core.ErrBusy, publicIs: ErrBusy},
			{name: "Failed", coreErr: core.ErrFailed, publicIs: ErrFailed},
			{name: "InProgress", coreErr: core.ErrInProgress, publicIs: ErrInProgress},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				connectErr := core.WrapNetworkUnavailable("Network.Connect", "connect failed", tc.coreErr)
				n := newNetwork((&fakeCoreNetwork{}).setConnectErr(connectErr), "/path")

				err := n.Connect(ctx)
				require.Error(t, err)
				require.ErrorIs(t, err, tc.publicIs)
			})
		}
	})

	t.Run("NilReceiver", func(t *testing.T) {
		t.Parallel()
		var n *Network
		require.Equal(t, "", n.Path())
		_, err := n.Name(ctx)
		require.ErrorIs(t, err, ErrInternal)
	})
}

func TestClientAllNetworks(t *testing.T) {
	ctx := context.Background()

	newClient := func(refs []core.NetworkRef, daemonErr error) *Client {
		fakeDaemon := &fakeCoreDaemon{}
		fakeDaemon.setNetworks(refs)
		if daemonErr != nil {
			fakeDaemon.setErr(daemonErr)
		}
		factory := func(ctx context.Context, path string) (core.NetworkIface, error) {
			return (&fakeCoreNetwork{}).setProps(core.NetworkProperties{Name: path}), nil
		}
		wire := &connect.Wiring{
			Conn:           &dbus.Conn{},
			Daemon:         fakeDaemon,
			Cleanup:        func() error { return nil },
			NetworkFactory: factory,
		}
		return &Client{
			daemon:  newDaemon(fakeDaemon),
			wire:    wire,
			cleanup: wire.Cleanup,
		}
	}

	t.Run("Success", func(t *testing.T) {
		refs := []core.NetworkRef{
			{Path: "/net/connman/iwd/phy0/wlan0/open", Name: "OpenNet"},
			{Path: "/net/connman/iwd/phy0/wlan0/secured_psk", Name: "SecuredNet"},
		}
		c := newClient(refs, nil)

		networks, err := c.AllNetworks(ctx)
		require.NoError(t, err)
		require.Len(t, networks, len(refs))
		for i, n := range networks {
			require.Equal(t, refs[i].Path, n.Path())
		}
	})

	t.Run("Empty", func(t *testing.T) {
		c := newClient(nil, nil)
		networks, err := c.AllNetworks(ctx)
		require.NoError(t, err)
		require.NotNil(t, networks)
		require.Empty(t, networks)
	})

	t.Run("Closed", func(t *testing.T) {
		c := newClient([]core.NetworkRef{{Path: "/net/connman/iwd/phy0/wlan0/open", Name: "OpenNet"}}, nil)
		require.NoError(t, c.Close())
		networks, err := c.AllNetworks(ctx)
		require.Nil(t, networks)
		require.ErrorIs(t, err, ErrInvalidState)
	})

	t.Run("NilReceiver", func(t *testing.T) {
		var c *Client
		networks, err := c.AllNetworks(ctx)
		require.Nil(t, networks)
		require.ErrorIs(t, err, ErrInternal)
	})
}
