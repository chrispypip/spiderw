//go:build unit

package spiderw

import (
	"context"
	"errors"
	"testing"

	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/require"

	"github.com/chrispypip/spiderw/internal/connect"
	"github.com/chrispypip/spiderw/internal/core"
)

func TestNetwork_Public(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("PropertiesAndType", func(t *testing.T) {
		t.Parallel()

		kn := "/net/connman/iwd/known/OpenNet"
		n := newNetwork((&fakeCoreNetwork{}).setProps(core.NetworkProperties{
			Name:               "OpenNet",
			Connected:          true,
			Device:             "/net/connman/iwd/phy0/wlan0",
			Type:               core.NetworkTypeOpen,
			KnownNetwork:       &kn,
			ExtendedServiceSet: []string{"/net/connman/iwd/phy0/wlan0/aabbccddeeff"},
		}), "/net/connman/iwd/phy0/wlan0/open")
		require.NotNil(t, n)
		require.Equal(t, "/net/connman/iwd/phy0/wlan0/open", n.Path())

		secType, err := n.Type(ctx)
		require.NoError(t, err)
		require.Equal(t, NetworkTypeOpen, secType)

		// No resolver: refs carry Path, with Name/Address empty. Assert every
		// bundle field so a mis-mapped field cannot pass unnoticed.
		props, err := n.Properties(ctx)
		require.NoError(t, err)
		require.Equal(t, "OpenNet", props.Name)
		require.True(t, props.Connected)
		require.Equal(t, NetworkTypeOpen, props.Type)
		require.Equal(t, DeviceRef{Path: "/net/connman/iwd/phy0/wlan0"}, props.Device)
		require.NotNil(t, props.KnownNetwork)
		require.Equal(t, kn, *props.KnownNetwork)
		require.Equal(t, []BasicServiceSetRef{{Path: "/net/connman/iwd/phy0/wlan0/aabbccddeeff"}}, props.ExtendedServiceSet)
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

	t.Run("NewNetwork_NilCore", func(t *testing.T) {
		t.Parallel()
		require.Nil(t, newNetwork(nil, "/ignored"))
	})

	t.Run("TypeString", func(t *testing.T) {
		t.Parallel()
		require.Equal(t, "open", NetworkTypeOpen.String())
		require.Equal(t, "psk", NetworkTypePSK.String())
	})

	// Read accessors: success returns the backend value, a nil receiver maps to an
	// internal error, and a backend failure maps to a public network error.
	fullBackend := func() *fakeCoreNetwork {
		kn := "/net/connman/iwd/known/OpenNet"
		return (&fakeCoreNetwork{}).setProps(core.NetworkProperties{
			Name:               "OpenNet",
			Connected:          true,
			Device:             "/net/connman/iwd/phy0/wlan0",
			Type:               core.NetworkTypeOpen,
			KnownNetwork:       &kn,
			ExtendedServiceSet: []string{"/net/connman/iwd/phy0/wlan0/aabbccddeeff"},
		})
	}

	reads := []struct {
		name string
		op   func(n *Network) (any, error)
	}{
		{name: "Name", op: func(n *Network) (any, error) { return n.Name(ctx) }},
		{name: "Connected", op: func(n *Network) (any, error) { return n.Connected(ctx) }},
		{name: "Device", op: func(n *Network) (any, error) { return n.Device(ctx) }},
		{name: "KnownNetwork", op: func(n *Network) (any, error) { return n.KnownNetwork(ctx) }},
		{name: "ExtendedServiceSet", op: func(n *Network) (any, error) { return n.ExtendedServiceSet(ctx) }},
		{name: "Type", op: func(n *Network) (any, error) { return n.Type(ctx) }},
		{name: "Properties", op: func(n *Network) (any, error) { return n.Properties(ctx) }},
	}

	for _, r := range reads {
		t.Run(r.name, func(t *testing.T) {
			t.Parallel()

			t.Run("Success", func(t *testing.T) {
				_, err := r.op(newNetwork(fullBackend(), "/path"))
				require.NoError(t, err)
			})

			t.Run("NilReceiver", func(t *testing.T) {
				_, err := r.op((*Network)(nil))
				require.Error(t, err)
				require.ErrorIs(t, err, ErrInternal)
			})

			t.Run("BackendError", func(t *testing.T) {
				f := fullBackend().setErr(core.WrapNetworkUnavailable("op", "boom", errors.New("x")))
				_, err := r.op(newNetwork(f, "/path"))
				require.Error(t, err)

				var pe *Error
				require.ErrorAs(t, err, &pe)
				require.Equal(t, ResourceNetwork, pe.Resource)
			})
		})
	}

	t.Run("Values", func(t *testing.T) {
		t.Parallel()
		n := newNetwork(fullBackend(), "/path")

		connected, err := n.Connected(ctx)
		require.NoError(t, err)
		require.True(t, connected)

		dev, err := n.Device(ctx)
		require.NoError(t, err)
		require.Equal(t, "/net/connman/iwd/phy0/wlan0", dev)

		kn, err := n.KnownNetwork(ctx)
		require.NoError(t, err)
		require.NotNil(t, kn)
		require.Equal(t, "/net/connman/iwd/known/OpenNet", *kn)

		ess, err := n.ExtendedServiceSet(ctx)
		require.NoError(t, err)
		require.Equal(t, []string{"/net/connman/iwd/phy0/wlan0/aabbccddeeff"}, ess)
	})

	t.Run("TypeInvalidRejectedAtBoundary", func(t *testing.T) {
		t.Parallel()
		f := (&fakeCoreNetwork{}).setProps(core.NetworkProperties{Type: core.NetworkType("garbage")})
		_, err := newNetwork(f, "/path").Type(ctx)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrInvalidArgument)

		var pe *Error
		require.ErrorAs(t, err, &pe)
		require.Equal(t, ResourceNetwork, pe.Resource)
	})

	t.Run("PropertiesInvalidTypeRejected", func(t *testing.T) {
		t.Parallel()
		f := (&fakeCoreNetwork{}).setProps(core.NetworkProperties{Name: "X", Type: core.NetworkType("garbage")})
		_, err := newNetwork(f, "/path").Properties(ctx)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrInvalidArgument)
	})

	t.Run("PropertiesResolverErrorPropagates", func(t *testing.T) {
		t.Parallel()
		f := (&fakeCoreNetwork{}).setProps(validCoreNetworkProps())
		n := newNetwork(f, "/path").withResolver(fakeResolver{err: errors.New("tree boom")})
		_, err := n.Properties(ctx)
		require.Error(t, err)
		require.Contains(t, err.Error(), "tree boom")
	})

	t.Run("PropertiesResolvesRefs", func(t *testing.T) {
		t.Parallel()
		p := validCoreNetworkProps() // Device "/net/connman/iwd/phy0/wlan0", one ESS entry
		f := (&fakeCoreNetwork{}).setProps(p)
		tree := fakeTree{
			p.Device:                p.Device + "-name",
			p.ExtendedServiceSet[0]: "de:ad:be:ef:ca:fe",
		}
		n := newNetwork(f, "/path").withResolver(fakeResolver{tree: tree})
		props, err := n.Properties(ctx)
		require.NoError(t, err)
		require.Equal(t, DeviceRef{Path: p.Device, Name: p.Device + "-name"}, props.Device)
		require.Equal(t, []BasicServiceSetRef{{Path: p.ExtendedServiceSet[0], Address: "de:ad:be:ef:ca:fe"}}, props.ExtendedServiceSet)
	})

	t.Run("SubscribePropertiesChanged", func(t *testing.T) {
		t.Parallel()

		t.Run("NilCallback", func(t *testing.T) {
			_, err := newTestNetwork(t).SubscribePropertiesChanged(ctx, nil)
			require.Error(t, err)
			var pe *Error
			require.ErrorAs(t, err, &pe)
			require.Equal(t, KindInvalidArgument, pe.Kind)
			require.Equal(t, ResourceNetwork, pe.Resource)
		})

		t.Run("NilReceiver", func(t *testing.T) {
			_, err := (*Network)(nil).SubscribePropertiesChanged(ctx, func(NetworkPropertiesChanged) {})
			require.Error(t, err)
			require.ErrorIs(t, err, ErrInternal)
		})

		t.Run("DeliversEvent", func(t *testing.T) {
			f := (&fakeCoreNetwork{}).setProps(validCoreNetworkProps()).setConnectedEvent(true)
			var got NetworkPropertiesChanged
			_, err := newNetwork(f, "/path").SubscribePropertiesChanged(ctx, func(ev NetworkPropertiesChanged) { got = ev })
			require.NoError(t, err)
			require.Equal(t, true, got.Changed["Connected"])
			require.Equal(t, []string{"KnownNetwork"}, got.Invalidated)
		})

		t.Run("BackendError", func(t *testing.T) {
			f := (&fakeCoreNetwork{}).setProps(validCoreNetworkProps()).setErr(core.WrapNetworkUnavailable("op", "boom", errors.New("x")))
			_, err := newNetwork(f, "/path").SubscribePropertiesChanged(ctx, func(NetworkPropertiesChanged) {})
			require.Error(t, err)
			var pe *Error
			require.ErrorAs(t, err, &pe)
			require.Equal(t, ResourceNetwork, pe.Resource)
		})
	})

	t.Run("SubscribeConnectedChanged", func(t *testing.T) {
		t.Parallel()

		t.Run("NilCallback", func(t *testing.T) {
			_, err := newTestNetwork(t).SubscribeConnectedChanged(ctx, nil)
			require.Error(t, err)
			var pe *Error
			require.ErrorAs(t, err, &pe)
			require.Equal(t, KindInvalidArgument, pe.Kind)
		})

		t.Run("NilReceiver", func(t *testing.T) {
			_, err := (*Network)(nil).SubscribeConnectedChanged(ctx, func(bool) {})
			require.Error(t, err)
			require.ErrorIs(t, err, ErrInternal)
		})

		t.Run("DeliversEvent", func(t *testing.T) {
			f := (&fakeCoreNetwork{}).setProps(validCoreNetworkProps()).setConnectedEvent(true)
			got, fired := false, false
			_, err := newNetwork(f, "/path").SubscribeConnectedChanged(ctx, func(b bool) {
				got = b
				fired = true
			})
			require.NoError(t, err)
			require.True(t, fired)
			require.True(t, got)
		})

		t.Run("BackendError", func(t *testing.T) {
			f := (&fakeCoreNetwork{}).setProps(validCoreNetworkProps()).setErr(core.WrapNetworkUnavailable("op", "boom", errors.New("x")))
			_, err := newNetwork(f, "/path").SubscribeConnectedChanged(ctx, func(bool) {})
			require.Error(t, err)
			var pe *Error
			require.ErrorAs(t, err, &pe)
			require.Equal(t, ResourceNetwork, pe.Resource)
		})
	})
}

func TestClient_AllNetworks(t *testing.T) {
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
