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

func TestKnownNetwork_Public(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("PropertiesAndType", func(t *testing.T) {
		t.Parallel()

		p := validCoreKnownNetworkProps()
		p.Hidden = true // distinct from zero so a mis-mapped field is caught
		k := newKnownNetwork((&fakeCoreKnownNetwork{}).setProps(p), "/net/connman/iwd/abc")
		require.NotNil(t, k)
		require.Equal(t, "/net/connman/iwd/abc", k.Path())

		secType, err := k.Type(ctx)
		require.NoError(t, err)
		require.Equal(t, NetworkTypePSK, secType)

		// Assert every bundle field so a mis-mapped field cannot pass unnoticed.
		props, err := k.Properties(ctx)
		require.NoError(t, err)
		require.Equal(t, "HomeNet", props.Name)
		require.Equal(t, NetworkTypePSK, props.Type)
		require.True(t, props.Hidden)
		require.NotNil(t, props.LastConnectedTime)
		require.Equal(t, "2024-01-02T03:04:05Z", *props.LastConnectedTime)
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

	// Read accessors: success returns the backend value, a nil receiver maps to an
	// internal error, and a backend failure maps to a public known-network error.
	reads := []struct {
		name string
		op   func(k *KnownNetwork) (any, error)
	}{
		{name: "Name", op: func(k *KnownNetwork) (any, error) { return k.Name(ctx) }},
		{name: "Type", op: func(k *KnownNetwork) (any, error) { return k.Type(ctx) }},
		{name: "Hidden", op: func(k *KnownNetwork) (any, error) { return k.Hidden(ctx) }},
		{name: "LastConnectedTime", op: func(k *KnownNetwork) (any, error) { return k.LastConnectedTime(ctx) }},
		{name: "AutoConnect", op: func(k *KnownNetwork) (any, error) { return k.AutoConnect(ctx) }},
		{name: "Properties", op: func(k *KnownNetwork) (any, error) { return k.Properties(ctx) }},
	}

	for _, r := range reads {
		t.Run(r.name, func(t *testing.T) {
			t.Parallel()

			t.Run("Success", func(t *testing.T) {
				k := newKnownNetwork((&fakeCoreKnownNetwork{}).setProps(validCoreKnownNetworkProps()), "/path")
				_, err := r.op(k)
				require.NoError(t, err)
			})

			t.Run("NilReceiver", func(t *testing.T) {
				_, err := r.op((*KnownNetwork)(nil))
				require.Error(t, err)
				require.ErrorIs(t, err, ErrInternal)
			})

			t.Run("BackendError", func(t *testing.T) {
				f := (&fakeCoreKnownNetwork{}).setProps(validCoreKnownNetworkProps()).
					setErr(core.WrapKnownNetworkUnavailable("op", "boom", errors.New("x")))
				_, err := r.op(newKnownNetwork(f, "/path"))
				require.Error(t, err)

				var pe *Error
				require.ErrorAs(t, err, &pe)
				require.Equal(t, ResourceKnownNetwork, pe.Resource)
			})
		})
	}

	t.Run("Values", func(t *testing.T) {
		t.Parallel()
		k := newKnownNetwork((&fakeCoreKnownNetwork{}).setProps(validCoreKnownNetworkProps()), "/path")

		hidden, err := k.Hidden(ctx)
		require.NoError(t, err)
		require.False(t, hidden)

		lct, err := k.LastConnectedTime(ctx)
		require.NoError(t, err)
		require.NotNil(t, lct)
		require.Equal(t, "2024-01-02T03:04:05Z", *lct)

		name, err := k.Name(ctx)
		require.NoError(t, err)
		require.Equal(t, "HomeNet", name)
	})

	t.Run("TypeInvalidRejectedAtBoundary", func(t *testing.T) {
		t.Parallel()
		f := (&fakeCoreKnownNetwork{}).setProps(core.KnownNetworkProperties{Type: core.NetworkType("garbage")})
		_, err := newKnownNetwork(f, "/path").Type(ctx)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrInvalidArgument)
	})

	t.Run("PropertiesInvalidTypeRejected", func(t *testing.T) {
		t.Parallel()
		f := (&fakeCoreKnownNetwork{}).setProps(core.KnownNetworkProperties{Name: "X", Type: core.NetworkType("garbage")})
		_, err := newKnownNetwork(f, "/path").Properties(ctx)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrInvalidArgument)
	})

	t.Run("SubscribePropertiesChanged", func(t *testing.T) {
		t.Parallel()

		t.Run("NilCallback", func(t *testing.T) {
			_, err := newTestKnownNetwork(t).SubscribePropertiesChanged(ctx, nil)
			require.Error(t, err)
			var pe *Error
			require.ErrorAs(t, err, &pe)
			require.Equal(t, KindInvalidArgument, pe.Kind)
			require.Equal(t, ResourceKnownNetwork, pe.Resource)
		})

		t.Run("NilReceiver", func(t *testing.T) {
			_, err := (*KnownNetwork)(nil).SubscribePropertiesChanged(ctx, func(KnownNetworkPropertiesChanged) {})
			require.Error(t, err)
			require.ErrorIs(t, err, ErrInternal)
		})

		t.Run("DeliversEvent", func(t *testing.T) {
			f := (&fakeCoreKnownNetwork{}).setProps(validCoreKnownNetworkProps()).setAutoConnectEvent(false)
			var got KnownNetworkPropertiesChanged
			_, err := newKnownNetwork(f, "/path").SubscribePropertiesChanged(ctx, func(ev KnownNetworkPropertiesChanged) { got = ev })
			require.NoError(t, err)
			require.Equal(t, false, got.Changed["AutoConnect"])
			require.Equal(t, []string{"LastConnectedTime"}, got.Invalidated)
		})

		t.Run("BackendError", func(t *testing.T) {
			f := (&fakeCoreKnownNetwork{}).setProps(validCoreKnownNetworkProps()).
				setErr(core.WrapKnownNetworkUnavailable("op", "boom", errors.New("x")))
			_, err := newKnownNetwork(f, "/path").SubscribePropertiesChanged(ctx, func(KnownNetworkPropertiesChanged) {})
			require.Error(t, err)
			var pe *Error
			require.ErrorAs(t, err, &pe)
			require.Equal(t, ResourceKnownNetwork, pe.Resource)
		})
	})

	t.Run("SubscribeAutoConnectChanged", func(t *testing.T) {
		t.Parallel()

		t.Run("NilCallback", func(t *testing.T) {
			_, err := newTestKnownNetwork(t).SubscribeAutoConnectChanged(ctx, nil)
			require.Error(t, err)
			var pe *Error
			require.ErrorAs(t, err, &pe)
			require.Equal(t, KindInvalidArgument, pe.Kind)
		})

		t.Run("NilReceiver", func(t *testing.T) {
			_, err := (*KnownNetwork)(nil).SubscribeAutoConnectChanged(ctx, func(bool) {})
			require.Error(t, err)
			require.ErrorIs(t, err, ErrInternal)
		})

		t.Run("DeliversEvent", func(t *testing.T) {
			f := (&fakeCoreKnownNetwork{}).setProps(validCoreKnownNetworkProps()).setAutoConnectEvent(true)
			got, fired := false, false
			_, err := newKnownNetwork(f, "/path").SubscribeAutoConnectChanged(ctx, func(b bool) {
				got = b
				fired = true
			})
			require.NoError(t, err)
			require.True(t, fired)
			require.True(t, got)
		})

		t.Run("BackendError", func(t *testing.T) {
			f := (&fakeCoreKnownNetwork{}).setProps(validCoreKnownNetworkProps()).
				setErr(core.WrapKnownNetworkUnavailable("op", "boom", errors.New("x")))
			_, err := newKnownNetwork(f, "/path").SubscribeAutoConnectChanged(ctx, func(bool) {})
			require.Error(t, err)
			var pe *Error
			require.ErrorAs(t, err, &pe)
			require.Equal(t, ResourceKnownNetwork, pe.Resource)
		})
	})
}

func TestClient_AllKnownNetworks(t *testing.T) {
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
