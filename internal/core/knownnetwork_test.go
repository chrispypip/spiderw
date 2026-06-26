//go:build unit

package core

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/chrispypip/spiderw/internal/iwdbus"
)

func TestKnownNetwork_Core(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("Properties", func(t *testing.T) {
		t.Parallel()
		k := NewKnownNetwork((&fakeIwdbusKnownNetwork{}).setProps(validKnownNetworkProps()))
		require.NotNil(t, k)

		props, err := k.Properties(ctx)
		require.NoError(t, err)
		require.Equal(t, "HomeNet", props.Name)
		require.Equal(t, NetworkTypePSK, props.Type)
		require.False(t, props.Hidden)
		require.NotNil(t, props.LastConnectedTime)
		require.True(t, props.AutoConnect)
	})

	t.Run("EmptyNameIsInvalidState", func(t *testing.T) {
		t.Parallel()
		p := validKnownNetworkProps()
		p.Name = "   "
		k := NewKnownNetwork((&fakeIwdbusKnownNetwork{}).setProps(p))
		_, err := k.Name(ctx)
		require.Error(t, err)
		require.Contains(t, err.Error(), "empty Name")
	})

	t.Run("InvalidTypeIsInvalidState", func(t *testing.T) {
		t.Parallel()
		p := validKnownNetworkProps()
		p.Type = iwdbus.NetworkType("wpa3")
		k := NewKnownNetwork((&fakeIwdbusKnownNetwork{}).setProps(p))
		_, err := k.Type(ctx)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unknown type")

		var ce *Error
		require.ErrorAs(t, err, &ce)
		require.Equal(t, ResourceKnownNetwork, ce.Resource)
	})

	t.Run("Hotspot", func(t *testing.T) {
		t.Parallel()
		p := validKnownNetworkProps()
		p.Type = iwdbus.NetworkTypeHotspot
		k := NewKnownNetwork((&fakeIwdbusKnownNetwork{}).setProps(p))
		secType, err := k.Type(ctx)
		require.NoError(t, err)
		require.Equal(t, NetworkTypeHotspot, secType)
	})

	t.Run("LastConnectedTimeNilNormalization", func(t *testing.T) {
		t.Parallel()
		p := validKnownNetworkProps()
		blank := "   "
		p.LastConnectedTime = &blank
		k := NewKnownNetwork((&fakeIwdbusKnownNetwork{}).setProps(p))
		lt, err := k.LastConnectedTime(ctx)
		require.NoError(t, err)
		require.Nil(t, lt)
	})

	t.Run("SetAutoConnect", func(t *testing.T) {
		t.Parallel()
		k := NewKnownNetwork((&fakeIwdbusKnownNetwork{}).setProps(validKnownNetworkProps()))
		require.NoError(t, k.SetAutoConnect(ctx, false))

		auto, err := k.AutoConnect(ctx)
		require.NoError(t, err)
		require.False(t, auto)
	})

	t.Run("ForgetSuccess", func(t *testing.T) {
		t.Parallel()
		k := NewKnownNetwork((&fakeIwdbusKnownNetwork{}).setProps(validKnownNetworkProps()))
		require.NoError(t, k.Forget(ctx))
	})

	t.Run("ForgetErrorPreservesSentinel", func(t *testing.T) {
		t.Parallel()
		// The iwdbus layer maps named iwd errors, so a Forget failure remains
		// matchable through the core error chain.
		raw := (&fakeIwdbusKnownNetwork{}).setProps(validKnownNetworkProps()).
			setForgetErr(&iwdbus.Error{Kind: iwdbus.ErrDBusMethod, Context: "Forget", Err: iwdbus.ErrBusy})
		k := NewKnownNetwork(raw)

		err := k.Forget(ctx)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrBusy)

		var ce *Error
		require.ErrorAs(t, err, &ce)
		require.Equal(t, KindUnavailable, ce.Kind)
		require.Equal(t, ResourceKnownNetwork, ce.Resource)
	})

	t.Run("BackendErrorMapsToUnavailable", func(t *testing.T) {
		t.Parallel()
		raw := (&fakeIwdbusKnownNetwork{}).setProps(validKnownNetworkProps()).
			setErr(iwdbus.WrapProperty(iwdbus.IwdKnownNetworkIface, "Name", errors.New("dbus failure")))
		k := NewKnownNetwork(raw)

		_, err := k.Name(ctx)
		require.Error(t, err)

		var ce *Error
		require.ErrorAs(t, err, &ce)
		require.Equal(t, KindUnavailable, ce.Kind)
		require.Equal(t, ResourceKnownNetwork, ce.Resource)
	})

	t.Run("SubscribeAutoConnectChangedDelivers", func(t *testing.T) {
		t.Parallel()
		f := (&fakeIwdbusKnownNetwork{}).setProps(validKnownNetworkProps()).setAutoConnectEvent(false)
		k := NewKnownNetwork(f)

		got := make(chan bool, 1)
		_, err := k.SubscribeAutoConnectChanged(ctx, func(b bool) { got <- b })
		require.NoError(t, err)
		require.False(t, <-got)
	})

	t.Run("SubscribeNilCallbackIsInvalidArgument", func(t *testing.T) {
		t.Parallel()
		k := newTestKnownNetwork(t)
		_, err := k.SubscribeAutoConnectChanged(ctx, nil)
		require.Error(t, err)
		var ce *Error
		require.ErrorAs(t, err, &ce)
		require.Equal(t, KindInvalidArgument, ce.Kind)
	})

	t.Run("NilReceiver", func(t *testing.T) {
		t.Parallel()
		var k *KnownNetwork
		err := k.Forget(ctx)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrKnownNetworkNotInitialized)
	})

	t.Run("NewKnownNetwork_NilRaw", func(t *testing.T) {
		t.Parallel()
		require.Nil(t, NewKnownNetwork(nil))
	})
}
