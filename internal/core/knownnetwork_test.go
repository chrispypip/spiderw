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

	t.Run("PropertiesEmptyNameIsInvalidState", func(t *testing.T) {
		t.Parallel()
		p := validKnownNetworkProps()
		p.Name = "   "
		k := NewKnownNetwork((&fakeIwdbusKnownNetwork{}).setProps(p))
		_, err := k.Properties(ctx)
		require.Error(t, err)
		require.Contains(t, err.Error(), "empty Name")
	})

	t.Run("PropertiesInvalidTypeIsInvalidState", func(t *testing.T) {
		t.Parallel()
		p := validKnownNetworkProps()
		p.Type = iwdbus.NetworkType("wpa3")
		k := NewKnownNetwork((&fakeIwdbusKnownNetwork{}).setProps(p))
		_, err := k.Properties(ctx)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unknown type")
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

	t.Run("NameSuccess", func(t *testing.T) {
		t.Parallel()
		k := NewKnownNetwork((&fakeIwdbusKnownNetwork{}).setProps(validKnownNetworkProps()))
		name, err := k.Name(ctx)
		require.NoError(t, err)
		require.Equal(t, "HomeNet", name)
	})

	t.Run("Hidden", func(t *testing.T) {
		t.Parallel()
		p := validKnownNetworkProps()
		p.Hidden = true
		k := NewKnownNetwork((&fakeIwdbusKnownNetwork{}).setProps(p))
		hidden, err := k.Hidden(ctx)
		require.NoError(t, err)
		require.True(t, hidden)
	})

	t.Run("SubscribePropertiesChangedNormalizes", func(t *testing.T) {
		t.Parallel()
		// The fake fires a Changed map carrying a dbus.Variant plus an
		// Invalidated list; the core callback must unwrap the variant and clone
		// the invalidated slice before delivering.
		f := (&fakeIwdbusKnownNetwork{}).setProps(validKnownNetworkProps()).setAutoConnectEvent(false)
		k := NewKnownNetwork(f)

		got := make(chan KnownNetworkPropertiesChanged, 1)
		_, err := k.SubscribePropertiesChanged(ctx, func(ev KnownNetworkPropertiesChanged) { got <- ev })
		require.NoError(t, err)
		ev := <-got
		require.Equal(t, false, ev.Changed["AutoConnect"])
		require.Equal(t, []string{"LastConnectedTime"}, ev.Invalidated)
	})

	t.Run("SubscribeNilCallbackIsInvalidArgument", func(t *testing.T) {
		t.Parallel()
		k := newTestKnownNetwork(t)

		_, err := k.SubscribeAutoConnectChanged(ctx, nil)
		require.Error(t, err)
		var ce *Error
		require.ErrorAs(t, err, &ce)
		require.Equal(t, KindInvalidArgument, ce.Kind)

		_, err = k.SubscribePropertiesChanged(ctx, nil)
		require.Error(t, err)
		require.ErrorAs(t, err, &ce)
		require.Equal(t, KindInvalidArgument, ce.Kind)
	})

	// Every getter, setter, and subscribe wraps a backend failure into a
	// matchable core Error (right Resource, cause chained through ErrCore).
	t.Run("BackendErrorWraps", func(t *testing.T) {
		t.Parallel()
		backendErr := errors.New("dbus boom")
		for _, tc := range []struct {
			name string
			call func(*KnownNetwork) error
		}{
			{"Name", func(k *KnownNetwork) error { _, err := k.Name(ctx); return err }},
			{"Type", func(k *KnownNetwork) error { _, err := k.Type(ctx); return err }},
			{"Hidden", func(k *KnownNetwork) error { _, err := k.Hidden(ctx); return err }},
			{"LastConnectedTime", func(k *KnownNetwork) error { _, err := k.LastConnectedTime(ctx); return err }},
			{"AutoConnect", func(k *KnownNetwork) error { _, err := k.AutoConnect(ctx); return err }},
			{"SetAutoConnect", func(k *KnownNetwork) error { return k.SetAutoConnect(ctx, true) }},
			{"Properties", func(k *KnownNetwork) error { _, err := k.Properties(ctx); return err }},
			{"SubscribePropertiesChanged", func(k *KnownNetwork) error {
				_, err := k.SubscribePropertiesChanged(ctx, func(KnownNetworkPropertiesChanged) {})
				return err
			}},
			{"SubscribeAutoConnectChanged", func(k *KnownNetwork) error {
				_, err := k.SubscribeAutoConnectChanged(ctx, func(bool) {})
				return err
			}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				k := NewKnownNetwork((&fakeIwdbusKnownNetwork{}).setProps(validKnownNetworkProps()).setErr(backendErr))
				err := tc.call(k)
				require.Error(t, err)
				var ce *Error
				require.ErrorAs(t, err, &ce)
				require.Equal(t, ResourceKnownNetwork, ce.Resource)
				require.ErrorIs(t, err, backendErr)
				require.ErrorIs(t, err, ErrCore)
			})
		}
	})

	t.Run("NilReceiver", func(t *testing.T) {
		t.Parallel()
		var k *KnownNetwork
		for _, tc := range []struct {
			name string
			call func() error
		}{
			{"Name", func() error { _, err := k.Name(ctx); return err }},
			{"Type", func() error { _, err := k.Type(ctx); return err }},
			{"Hidden", func() error { _, err := k.Hidden(ctx); return err }},
			{"LastConnectedTime", func() error { _, err := k.LastConnectedTime(ctx); return err }},
			{"AutoConnect", func() error { _, err := k.AutoConnect(ctx); return err }},
			{"SetAutoConnect", func() error { return k.SetAutoConnect(ctx, true) }},
			{"Forget", func() error { return k.Forget(ctx) }},
			{"Properties", func() error { _, err := k.Properties(ctx); return err }},
			{"SubscribePropertiesChanged", func() error {
				_, err := k.SubscribePropertiesChanged(ctx, func(KnownNetworkPropertiesChanged) {})
				return err
			}},
			{"SubscribeAutoConnectChanged", func() error {
				_, err := k.SubscribeAutoConnectChanged(ctx, func(bool) {})
				return err
			}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				err := tc.call()
				require.ErrorIs(t, err, ErrKnownNetworkNotInitialized)
				require.ErrorIs(t, err, ErrCore)
			})
		}
	})

	t.Run("NewKnownNetwork_NilRaw", func(t *testing.T) {
		t.Parallel()
		require.Nil(t, NewKnownNetwork(nil))
	})
}
