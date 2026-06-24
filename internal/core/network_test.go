//go:build unit

package core

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/chrispypip/spiderw/internal/iwdbus"
)

func TestNetwork_Core(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("Properties", func(t *testing.T) {
		t.Parallel()
		n := NewNetwork((&fakeIwdbusNetwork{}).setProps(validNetworkProps()))
		require.NotNil(t, n)

		props, err := n.Properties(ctx)
		require.NoError(t, err)
		require.Equal(t, "OpenNet", props.Name)
		require.Equal(t, "/net/connman/iwd/phy0/wlan0", props.Device)
		require.Equal(t, SecurityTypeOpen, props.Type)
		require.NotNil(t, props.KnownNetwork)
		require.Equal(t, []string{"/net/connman/iwd/phy0/wlan0/aabbccddeeff"}, props.ExtendedServiceSet)
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

	t.Run("InvalidTypeIsInvalidState", func(t *testing.T) {
		t.Parallel()
		p := validNetworkProps()
		p.Type = iwdbus.SecurityType("wpa3")
		n := NewNetwork((&fakeIwdbusNetwork{}).setProps(p))
		_, err := n.Type(ctx)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unknown security type")
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

	t.Run("NilReceiver", func(t *testing.T) {
		t.Parallel()
		var n *Network
		err := n.Connect(ctx)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrNetworkNotInitialized)
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
}
