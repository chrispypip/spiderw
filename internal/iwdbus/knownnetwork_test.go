//go:build unit

package iwdbus

import (
	"context"
	"fmt"
	"testing"

	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/require"
)

func TestKnownNetwork_Iwdbus(t *testing.T) {
	t.Parallel()

	t.Run("Getters", func(t *testing.T) {
		t.Parallel()
		t.Run("GetName", testKnownNetwork_GetName)
		t.Run("GetType", testKnownNetwork_GetType)
		t.Run("GetType_Hotspot", testKnownNetwork_GetType_Hotspot)
		t.Run("GetHidden", testKnownNetwork_GetHidden)
		t.Run("GetLastConnectedTime", testKnownNetwork_GetLastConnectedTime)
		t.Run("GetLastConnectedTime_Absent", testKnownNetwork_GetLastConnectedTime_Absent)
		t.Run("GetAutoConnect", testKnownNetwork_GetAutoConnect)
	})

	t.Run("Set", func(t *testing.T) {
		t.Parallel()
		t.Run("SetAutoConnect", testKnownNetwork_SetAutoConnect)
		t.Run("SetAutoConnect_Err", testKnownNetwork_SetAutoConnect_Err)
	})

	t.Run("Forget", func(t *testing.T) {
		t.Parallel()
		t.Run("Success", testKnownNetwork_Forget)
		t.Run("ErrorMapping", testKnownNetwork_Forget_ErrorMapping)
	})

	t.Run("Properties", func(t *testing.T) {
		t.Parallel()
		t.Run("Full", testKnownNetwork_GetProperties)
		t.Run("OptionalLastConnectedTimeAbsent", testKnownNetwork_GetProperties_NoLastConnectedTime)
		t.Run("Errors", testKnownNetwork_GetProperties_Errors)
	})

	t.Run("NotInitialized", testKnownNetwork_NoIntro)

	t.Run("Subscribe", func(t *testing.T) {
		t.Parallel()
		t.Run("AutoConnectChanged", testKnownNetwork_SubscribeAutoConnectChanged)
		t.Run("HiddenChanged", testKnownNetwork_SubscribeHiddenChanged)
		t.Run("LastConnectedTimeChanged", testKnownNetwork_SubscribeLastConnectedTimeChanged)
		t.Run("NewSubscribers_Guards", testKnownNetwork_SubscribeNew_SkipMalformedAndNilCallback)
	})

	t.Run("Firehose", func(t *testing.T) {
		t.Parallel()
		t.Run("NilCallback", testKnownNetwork_Firehose_NilCallback)
		t.Run("ReceivesAll", testKnownNetwork_FirehoseReceivesAll)
	})
}

func testKnownNetwork_Firehose_NilCallback(t *testing.T) {
	t.Parallel()
	k := &KnownNetwork{signals: newFakeSignalSource(t)}
	err := k.Firehose(context.Background(), nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "fn cannot be nil")
}

func testKnownNetwork_FirehoseReceivesAll(t *testing.T) {
	fake := newFakeSignalSource(t)
	known := &KnownNetwork{signals: fake}

	var recv FirehoseSignal
	fired := make(chan struct{}, 1)

	err := known.Firehose(context.Background(), func(s FirehoseSignal) {
		recv = s
		fired <- struct{}{}
	})
	require.NoError(t, err)

	fake.emit(
		IwdKnownNetworkIface,
		"PropertiesChanged",
		map[string]dbus.Variant{"AutoConnect": dbus.MakeVariant(false)},
		nil,
	)

	requireFired(t, fired)
	require.Equal(t, IwdKnownNetworkIface, recv.Interface)
	require.Equal(t, "PropertiesChanged", recv.Member)
}

func testKnownNetwork_GetName(t *testing.T) {
	t.Parallel()
	k := &KnownNetwork{call: &fakeCaller{getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
		require.Equal(t, IwdKnownNetworkIface, iface)
		require.Equal(t, "Name", prop)
		return "HomeNet", nil
	}}}
	name, err := k.GetName(context.Background())
	require.NoError(t, err)
	require.Equal(t, "HomeNet", name)
}

func testKnownNetwork_GetType(t *testing.T) {
	t.Parallel()
	k := &KnownNetwork{call: &fakeCaller{getPropFn: func(ctx context.Context, _, prop string) (interface{}, error) {
		require.Equal(t, "Type", prop)
		return "psk", nil
	}}}
	secType, err := k.GetType(context.Background())
	require.NoError(t, err)
	require.Equal(t, NetworkTypePSK, secType)
}

func testKnownNetwork_GetType_Hotspot(t *testing.T) {
	t.Parallel()
	k := &KnownNetwork{call: &fakeCaller{getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
		return "hotspot", nil
	}}}
	secType, err := k.GetType(context.Background())
	require.NoError(t, err)
	require.Equal(t, NetworkTypeHotspot, secType)
}

func testKnownNetwork_GetHidden(t *testing.T) {
	t.Parallel()
	k := &KnownNetwork{call: &fakeCaller{getPropFn: func(ctx context.Context, _, prop string) (interface{}, error) {
		require.Equal(t, "Hidden", prop)
		return true, nil
	}}}
	hidden, err := k.GetHidden(context.Background())
	require.NoError(t, err)
	require.True(t, hidden)
}

func testKnownNetwork_GetLastConnectedTime(t *testing.T) {
	t.Parallel()
	k := &KnownNetwork{call: &fakeCaller{getPropFn: func(ctx context.Context, _, prop string) (interface{}, error) {
		require.Equal(t, "LastConnectedTime", prop)
		return "2024-01-02T03:04:05Z", nil
	}}}
	lt, err := k.GetLastConnectedTime(context.Background())
	require.NoError(t, err)
	require.NotNil(t, lt)
	require.Equal(t, "2024-01-02T03:04:05Z", *lt)
}

func testKnownNetwork_GetLastConnectedTime_Absent(t *testing.T) {
	t.Parallel()
	// iwd omits LastConnectedTime when the network was never connected; the getter
	// collapses the "no value" reply to nil.
	k := &KnownNetwork{call: &fakeCaller{getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
		return nil, fmt.Errorf("Getting property value failed")
	}}}
	lt, err := k.GetLastConnectedTime(context.Background())
	require.NoError(t, err)
	require.Nil(t, lt)
}

func testKnownNetwork_GetAutoConnect(t *testing.T) {
	t.Parallel()
	k := &KnownNetwork{call: &fakeCaller{getPropFn: func(ctx context.Context, _, prop string) (interface{}, error) {
		require.Equal(t, "AutoConnect", prop)
		return true, nil
	}}}
	auto, err := k.GetAutoConnect(context.Background())
	require.NoError(t, err)
	require.True(t, auto)
}

func testKnownNetwork_SetAutoConnect(t *testing.T) {
	t.Parallel()
	var called bool
	k := &KnownNetwork{call: &fakeCaller{setPropFn: func(ctx context.Context, iface, prop string, val interface{}) error {
		called = true
		require.Equal(t, IwdKnownNetworkIface, iface)
		require.Equal(t, "AutoConnect", prop)
		require.Equal(t, false, val)
		return nil
	}}}
	require.NoError(t, k.SetAutoConnect(context.Background(), false))
	require.True(t, called)
}

func testKnownNetwork_SetAutoConnect_Err(t *testing.T) {
	t.Parallel()
	k := &KnownNetwork{call: &fakeCaller{setPropFn: func(ctx context.Context, iface, prop string, value interface{}) error {
		return fmt.Errorf("dbus failure")
	}}}
	err := k.SetAutoConnect(context.Background(), true)
	require.Error(t, err)
	require.Contains(t, err.Error(), "dbus failure")
}

func testKnownNetwork_Forget(t *testing.T) {
	t.Parallel()
	var called bool
	k := &KnownNetwork{call: &fakeCaller{callFn: func(ctx context.Context, iface, method string, _ ...interface{}) ([]interface{}, error) {
		called = true
		require.Equal(t, IwdKnownNetworkIface, iface)
		require.Equal(t, "Forget", method)
		return nil, nil
	}}}
	require.NoError(t, k.Forget(context.Background()))
	require.True(t, called)
}

func testKnownNetwork_Forget_ErrorMapping(t *testing.T) {
	t.Parallel()
	// A named iwd error from Forget maps to its sentinel via the shared method
	// error wrapper.
	k := &KnownNetwork{call: &fakeCaller{callFn: func(ctx context.Context, iface, method string, args ...interface{}) ([]interface{}, error) {
		return nil, dbus.Error{Name: IwdErrorBusy, Body: []interface{}{"busy"}}
	}}}
	err := k.Forget(context.Background())
	require.Error(t, err)
	require.ErrorIs(t, err, ErrBusy)
	require.ErrorIs(t, err, ErrDBusMethod)
}

func fullKnownNetworkProps() map[string]dbus.Variant {
	return map[string]dbus.Variant{
		"Name":              dbus.MakeVariant("HomeNet"),
		"Type":              dbus.MakeVariant("psk"),
		"Hidden":            dbus.MakeVariant(false),
		"LastConnectedTime": dbus.MakeVariant("2024-01-02T03:04:05Z"),
		"AutoConnect":       dbus.MakeVariant(true),
	}
}

func testKnownNetwork_GetProperties(t *testing.T) {
	t.Parallel()
	k := &KnownNetwork{call: &fakeCaller{getAllFn: func(ctx context.Context, iface string) (map[string]dbus.Variant, error) {
		require.Equal(t, IwdKnownNetworkIface, iface)
		return fullKnownNetworkProps(), nil
	}}}
	props, err := k.GetProperties(context.Background())
	require.NoError(t, err)
	require.Equal(t, "HomeNet", props.Name)
	require.Equal(t, NetworkTypePSK, props.Type)
	require.False(t, props.Hidden)
	require.NotNil(t, props.LastConnectedTime)
	require.Equal(t, "2024-01-02T03:04:05Z", *props.LastConnectedTime)
	require.True(t, props.AutoConnect)
}

func testKnownNetwork_GetProperties_NoLastConnectedTime(t *testing.T) {
	t.Parallel()
	k := &KnownNetwork{call: &fakeCaller{getAllFn: func(ctx context.Context, iface string) (map[string]dbus.Variant, error) {
		m := fullKnownNetworkProps()
		delete(m, "LastConnectedTime")
		return m, nil
	}}}
	props, err := k.GetProperties(context.Background())
	require.NoError(t, err)
	require.Nil(t, props.LastConnectedTime)
}

func testKnownNetwork_GetProperties_Errors(t *testing.T) {
	t.Parallel()

	without := func(key string) map[string]dbus.Variant {
		m := fullKnownNetworkProps()
		delete(m, key)
		return m
	}
	with := func(key string, v dbus.Variant) map[string]dbus.Variant {
		m := fullKnownNetworkProps()
		m[key] = v
		return m
	}

	cases := []struct {
		name         string
		props        map[string]dbus.Variant
		wantContains string
	}{
		{name: "missing Name", props: without("Name"), wantContains: "property=Name"},
		{name: "missing Type", props: without("Type"), wantContains: "property=Type"},
		{name: "missing Hidden", props: without("Hidden"), wantContains: "property=Hidden"},
		{name: "missing AutoConnect", props: without("AutoConnect"), wantContains: "property=AutoConnect"},
		{name: "Type invalid", props: with("Type", dbus.MakeVariant("wpa3")), wantContains: "invalid network type"},
		{name: "Hidden wrong type", props: with("Hidden", dbus.MakeVariant("nope")), wantContains: "expected bool"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			k := &KnownNetwork{call: &fakeCaller{getAllFn: func(ctx context.Context, iface string) (map[string]dbus.Variant, error) {
				return tc.props, nil
			}}}
			_, err := k.GetProperties(context.Background())
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.wantContains)
		})
	}
}

func testKnownNetwork_SubscribeAutoConnectChanged(t *testing.T) {
	t.Parallel()
	fake := newFakeSignalSource(t)
	known := &KnownNetwork{signals: fake}

	var recv bool
	fired := make(chan struct{}, 1)
	_, err := known.SubscribeAutoConnectChanged(context.Background(), func(b bool) {
		recv = b
		fired <- struct{}{}
	})
	require.NoError(t, err)

	fake.emit("org.freedesktop.DBus.Properties", "PropertiesChanged", IwdKnownNetworkIface, map[string]dbus.Variant{
		"AutoConnect": dbus.MakeVariant(false),
	}, nil)

	requireFired(t, fired)
	require.False(t, recv)
}

// testKnownNetwork_NoIntro checks every init-guarded method reports a clean
// "known network is not initialized" error when the KnownNetwork has no caller.
func testKnownNetwork_NoIntro(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	for _, tc := range []struct {
		name string
		call func(*KnownNetwork) error
	}{
		{"GetName", func(k *KnownNetwork) error { _, err := k.GetName(ctx); return err }},
		{"GetType", func(k *KnownNetwork) error { _, err := k.GetType(ctx); return err }},
		{"GetHidden", func(k *KnownNetwork) error { _, err := k.GetHidden(ctx); return err }},
		{"GetAutoConnect", func(k *KnownNetwork) error { _, err := k.GetAutoConnect(ctx); return err }},
		{"GetLastConnectedTime", func(k *KnownNetwork) error { _, err := k.GetLastConnectedTime(ctx); return err }},
		{"GetProperties", func(k *KnownNetwork) error { _, err := k.GetProperties(ctx); return err }},
		{"SetAutoConnect", func(k *KnownNetwork) error { return k.SetAutoConnect(ctx, true) }},
		{"Forget", func(k *KnownNetwork) error { return k.Forget(ctx) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.call(&KnownNetwork{call: nil})
			require.Error(t, err)
			require.Contains(t, err.Error(), "known network is not initialized")
		})
	}
}

func testKnownNetwork_SubscribeHiddenChanged(t *testing.T) {
	t.Parallel()

	fake := newFakeSignalSource(t)
	k := &KnownNetwork{signals: fake}

	got := make(chan bool, 1)
	_, err := k.SubscribeHiddenChanged(context.Background(), func(b bool) { got <- b })
	require.NoError(t, err)

	fake.emit("org.freedesktop.DBus.Properties", "PropertiesChanged", IwdKnownNetworkIface,
		map[string]dbus.Variant{"Hidden": dbus.MakeVariant(true)}, []string{})

	require.True(t, <-got)
}

func testKnownNetwork_SubscribeLastConnectedTimeChanged(t *testing.T) {
	t.Parallel()

	fake := newFakeSignalSource(t)
	k := &KnownNetwork{signals: fake}

	got := make(chan *string, 1)
	_, err := k.SubscribeLastConnectedTimeChanged(context.Background(), func(s *string) { got <- s })
	require.NoError(t, err)

	// iwd updates the timestamp on each successful connection.
	fake.emit("org.freedesktop.DBus.Properties", "PropertiesChanged", IwdKnownNetworkIface,
		map[string]dbus.Variant{"LastConnectedTime": dbus.MakeVariant("2026-07-13T10:04:00Z")}, []string{})

	ts := <-got
	require.NotNil(t, ts)
	require.Equal(t, "2026-07-13T10:04:00Z", *ts)
}

func testKnownNetwork_SubscribeNew_SkipMalformedAndNilCallback(t *testing.T) {
	t.Parallel()

	t.Run("malformed skipped", func(t *testing.T) {
		t.Parallel()
		fake := newFakeSignalSource(t)
		k := &KnownNetwork{signals: fake}

		fired := make(chan struct{}, 1)
		_, err := k.SubscribeLastConnectedTimeChanged(context.Background(), func(*string) { fired <- struct{}{} })
		require.NoError(t, err)

		fake.emit("org.freedesktop.DBus.Properties", "PropertiesChanged", IwdKnownNetworkIface,
			map[string]dbus.Variant{"LastConnectedTime": dbus.MakeVariant(int64(1))}, []string{})

		select {
		case <-fired:
			t.Fatal("callback fired for a malformed LastConnectedTime value")
		default:
		}
	})

	t.Run("nil callback", func(t *testing.T) {
		t.Parallel()
		k := &KnownNetwork{signals: newFakeSignalSource(t)}

		_, err := k.SubscribeHiddenChanged(context.Background(), nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "fn cannot be nil")

		_, err = k.SubscribeLastConnectedTimeChanged(context.Background(), nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "fn cannot be nil")
	})
}
