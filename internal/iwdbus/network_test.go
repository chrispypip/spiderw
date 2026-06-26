//go:build unit

package iwdbus

import (
	"context"
	"fmt"
	"testing"

	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/require"
)

func TestNetwork_Iwdbus(t *testing.T) {
	t.Parallel()

	t.Run("Getters", func(t *testing.T) {
		t.Parallel()
		t.Run("GetName", testNetwork_GetName)
		t.Run("GetConnected", testNetwork_GetConnected)
		t.Run("GetDevice", testNetwork_GetDevice)
		t.Run("GetType", testNetwork_GetType)
		t.Run("GetType_Invalid", testNetwork_GetType_Invalid)
		t.Run("GetKnownNetwork", testNetwork_GetKnownNetwork)
		t.Run("GetKnownNetwork_Absent", testNetwork_GetKnownNetwork_Absent)
		t.Run("GetExtendedServiceSet", testNetwork_GetExtendedServiceSet)
		t.Run("GetName_NoIntro", testNetwork_GetName_NoIntro)
	})

	t.Run("Connect", func(t *testing.T) {
		t.Parallel()
		t.Run("Success", testNetwork_Connect)
		t.Run("NoAgent", testNetwork_Connect_NoAgent)
		t.Run("OtherError", testNetwork_Connect_OtherError)
		t.Run("NoIntro", testNetwork_Connect_NoIntro)
	})

	t.Run("Properties", func(t *testing.T) {
		t.Parallel()
		t.Run("Full", testNetwork_GetProperties)
		t.Run("OptionalKnownNetworkAbsent", testNetwork_GetProperties_NoKnownNetwork)
		t.Run("Errors", testNetwork_GetProperties_Errors)
	})

	t.Run("Subscribe", func(t *testing.T) {
		t.Parallel()
		t.Run("ConnectedChanged", testNetwork_SubscribeConnectedChanged)
	})

	t.Run("Firehose", func(t *testing.T) {
		t.Parallel()
		t.Run("ReceivesAll", testNetwork_FirehoseReceivesAll)
		t.Run("PropertiesChanged", testNetwork_FirehosePropertiesChanged)
	})
}

func testNetwork_GetName(t *testing.T) {
	t.Parallel()
	n := &Network{call: &fakeCaller{getPropFn: func(_ context.Context, iface, prop string) (interface{}, error) {
		require.Equal(t, IwdNetworkIface, iface)
		require.Equal(t, "Name", prop)
		return "OpenNet", nil
	}}}
	name, err := n.GetName(context.Background())
	require.NoError(t, err)
	require.Equal(t, "OpenNet", name)
}

func testNetwork_GetConnected(t *testing.T) {
	t.Parallel()
	n := &Network{call: &fakeCaller{getPropFn: func(_ context.Context, _, prop string) (interface{}, error) {
		require.Equal(t, "Connected", prop)
		return true, nil
	}}}
	connected, err := n.GetConnected(context.Background())
	require.NoError(t, err)
	require.True(t, connected)
}

func testNetwork_GetDevice(t *testing.T) {
	t.Parallel()
	n := &Network{call: &fakeCaller{getPropFn: func(_ context.Context, _, prop string) (interface{}, error) {
		require.Equal(t, "Device", prop)
		return dbus.ObjectPath("/net/connman/iwd/phy0/wlan0"), nil
	}}}
	device, err := n.GetDevice(context.Background())
	require.NoError(t, err)
	require.Equal(t, dbus.ObjectPath("/net/connman/iwd/phy0/wlan0"), device)
}

func testNetwork_GetType(t *testing.T) {
	t.Parallel()
	n := &Network{call: &fakeCaller{getPropFn: func(_ context.Context, _, prop string) (interface{}, error) {
		require.Equal(t, "Type", prop)
		return "psk", nil
	}}}
	secType, err := n.GetType(context.Background())
	require.NoError(t, err)
	require.Equal(t, NetworkTypePSK, secType)
}

func testNetwork_GetType_Invalid(t *testing.T) {
	t.Parallel()
	n := &Network{call: &fakeCaller{getPropFn: func(_ context.Context, _, _ string) (interface{}, error) {
		return "wpa3", nil
	}}}
	_, err := n.GetType(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid network type")
}

func testNetwork_GetKnownNetwork(t *testing.T) {
	t.Parallel()
	n := &Network{call: &fakeCaller{getPropFn: func(_ context.Context, _, prop string) (interface{}, error) {
		require.Equal(t, "KnownNetwork", prop)
		return dbus.ObjectPath("/net/connman/iwd/known_networks/1"), nil
	}}}
	known, err := n.GetKnownNetwork(context.Background())
	require.NoError(t, err)
	require.NotNil(t, known)
	require.Equal(t, "/net/connman/iwd/known_networks/1", *known)
}

func testNetwork_GetKnownNetwork_Absent(t *testing.T) {
	t.Parallel()
	// iwd declines optional properties with a "Getting property value failed"
	// style reply; the wrapper collapses that to nil.
	n := &Network{call: &fakeCaller{getPropFn: func(_ context.Context, _, _ string) (interface{}, error) {
		return nil, fmt.Errorf("Getting property value failed")
	}}}
	known, err := n.GetKnownNetwork(context.Background())
	require.NoError(t, err)
	require.Nil(t, known)
}

func testNetwork_GetExtendedServiceSet(t *testing.T) {
	t.Parallel()
	n := &Network{call: &fakeCaller{getPropFn: func(_ context.Context, _, prop string) (interface{}, error) {
		require.Equal(t, "ExtendedServiceSet", prop)
		return []dbus.ObjectPath{
			"/net/connman/iwd/phy0/wlan0/aabbccddeeff",
			"/net/connman/iwd/phy0/wlan0/bbccddeeff00",
		}, nil
	}}}
	ess, err := n.GetExtendedServiceSet(context.Background())
	require.NoError(t, err)
	require.Equal(t, []string{
		"/net/connman/iwd/phy0/wlan0/aabbccddeeff",
		"/net/connman/iwd/phy0/wlan0/bbccddeeff00",
	}, ess)
}

func testNetwork_GetName_NoIntro(t *testing.T) {
	t.Parallel()
	n := &Network{call: nil}
	_, err := n.GetName(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "network is not initialized")
}

func testNetwork_Connect(t *testing.T) {
	t.Parallel()
	var called bool
	n := &Network{call: &fakeCaller{callFn: func(_ context.Context, iface, method string, _ ...interface{}) ([]interface{}, error) {
		called = true
		require.Equal(t, IwdNetworkIface, iface)
		require.Equal(t, "Connect", method)
		return nil, nil
	}}}
	require.NoError(t, n.Connect(context.Background()))
	require.True(t, called)
}

func testNetwork_Connect_NoAgent(t *testing.T) {
	t.Parallel()
	n := &Network{call: &fakeCaller{callFn: func(_ context.Context, _, _ string, _ ...interface{}) ([]interface{}, error) {
		return nil, dbus.Error{Name: IwdErrorNoAgent, Body: []interface{}{"No agent registered"}}
	}}}
	err := n.Connect(context.Background())
	require.Error(t, err)
	require.ErrorIs(t, err, ErrNoAgent)
}

func testNetwork_Connect_OtherError(t *testing.T) {
	t.Parallel()
	// An unrecognized iwd error name falls back to a generic method error with no
	// specific sentinel.
	n := &Network{call: &fakeCaller{callFn: func(_ context.Context, _, _ string, _ ...interface{}) ([]interface{}, error) {
		return nil, dbus.Error{Name: "net.connman.iwd.SomethingNew", Body: []interface{}{"boom"}}
	}}}
	err := n.Connect(context.Background())
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrNoAgent)
	require.NotErrorIs(t, err, ErrFailed)
	require.ErrorIs(t, err, ErrDBusMethod)
}

func testNetwork_Connect_NoIntro(t *testing.T) {
	t.Parallel()
	n := &Network{call: nil}
	err := n.Connect(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "network is not initialized")
}

func fullNetworkProps() map[string]dbus.Variant {
	return map[string]dbus.Variant{
		"Name":               dbus.MakeVariant("OpenNet"),
		"Connected":          dbus.MakeVariant(false),
		"Device":             dbus.MakeVariant(dbus.ObjectPath("/net/connman/iwd/phy0/wlan0")),
		"Type":               dbus.MakeVariant("open"),
		"KnownNetwork":       dbus.MakeVariant(dbus.ObjectPath("/net/connman/iwd/known_networks/1")),
		"ExtendedServiceSet": dbus.MakeVariant([]dbus.ObjectPath{"/net/connman/iwd/phy0/wlan0/aabbccddeeff"}),
	}
}

func testNetwork_GetProperties(t *testing.T) {
	t.Parallel()
	n := &Network{call: &fakeCaller{getAllFn: func(_ context.Context, iface string) (map[string]dbus.Variant, error) {
		require.Equal(t, IwdNetworkIface, iface)
		return fullNetworkProps(), nil
	}}}
	props, err := n.GetProperties(context.Background())
	require.NoError(t, err)
	require.Equal(t, "OpenNet", props.Name)
	require.False(t, props.Connected)
	require.Equal(t, dbus.ObjectPath("/net/connman/iwd/phy0/wlan0"), props.Device)
	require.Equal(t, NetworkTypeOpen, props.Type)
	require.NotNil(t, props.KnownNetwork)
	require.Equal(t, "/net/connman/iwd/known_networks/1", *props.KnownNetwork)
	require.Equal(t, []string{"/net/connman/iwd/phy0/wlan0/aabbccddeeff"}, props.ExtendedServiceSet)
}

func testNetwork_GetProperties_NoKnownNetwork(t *testing.T) {
	t.Parallel()
	n := &Network{call: &fakeCaller{getAllFn: func(_ context.Context, _ string) (map[string]dbus.Variant, error) {
		m := fullNetworkProps()
		delete(m, "KnownNetwork")
		return m, nil
	}}}
	props, err := n.GetProperties(context.Background())
	require.NoError(t, err)
	require.Nil(t, props.KnownNetwork)
}

func testNetwork_GetProperties_Errors(t *testing.T) {
	t.Parallel()

	without := func(key string) map[string]dbus.Variant {
		m := fullNetworkProps()
		delete(m, key)
		return m
	}
	with := func(key string, v dbus.Variant) map[string]dbus.Variant {
		m := fullNetworkProps()
		m[key] = v
		return m
	}

	cases := []struct {
		name         string
		props        map[string]dbus.Variant
		wantContains string
	}{
		{name: "missing Name", props: without("Name"), wantContains: "property=Name"},
		{name: "missing Connected", props: without("Connected"), wantContains: "property=Connected"},
		{name: "missing Device", props: without("Device"), wantContains: "property=Device"},
		{name: "missing Type", props: without("Type"), wantContains: "property=Type"},
		{name: "missing ExtendedServiceSet", props: without("ExtendedServiceSet"), wantContains: "property=ExtendedServiceSet"},
		{name: "Type invalid", props: with("Type", dbus.MakeVariant("wpa3")), wantContains: "invalid network type"},
		{name: "Connected wrong type", props: with("Connected", dbus.MakeVariant("nope")), wantContains: "expected bool"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			n := &Network{call: &fakeCaller{getAllFn: func(_ context.Context, _ string) (map[string]dbus.Variant, error) {
				return tc.props, nil
			}}}
			_, err := n.GetProperties(context.Background())
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.wantContains)
		})
	}
}

func testNetwork_SubscribeConnectedChanged(t *testing.T) {
	t.Parallel()
	fake := newFakeSignalSource(t)
	network := &Network{signals: fake}

	var recv bool
	fired := make(chan struct{}, 1)
	_, err := network.SubscribeConnectedChanged(context.Background(), func(b bool) {
		recv = b
		fired <- struct{}{}
	})
	require.NoError(t, err)

	fake.emit("org.freedesktop.DBus.Properties", "PropertiesChanged", IwdNetworkIface, map[string]dbus.Variant{
		"Connected": dbus.MakeVariant(true),
	}, nil)

	requireFired(t, fired)
	require.True(t, recv)
}

func testNetwork_FirehoseReceivesAll(t *testing.T) {
	fake := newFakeSignalSource(t)
	network := &Network{signals: fake}

	var recv FirehoseSignal
	fired := make(chan struct{}, 1)

	err := network.Firehose(context.Background(), func(s FirehoseSignal) {
		recv = s
		fired <- struct{}{}
	})
	require.NoError(t, err)

	fake.emit(
		IwdNetworkIface,
		"ConnectedChanged",
		map[string]dbus.Variant{"Connected": dbus.MakeVariant(true)},
		nil,
	)

	requireFired(t, fired)
	require.Equal(t, IwdNetworkIface, recv.Interface)
	require.Equal(t, "ConnectedChanged", recv.Member)
}

func testNetwork_FirehosePropertiesChanged(t *testing.T) {
	fake := newFakeSignalSource(t)
	network := &Network{signals: fake}

	fired := make(chan struct{}, 1)
	var recv FirehoseSignal

	_ = network.Firehose(context.Background(), func(s FirehoseSignal) {
		recv = s
		fired <- struct{}{}
	})

	changed := map[string]dbus.Variant{"Connected": dbus.MakeVariant(true)}
	fake.emit("org.freedesktop.DBus.Properties", "PropertiesChanged", IwdNetworkIface, changed, []string{})

	requireFired(t, fired)
	require.Equal(t, "org.freedesktop.DBus.Properties", recv.Interface)
	require.Equal(t, "PropertiesChanged", recv.Member)
	require.Len(t, recv.Body, 3)

	s, ok := recv.Body[0].(string)
	require.True(t, ok)
	require.Equal(t, IwdNetworkIface, s)
}
