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
		t.Run("GetName_NoIntro", testKnownNetwork_GetName_NoIntro)
	})

	t.Run("Set", func(t *testing.T) {
		t.Parallel()
		t.Run("SetAutoConnect", testKnownNetwork_SetAutoConnect)
		t.Run("SetAutoConnect_Err", testKnownNetwork_SetAutoConnect_Err)
		t.Run("SetAutoConnect_NoIntro", testKnownNetwork_SetAutoConnect_NoIntro)
	})

	t.Run("Forget", func(t *testing.T) {
		t.Parallel()
		t.Run("Success", testKnownNetwork_Forget)
		t.Run("ErrorMapping", testKnownNetwork_Forget_ErrorMapping)
		t.Run("NoIntro", testKnownNetwork_Forget_NoIntro)
	})

	t.Run("Properties", func(t *testing.T) {
		t.Parallel()
		t.Run("Full", testKnownNetwork_GetProperties)
		t.Run("OptionalLastConnectedTimeAbsent", testKnownNetwork_GetProperties_NoLastConnectedTime)
		t.Run("Errors", testKnownNetwork_GetProperties_Errors)
	})

	t.Run("Subscribe", func(t *testing.T) {
		t.Parallel()
		t.Run("AutoConnectChanged", testKnownNetwork_SubscribeAutoConnectChanged)
	})
}

func testKnownNetwork_GetName(t *testing.T) {
	t.Parallel()
	k := &KnownNetwork{call: &fakeCaller{getPropFn: func(_ context.Context, iface, prop string) (interface{}, error) {
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
	k := &KnownNetwork{call: &fakeCaller{getPropFn: func(_ context.Context, _, prop string) (interface{}, error) {
		require.Equal(t, "Type", prop)
		return "psk", nil
	}}}
	secType, err := k.GetType(context.Background())
	require.NoError(t, err)
	require.Equal(t, NetworkTypePSK, secType)
}

func testKnownNetwork_GetType_Hotspot(t *testing.T) {
	t.Parallel()
	k := &KnownNetwork{call: &fakeCaller{getPropFn: func(_ context.Context, _, _ string) (interface{}, error) {
		return "hotspot", nil
	}}}
	secType, err := k.GetType(context.Background())
	require.NoError(t, err)
	require.Equal(t, NetworkTypeHotspot, secType)
}

func testKnownNetwork_GetHidden(t *testing.T) {
	t.Parallel()
	k := &KnownNetwork{call: &fakeCaller{getPropFn: func(_ context.Context, _, prop string) (interface{}, error) {
		require.Equal(t, "Hidden", prop)
		return true, nil
	}}}
	hidden, err := k.GetHidden(context.Background())
	require.NoError(t, err)
	require.True(t, hidden)
}

func testKnownNetwork_GetLastConnectedTime(t *testing.T) {
	t.Parallel()
	k := &KnownNetwork{call: &fakeCaller{getPropFn: func(_ context.Context, _, prop string) (interface{}, error) {
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
	k := &KnownNetwork{call: &fakeCaller{getPropFn: func(_ context.Context, _, _ string) (interface{}, error) {
		return nil, fmt.Errorf("Getting property value failed")
	}}}
	lt, err := k.GetLastConnectedTime(context.Background())
	require.NoError(t, err)
	require.Nil(t, lt)
}

func testKnownNetwork_GetAutoConnect(t *testing.T) {
	t.Parallel()
	k := &KnownNetwork{call: &fakeCaller{getPropFn: func(_ context.Context, _, prop string) (interface{}, error) {
		require.Equal(t, "AutoConnect", prop)
		return true, nil
	}}}
	auto, err := k.GetAutoConnect(context.Background())
	require.NoError(t, err)
	require.True(t, auto)
}

func testKnownNetwork_GetName_NoIntro(t *testing.T) {
	t.Parallel()
	k := &KnownNetwork{call: nil}
	_, err := k.GetName(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "known network is not initialized")
}

func testKnownNetwork_SetAutoConnect(t *testing.T) {
	t.Parallel()
	var called bool
	k := &KnownNetwork{call: &fakeCaller{setPropFn: func(_ context.Context, iface, prop string, val interface{}) error {
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
	k := &KnownNetwork{call: &fakeCaller{setPropFn: func(_ context.Context, _, _ string, _ interface{}) error {
		return fmt.Errorf("dbus failure")
	}}}
	err := k.SetAutoConnect(context.Background(), true)
	require.Error(t, err)
	require.Contains(t, err.Error(), "dbus failure")
}

func testKnownNetwork_SetAutoConnect_NoIntro(t *testing.T) {
	t.Parallel()
	k := &KnownNetwork{call: nil}
	err := k.SetAutoConnect(context.Background(), true)
	require.Error(t, err)
	require.Contains(t, err.Error(), "known network is not initialized")
}

func testKnownNetwork_Forget(t *testing.T) {
	t.Parallel()
	var called bool
	k := &KnownNetwork{call: &fakeCaller{callFn: func(_ context.Context, iface, method string, _ ...interface{}) ([]interface{}, error) {
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
	k := &KnownNetwork{call: &fakeCaller{callFn: func(_ context.Context, _, _ string, _ ...interface{}) ([]interface{}, error) {
		return nil, dbus.Error{Name: IwdErrorBusy, Body: []interface{}{"busy"}}
	}}}
	err := k.Forget(context.Background())
	require.Error(t, err)
	require.ErrorIs(t, err, ErrBusy)
	require.ErrorIs(t, err, ErrDBusMethod)
}

func testKnownNetwork_Forget_NoIntro(t *testing.T) {
	t.Parallel()
	k := &KnownNetwork{call: nil}
	err := k.Forget(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "known network is not initialized")
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
	k := &KnownNetwork{call: &fakeCaller{getAllFn: func(_ context.Context, iface string) (map[string]dbus.Variant, error) {
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
	k := &KnownNetwork{call: &fakeCaller{getAllFn: func(_ context.Context, _ string) (map[string]dbus.Variant, error) {
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
			k := &KnownNetwork{call: &fakeCaller{getAllFn: func(_ context.Context, _ string) (map[string]dbus.Variant, error) {
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
