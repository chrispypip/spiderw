//go:build unit

package core

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/chrispypip/spiderw/internal/iwdbus"
)

func TestNetwork_Core(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("Properties", func(t *testing.T) {
		t.Parallel()
		p := validNetworkProps()
		p.Connected = true // distinct from zero so a mismapped field is caught
		n := NewNetwork((&fakeIwdbusNetwork{}).setProps(p))
		require.NotNil(t, n)

		props, err := n.Properties(ctx)
		require.NoError(t, err)
		require.Equal(t, "OpenNet", props.Name)
		require.True(t, props.Connected)
		require.Equal(t, "/net/connman/iwd/phy0/wlan0", props.Device)
		require.Equal(t, NetworkTypeOpen, props.Type)
		require.NotNil(t, props.KnownNetwork)
		require.Equal(t, []string{"/net/connman/iwd/phy0/wlan0/aabbccddeeff"}, props.ExtendedServiceSet)
	})

	t.Run("NameSuccess", func(t *testing.T) {
		t.Parallel()
		n := NewNetwork((&fakeIwdbusNetwork{}).setProps(validNetworkProps()))
		name, err := n.Name(ctx)
		require.NoError(t, err)
		require.Equal(t, "OpenNet", name)
	})

	t.Run("EmptyNameIsInvalidState", func(t *testing.T) {
		t.Parallel()
		p := validNetworkProps()
		p.Name = "   "
		n := NewNetwork((&fakeIwdbusNetwork{}).setProps(p))
		_, err := n.Name(ctx)
		require.Error(t, err)
		require.Contains(t, err.Error(), "empty Name")
	})

	t.Run("EmptyDeviceIsInvalidState", func(t *testing.T) {
		t.Parallel()
		p := validNetworkProps()
		p.Device = "   "
		n := NewNetwork((&fakeIwdbusNetwork{}).setProps(p))
		_, err := n.Device(ctx)
		require.Error(t, err)
		require.Contains(t, err.Error(), "empty Device")
	})

	// Properties applies the same normalization as the per-property getters, so
	// each invalid backend field surfaces as an invalid-state error from the
	// single bundled read.
	t.Run("PropertiesInvalidState", func(t *testing.T) {
		t.Parallel()
		for _, tc := range []struct {
			name    string
			mutate  func(*iwdbus.NetworkProperties)
			wantSub string
		}{
			{"EmptyName", func(p *iwdbus.NetworkProperties) { p.Name = "  " }, "empty Name"},
			{"EmptyDevice", func(p *iwdbus.NetworkProperties) { p.Device = "  " }, "empty Device"},
			{"InvalidType", func(p *iwdbus.NetworkProperties) { p.Type = iwdbus.NetworkType("wpa3") }, "unknown type"},
			{"EmptyESSEntry", func(p *iwdbus.NetworkProperties) { p.ExtendedServiceSet = []string{"  "} }, "empty basic service set path"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				p := validNetworkProps()
				tc.mutate(&p)
				n := NewNetwork((&fakeIwdbusNetwork{}).setProps(p))
				_, err := n.Properties(ctx)
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.wantSub)
			})
		}
	})

	t.Run("InvalidTypeIsInvalidState", func(t *testing.T) {
		t.Parallel()
		p := validNetworkProps()
		p.Type = iwdbus.NetworkType("wpa3")
		n := NewNetwork((&fakeIwdbusNetwork{}).setProps(p))
		_, err := n.Type(ctx)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unknown type")
	})

	t.Run("KnownNetworkNilPassthrough", func(t *testing.T) {
		t.Parallel()
		p := validNetworkProps()
		p.KnownNetwork = nil
		n := NewNetwork((&fakeIwdbusNetwork{}).setProps(p))
		known, err := n.KnownNetwork(ctx)
		require.NoError(t, err)
		require.Nil(t, known)
	})

	t.Run("EmptyESSEntryIsInvalidState", func(t *testing.T) {
		t.Parallel()
		p := validNetworkProps()
		p.ExtendedServiceSet = []string{"  "}
		n := NewNetwork((&fakeIwdbusNetwork{}).setProps(p))
		_, err := n.ExtendedServiceSet(ctx)
		require.Error(t, err)
		require.Contains(t, err.Error(), "empty basic service set path")
	})

	t.Run("ConnectSuccess", func(t *testing.T) {
		t.Parallel()
		n := NewNetwork((&fakeIwdbusNetwork{}).setProps(validNetworkProps()))
		require.NoError(t, n.Connect(ctx))
	})

	t.Run("ConnectNoAgentPreservesSentinel", func(t *testing.T) {
		t.Parallel()
		// The iwdbus layer wraps NoAgent so errors.Is(err, ErrNoAgent) holds.
		raw := (&fakeIwdbusNetwork{}).setProps(validNetworkProps()).
			setConnectErr(&iwdbus.Error{Kind: iwdbus.ErrDBusMethod, Context: "Connect", Err: iwdbus.ErrNoAgent})
		n := NewNetwork(raw)

		err := n.Connect(ctx)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrNoAgent)

		var ce *Error
		require.ErrorAs(t, err, &ce)
		require.Equal(t, KindUnavailable, ce.Kind)
		require.Equal(t, ResourceNetwork, ce.Resource)
		require.Contains(t, ce.Details, "no credentials agent")
	})

	t.Run("ConnectGenericErrorWraps", func(t *testing.T) {
		t.Parallel()
		// A non-agent Connect failure wraps into a matchable core Error rather
		// than taking the NoAgent branch.
		raw := (&fakeIwdbusNetwork{}).setProps(validNetworkProps()).setConnectErr(errors.New("dbus boom"))
		err := NewNetwork(raw).Connect(ctx)
		require.Error(t, err)
		var ce *Error
		require.ErrorAs(t, err, &ce)
		require.Equal(t, ResourceNetwork, ce.Resource)
		require.NotContains(t, ce.Details, "no credentials agent")
	})

	t.Run("NilReceiver", func(t *testing.T) {
		t.Parallel()
		var n *Network
		for _, tc := range []struct {
			name string
			call func() error
		}{
			{"Name", func() error { _, err := n.Name(ctx); return err }},
			{"Connected", func() error { _, err := n.Connected(ctx); return err }},
			{"Device", func() error { _, err := n.Device(ctx); return err }},
			{"Type", func() error { _, err := n.Type(ctx); return err }},
			{"KnownNetwork", func() error { _, err := n.KnownNetwork(ctx); return err }},
			{"ExtendedServiceSet", func() error { _, err := n.ExtendedServiceSet(ctx); return err }},
			{"Properties", func() error { _, err := n.Properties(ctx); return err }},
			{"Connect", func() error { return n.Connect(ctx) }},
			{"SubscribeConnectedChanged", func() error {
				_, err := n.SubscribeConnectedChanged(ctx, func(bool) {})
				return err
			}},
			{"SubscribePropertiesChanged", func() error {
				_, err := n.SubscribePropertiesChanged(ctx, func(NetworkPropertiesChanged) {})
				return err
			}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				err := tc.call()
				require.ErrorIs(t, err, ErrNetworkNotInitialized)
				require.ErrorIs(t, err, ErrCore)
			})
		}
	})

	t.Run("NewNetwork_NilRaw", func(t *testing.T) {
		t.Parallel()
		require.Nil(t, NewNetwork(nil))
	})

	t.Run("SubscribeNilCallbackIsInvalidArgument", func(t *testing.T) {
		t.Parallel()
		n := newTestNetwork(t)

		_, err := n.SubscribeConnectedChanged(ctx, nil)
		require.Error(t, err)
		var ce *Error
		require.ErrorAs(t, err, &ce)
		require.Equal(t, KindInvalidArgument, ce.Kind)

		_, err = n.SubscribePropertiesChanged(ctx, nil)
		require.Error(t, err)
		require.ErrorAs(t, err, &ce)
		require.Equal(t, KindInvalidArgument, ce.Kind)
	})

	t.Run("SubscribeConnectedChangedDelivers", func(t *testing.T) {
		t.Parallel()
		f := (&fakeIwdbusNetwork{}).setProps(validNetworkProps()).setConnectedEvent(true)
		n := NewNetwork(f)

		got := make(chan bool, 1)
		_, err := n.SubscribeConnectedChanged(ctx, func(b bool) { got <- b })
		require.NoError(t, err)
		require.True(t, <-got)
	})

	t.Run("SubscribePropertiesChangedNormalizes", func(t *testing.T) {
		t.Parallel()
		// The fake fires a Changed map carrying a dbus.Variant; the core callback
		// must unwrap it to a plain value before delivering.
		f := (&fakeIwdbusNetwork{}).setProps(validNetworkProps()).setConnectedEvent(true)
		n := NewNetwork(f)

		got := make(chan NetworkPropertiesChanged, 1)
		_, err := n.SubscribePropertiesChanged(ctx, func(ev NetworkPropertiesChanged) { got <- ev })
		require.NoError(t, err)
		ev := <-got
		require.Equal(t, true, ev.Changed["Connected"])
		require.Equal(t, []string{"KnownNetwork"}, ev.Invalidated)
	})

	t.Run("Connected", func(t *testing.T) {
		t.Parallel()
		p := validNetworkProps()
		p.Connected = true
		got, err := NewNetwork((&fakeIwdbusNetwork{}).setProps(p)).Connected(ctx)
		require.NoError(t, err)
		require.True(t, got)
	})

	t.Run("Device", func(t *testing.T) {
		t.Parallel()
		got, err := NewNetwork((&fakeIwdbusNetwork{}).setProps(validNetworkProps())).Device(ctx)
		require.NoError(t, err)
		require.Equal(t, "/net/connman/iwd/phy0/wlan0", got)
	})

	// Every getter wraps a backend failure into a matchable core Error (right
	// Resource, cause chained through ErrCore) rather than leaking the raw error.
	t.Run("BackendErrorWraps", func(t *testing.T) {
		t.Parallel()
		backendErr := errors.New("dbus boom")
		for _, tc := range []struct {
			name string
			call func(*Network) error
		}{
			{"Name", func(n *Network) error { _, err := n.Name(ctx); return err }},
			{"Connected", func(n *Network) error { _, err := n.Connected(ctx); return err }},
			{"Device", func(n *Network) error { _, err := n.Device(ctx); return err }},
			{"Type", func(n *Network) error { _, err := n.Type(ctx); return err }},
			{"KnownNetwork", func(n *Network) error { _, err := n.KnownNetwork(ctx); return err }},
			{"ExtendedServiceSet", func(n *Network) error { _, err := n.ExtendedServiceSet(ctx); return err }},
			{"Properties", func(n *Network) error { _, err := n.Properties(ctx); return err }},
			{"SubscribePropertiesChanged", func(n *Network) error {
				_, err := n.SubscribePropertiesChanged(ctx, func(NetworkPropertiesChanged) {})
				return err
			}},
			{"SubscribeConnectedChanged", func(n *Network) error {
				_, err := n.SubscribeConnectedChanged(ctx, func(bool) {})
				return err
			}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				n := NewNetwork((&fakeIwdbusNetwork{}).setProps(validNetworkProps()).setErr(backendErr))
				err := tc.call(n)
				require.Error(t, err)
				var ce *Error
				require.ErrorAs(t, err, &ce)
				require.Equal(t, ResourceNetwork, ce.Resource)
				require.ErrorIs(t, err, backendErr)
				require.ErrorIs(t, err, ErrCore)
			})
		}
	})
}
