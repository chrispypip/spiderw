//go:build unit

package iwdbus

import (
	"context"
	"fmt"
	"testing"

	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/require"
)

func TestStation_Iwdbus(t *testing.T) {
	t.Parallel()

	t.Run("Getters", func(t *testing.T) {
		t.Parallel()
		t.Run("Station_GetState", testStation_GetState)
		t.Run("Station_GetState_Invalid", testStation_GetState_Invalid)
		t.Run("Station_GetScanning", testStation_GetScanning)
		t.Run("Station_GetConnectedNetwork", testStation_GetConnectedNetwork)
		t.Run("Station_GetConnectedNetwork_Absent", testStation_GetConnectedNetwork_Absent)
		t.Run("Station_GetConnectedNetwork_RootPath", testStation_GetConnectedNetwork_RootPath)
		t.Run("Station_GetConnectedAccessPoint", testStation_GetConnectedAccessPoint)
		t.Run("Station_GetConnectedAccessPoint_Absent", testStation_GetConnectedAccessPoint_Absent)
		t.Run("Station_GetAffinities", testStation_GetAffinities)
		t.Run("Station_GetAffinities_Empty", testStation_GetAffinities_Empty)
		t.Run("Station_GetAffinities_Absent", testStation_GetAffinities_Absent)
		t.Run("Station_GetterWrongTypes", testStation_GetterWrongTypes)
		t.Run("Station_GetterBackendErrors", testStation_GetterBackendErrors)
	})

	t.Run("Properties", func(t *testing.T) {
		t.Parallel()
		t.Run("Station_GetProperties", testStation_GetProperties)
		t.Run("Station_GetProperties_Disconnected", testStation_GetProperties_Disconnected)
		t.Run("Station_GetProperties_Errors", testStation_GetProperties_Errors)
	})

	t.Run("Operations", func(t *testing.T) {
		t.Parallel()
		t.Run("Station_Scan", testStation_Scan)
		t.Run("Station_Scan_Err", testStation_Scan_Err)
		t.Run("Station_GetOrderedNetworks", testStation_GetOrderedNetworks)
		t.Run("Station_GetOrderedNetworks_Empty", testStation_GetOrderedNetworks_Empty)
		t.Run("Station_GetOrderedNetworks_Err", testStation_GetOrderedNetworks_Err)
		t.Run("Station_SetAffinities", testStation_SetAffinities)
		t.Run("Station_SetAffinities_Err", testStation_SetAffinities_Err)
		t.Run("Station_SetAffinities_NotSupportedMatchable", testStation_SetAffinities_NotSupportedMatchable)
		t.Run("Station_Disconnect", testStation_Disconnect)
		t.Run("Station_Disconnect_Err", testStation_Disconnect_Err)
		t.Run("Station_ConnectHiddenNetwork", testStation_ConnectHiddenNetwork)
		t.Run("Station_ConnectHiddenNetwork_NotFoundMatchable", testStation_ConnectHiddenNetwork_NotFoundMatchable)
		t.Run("Station_GetHiddenAccessPoints", testStation_GetHiddenAccessPoints)
		t.Run("Station_GetHiddenAccessPoints_Empty", testStation_GetHiddenAccessPoints_Empty)
		t.Run("Station_GetHiddenAccessPoints_BadType", testStation_GetHiddenAccessPoints_BadType)
		t.Run("Station_GetHiddenAccessPoints_Err", testStation_GetHiddenAccessPoints_Err)
	})

	t.Run("NotInitialized", testStation_NoIntro)

	t.Run("Subscribe", func(t *testing.T) {
		t.Parallel()
		t.Run("Station_SubscribePropertiesChanged", testStation_SubscribePropertiesChanged)
		t.Run("Station_SubscribeStateChanged", testStation_SubscribeStateChanged)
		t.Run("Station_SubscribeStateChanged_IgnoresInvalid", testStation_SubscribeStateChanged_IgnoresInvalid)
		t.Run("Station_SubscribeScanningChanged", testStation_SubscribeScanningChanged)
		t.Run("Station_SubscribeScanningChanged_IgnoresUnrelated", testStation_SubscribeScanningChanged_IgnoresUnrelated)
		t.Run("Station_SubscribeScanningChanged_Unsubscribe", testStation_SubscribeScanningChanged_Unsubscribe)
	})

	t.Run("Firehose", func(t *testing.T) {
		t.Parallel()
		t.Run("Station_FirehoseReceivesAll", testStation_FirehoseReceivesAll)
	})
}

func testStation_GetState(t *testing.T) {
	t.Parallel()

	s := &Station{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			require.Equal(t, IwdStationIface, iface)
			require.Equal(t, "State", prop)
			return "connected", nil
		},
	}}

	state, err := s.GetState(context.Background())
	require.NoError(t, err)
	require.Equal(t, StationStateConnected, state)
}

func testStation_GetState_Invalid(t *testing.T) {
	t.Parallel()

	s := &Station{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			return "bad-state", nil
		},
	}}

	_, err := s.GetState(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid station state")
}

// testStation_GetterWrongTypes checks that every scalar property getter reports a
// type-specific conversion error when the backend returns the wrong Go type.
func testStation_GetterWrongTypes(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		badValue interface{}
		call     func(context.Context, *Station) error
		wantHint string
	}{
		{"GetState", 123, func(ctx context.Context, s *Station) error { _, err := s.GetState(ctx); return err }, "expected string"},
		{"GetScanning", "nope", func(ctx context.Context, s *Station) error { _, err := s.GetScanning(ctx); return err }, "expected bool"},
		{"GetAffinities", "not-an-array", func(ctx context.Context, s *Station) error { _, err := s.GetAffinities(ctx); return err }, "expected object path array"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s := &Station{call: &fakeCaller{
				getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
					return tc.badValue, nil
				},
			}}

			err := tc.call(context.Background(), s)
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.wantHint)
		})
	}
}

// testStation_GetterBackendErrors checks that a generic backend read failure
// (distinct from the "unknown property" absence case) surfaces as an error.
func testStation_GetterBackendErrors(t *testing.T) {
	t.Parallel()

	newFailing := func() *Station {
		return &Station{call: &fakeCaller{
			getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
				return nil, fmt.Errorf("dbus failure")
			},
		}}
	}

	for _, tc := range []struct {
		name string
		call func(context.Context, *Station) error
	}{
		{"GetState", func(ctx context.Context, s *Station) error { _, err := s.GetState(ctx); return err }},
		{"GetConnectedNetwork", func(ctx context.Context, s *Station) error { _, err := s.GetConnectedNetwork(ctx); return err }},
		{"GetConnectedAccessPoint", func(ctx context.Context, s *Station) error { _, err := s.GetConnectedAccessPoint(ctx); return err }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.call(context.Background(), newFailing())
			require.Error(t, err)
			require.Contains(t, err.Error(), "dbus failure")
		})
	}
}

func testStation_GetScanning(t *testing.T) {
	t.Parallel()

	s := &Station{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			require.Equal(t, IwdStationIface, iface)
			require.Equal(t, "Scanning", prop)
			return true, nil
		},
	}}

	scanning, err := s.GetScanning(context.Background())
	require.NoError(t, err)
	require.True(t, scanning)
}

func testStation_GetConnectedNetwork(t *testing.T) {
	t.Parallel()

	const path = "/net/connman/iwd/phy0/wlan0/net0"
	s := &Station{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			require.Equal(t, IwdStationIface, iface)
			require.Equal(t, "ConnectedNetwork", prop)
			return dbus.ObjectPath(path), nil
		},
	}}

	got, err := s.GetConnectedNetwork(context.Background())
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, path, *got)
}

func testStation_GetConnectedNetwork_Absent(t *testing.T) {
	t.Parallel()

	// iwd omits ConnectedNetwork while disconnected; the getter reports it as an
	// "unknown property" failure, which must collapse to (nil, nil).
	s := &Station{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			return nil, fmt.Errorf("Getting property value failed")
		},
	}}

	got, err := s.GetConnectedNetwork(context.Background())
	require.NoError(t, err)
	require.Nil(t, got)
}

func testStation_GetConnectedNetwork_RootPath(t *testing.T) {
	t.Parallel()

	// The root path "/" is iwd's "no object" sentinel and must yield nil.
	s := &Station{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			return dbus.ObjectPath("/"), nil
		},
	}}

	got, err := s.GetConnectedNetwork(context.Background())
	require.NoError(t, err)
	require.Nil(t, got)
}

func testStation_GetConnectedAccessPoint(t *testing.T) {
	t.Parallel()

	const path = "/net/connman/iwd/phy0/wlan0/abc123"
	s := &Station{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			require.Equal(t, IwdStationIface, iface)
			require.Equal(t, "ConnectedAccessPoint", prop)
			return dbus.ObjectPath(path), nil
		},
	}}

	got, err := s.GetConnectedAccessPoint(context.Background())
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, path, *got)
}

func testStation_GetConnectedAccessPoint_Absent(t *testing.T) {
	t.Parallel()

	// ConnectedAccessPoint is experimental/optional; an "unknown property"
	// failure must collapse to (nil, nil).
	s := &Station{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			return nil, fmt.Errorf("Getting property value failed")
		},
	}}

	got, err := s.GetConnectedAccessPoint(context.Background())
	require.NoError(t, err)
	require.Nil(t, got)
}

func testStation_GetAffinities(t *testing.T) {
	t.Parallel()

	s := &Station{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			require.Equal(t, IwdStationIface, iface)
			require.Equal(t, "Affinities", prop)
			return []dbus.ObjectPath{
				"/net/connman/iwd/phy0/wlan0/aaa",
				"/net/connman/iwd/phy0/wlan0/bbb",
			}, nil
		},
	}}

	got, err := s.GetAffinities(context.Background())
	require.NoError(t, err)
	require.Equal(t, []string{
		"/net/connman/iwd/phy0/wlan0/aaa",
		"/net/connman/iwd/phy0/wlan0/bbb",
	}, got)
}

func testStation_GetAffinities_Empty(t *testing.T) {
	t.Parallel()

	// A present-but-empty array is distinct from absent: it yields an empty
	// (non-nil) slice.
	s := &Station{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			return []dbus.ObjectPath{}, nil
		},
	}}

	got, err := s.GetAffinities(context.Background())
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Empty(t, got)
}

func testStation_GetAffinities_Absent(t *testing.T) {
	t.Parallel()

	// Affinities is experimental/optional; an "unknown property" failure must
	// collapse to (nil, nil).
	s := &Station{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			return nil, fmt.Errorf("Getting property value failed")
		},
	}}

	got, err := s.GetAffinities(context.Background())
	require.NoError(t, err)
	require.Nil(t, got)
}

func testStation_GetProperties(t *testing.T) {
	t.Parallel()

	const path = "/net/connman/iwd/phy0/wlan0/net0"
	const ap = "/net/connman/iwd/phy0/wlan0/abc123"
	s := newGetAllStation(func(ctx context.Context, iface string) (map[string]dbus.Variant, error) {
		require.Equal(t, IwdStationIface, iface)
		return map[string]dbus.Variant{
			"State":                dbus.MakeVariant("connected"),
			"Scanning":             dbus.MakeVariant(false),
			"ConnectedNetwork":     dbus.MakeVariant(dbus.ObjectPath(path)),
			"ConnectedAccessPoint": dbus.MakeVariant(dbus.ObjectPath(ap)),
			"Affinities":           dbus.MakeVariant([]dbus.ObjectPath{dbus.ObjectPath(ap)}),
		}, nil
	})

	props, err := s.GetProperties(context.Background())
	require.NoError(t, err)
	require.Equal(t, StationStateConnected, props.State)
	require.False(t, props.Scanning)
	require.NotNil(t, props.ConnectedNetwork)
	require.Equal(t, path, *props.ConnectedNetwork)
	require.NotNil(t, props.ConnectedAccessPoint)
	require.Equal(t, ap, *props.ConnectedAccessPoint)
	require.Equal(t, []string{ap}, props.Affinities)
}

func testStation_GetProperties_Disconnected(t *testing.T) {
	t.Parallel()

	// A disconnected station omits ConnectedNetwork entirely; that is not an
	// error, it leaves ConnectedNetwork nil.
	s := newGetAllStation(func(ctx context.Context, iface string) (map[string]dbus.Variant, error) {
		return map[string]dbus.Variant{
			"State":    dbus.MakeVariant("disconnected"),
			"Scanning": dbus.MakeVariant(false),
		}, nil
	})

	props, err := s.GetProperties(context.Background())
	require.NoError(t, err)
	require.Equal(t, StationStateDisconnected, props.State)
	require.Nil(t, props.ConnectedNetwork)
	require.Nil(t, props.ConnectedAccessPoint)
	require.Nil(t, props.Affinities)
}

func testStation_GetProperties_Errors(t *testing.T) {
	t.Parallel()

	full := func() map[string]dbus.Variant {
		return map[string]dbus.Variant{
			"State":                dbus.MakeVariant("connected"),
			"Scanning":             dbus.MakeVariant(false),
			"ConnectedNetwork":     dbus.MakeVariant(dbus.ObjectPath("/net/connman/iwd/phy0/wlan0/net0")),
			"ConnectedAccessPoint": dbus.MakeVariant(dbus.ObjectPath("/net/connman/iwd/phy0/wlan0/abc123")),
			"Affinities":           dbus.MakeVariant([]dbus.ObjectPath{}),
		}
	}

	without := func(key string) map[string]dbus.Variant {
		m := full()
		delete(m, key)
		return m
	}

	with := func(key string, v dbus.Variant) map[string]dbus.Variant {
		m := full()
		m[key] = v
		return m
	}

	cases := []struct {
		name         string
		props        map[string]dbus.Variant
		callErr      error
		wantContains string
	}{
		{name: "missing State", props: without("State"), wantContains: "property=State"},
		{name: "missing Scanning", props: without("Scanning"), wantContains: "property=Scanning"},
		{name: "State invalid", props: with("State", dbus.MakeVariant("bad-state")), wantContains: "invalid station state"},
		{name: "State wrong type", props: with("State", dbus.MakeVariant(123)), wantContains: "expected string"},
		{name: "Scanning wrong type", props: with("Scanning", dbus.MakeVariant("nope")), wantContains: "expected bool"},
		{name: "ConnectedNetwork wrong type", props: with("ConnectedNetwork", dbus.MakeVariant(42)), wantContains: "expected object path"},
		{name: "ConnectedAccessPoint wrong type", props: with("ConnectedAccessPoint", dbus.MakeVariant(42)), wantContains: "expected object path"},
		{name: "Affinities wrong type", props: with("Affinities", dbus.MakeVariant("nope")), wantContains: "expected object path array"},
		{name: "GetAll call error", callErr: fmt.Errorf("dbus failure"), wantContains: "dbus failure"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s := newGetAllStation(func(ctx context.Context, iface string) (map[string]dbus.Variant, error) {
				if tc.callErr != nil {
					return nil, tc.callErr
				}
				return tc.props, nil
			})

			_, err := s.GetProperties(context.Background())
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.wantContains)
		})
	}
}

func testStation_Scan(t *testing.T) {
	t.Parallel()

	var called bool
	s := &Station{call: &fakeCaller{
		callFn: func(ctx context.Context, iface, method string, args ...interface{}) ([]interface{}, error) {
			called = true
			require.Equal(t, IwdStationIface, iface)
			require.Equal(t, "Scan", method)
			return nil, nil
		},
	}}

	require.NoError(t, s.Scan(context.Background()))
	require.True(t, called)
}

func testStation_Scan_Err(t *testing.T) {
	t.Parallel()

	s := &Station{call: &fakeCaller{
		callFn: func(ctx context.Context, iface, method string, args ...interface{}) ([]interface{}, error) {
			return nil, fmt.Errorf("dbus failure")
		},
	}}

	err := s.Scan(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "dbus failure")
}

func testStation_GetOrderedNetworks(t *testing.T) {
	t.Parallel()

	s := &Station{call: &fakeCaller{
		callFn: func(ctx context.Context, iface, method string, args ...interface{}) ([]interface{}, error) {
			require.Equal(t, IwdStationIface, iface)
			require.Equal(t, "GetOrderedNetworks", method)
			// a(on): array of (object path, int16 signal). godbus decodes each
			// (on) struct to a []interface{}.
			return []interface{}{
				[][]interface{}{
					{dbus.ObjectPath("/net/connman/iwd/phy0/wlan0/net0"), int16(-6000)},
					{dbus.ObjectPath("/net/connman/iwd/phy0/wlan0/net1"), int16(-7200)},
				},
			}, nil
		},
	}}

	got, err := s.GetOrderedNetworks(context.Background())
	require.NoError(t, err)
	require.Equal(t, []OrderedNetwork{
		{Network: "/net/connman/iwd/phy0/wlan0/net0", SignalStrength: -6000},
		{Network: "/net/connman/iwd/phy0/wlan0/net1", SignalStrength: -7200},
	}, got)
}

func testStation_GetOrderedNetworks_Empty(t *testing.T) {
	t.Parallel()

	s := &Station{call: &fakeCaller{
		callFn: func(ctx context.Context, iface, method string, args ...interface{}) ([]interface{}, error) {
			return []interface{}{[][]interface{}{}}, nil
		},
	}}

	got, err := s.GetOrderedNetworks(context.Background())
	require.NoError(t, err)
	require.Empty(t, got)
}

func testStation_GetOrderedNetworks_Err(t *testing.T) {
	t.Parallel()

	s := &Station{call: &fakeCaller{
		callFn: func(ctx context.Context, iface, method string, args ...interface{}) ([]interface{}, error) {
			return nil, fmt.Errorf("dbus failure")
		},
	}}

	_, err := s.GetOrderedNetworks(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "dbus failure")
}

func testStation_SetAffinities(t *testing.T) {
	t.Parallel()

	var got interface{}
	s := &Station{call: &fakeCaller{
		setPropFn: func(ctx context.Context, iface, prop string, value interface{}) error {
			require.Equal(t, IwdStationIface, iface)
			require.Equal(t, "Affinities", prop)
			got = value
			return nil
		},
	}}

	err := s.SetAffinities(context.Background(), []string{
		"/net/connman/iwd/phy0/wlan0/aabbccddeeff",
		"/net/connman/iwd/phy0/wlan0/bbccddeeff00",
	})
	require.NoError(t, err)
	require.Equal(t, []dbus.ObjectPath{
		"/net/connman/iwd/phy0/wlan0/aabbccddeeff",
		"/net/connman/iwd/phy0/wlan0/bbccddeeff00",
	}, got)
}

func testStation_SetAffinities_Err(t *testing.T) {
	t.Parallel()

	s := &Station{call: &fakeCaller{
		setPropFn: func(ctx context.Context, iface, prop string, value interface{}) error {
			return fmt.Errorf("dbus failure")
		},
	}}

	err := s.SetAffinities(context.Background(), []string{"/x"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "dbus failure")
}

func testStation_SetAffinities_NotSupportedMatchable(t *testing.T) {
	t.Parallel()

	// iwd rejects the write on hardware that can't honor it; the named
	// net.connman.iwd.NotSupported error must surface as a matchable sentinel.
	s := &Station{call: &fakeCaller{
		setPropFn: func(ctx context.Context, iface, prop string, value interface{}) error {
			return dbus.Error{
				Name: IwdErrorNotSupported,
				Body: []interface{}{"Operation not supported"},
			}
		},
	}}

	err := s.SetAffinities(context.Background(), []string{"/net/connman/iwd/0/3/net/a0b1c2d3e4f5"})
	require.Error(t, err)
	require.ErrorIs(t, err, ErrNotSupported, "expected ErrNotSupported, got %v", err)
	require.ErrorIs(t, err, ErrDBusProperty, "should still classify as a property error")
}

func testStation_Disconnect(t *testing.T) {
	t.Parallel()

	var called bool
	s := &Station{call: &fakeCaller{
		callFn: func(ctx context.Context, iface, method string, args ...interface{}) ([]interface{}, error) {
			called = true
			require.Equal(t, IwdStationIface, iface)
			require.Equal(t, "Disconnect", method)
			return nil, nil
		},
	}}

	require.NoError(t, s.Disconnect(context.Background()))
	require.True(t, called)
}

func testStation_Disconnect_Err(t *testing.T) {
	t.Parallel()

	s := &Station{call: &fakeCaller{
		callFn: func(ctx context.Context, iface, method string, args ...interface{}) ([]interface{}, error) {
			return nil, fmt.Errorf("dbus failure")
		},
	}}

	err := s.Disconnect(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "dbus failure")
}

func testStation_ConnectHiddenNetwork(t *testing.T) {
	t.Parallel()

	var gotName string
	s := &Station{call: &fakeCaller{
		callFn: func(ctx context.Context, iface, method string, args ...interface{}) ([]interface{}, error) {
			require.Equal(t, IwdStationIface, iface)
			require.Equal(t, "ConnectHiddenNetwork", method)
			require.Len(t, args, 1)
			gotName, _ = args[0].(string)
			return nil, nil
		},
	}}

	require.NoError(t, s.ConnectHiddenNetwork(context.Background(), "HiddenNet"))
	require.Equal(t, "HiddenNet", gotName)
}

func testStation_ConnectHiddenNetwork_NotFoundMatchable(t *testing.T) {
	t.Parallel()

	s := &Station{call: &fakeCaller{
		callFn: func(ctx context.Context, iface, method string, args ...interface{}) ([]interface{}, error) {
			return nil, dbus.Error{Name: IwdErrorNotFound, Body: []interface{}{"no such hidden network"}}
		},
	}}

	err := s.ConnectHiddenNetwork(context.Background(), "Nope")
	require.Error(t, err)
	require.ErrorIs(t, err, ErrNotFound)
}

func testStation_GetHiddenAccessPoints(t *testing.T) {
	t.Parallel()

	s := &Station{call: &fakeCaller{
		callFn: func(ctx context.Context, iface, method string, args ...interface{}) ([]interface{}, error) {
			require.Equal(t, IwdStationIface, iface)
			require.Equal(t, "GetHiddenAccessPoints", method)
			// a(sns): array of (address, int16 signal, type).
			return []interface{}{
				[][]interface{}{
					{"aa:bb:cc:dd:ee:ff", int16(-6000), "psk"},
					{"11:22:33:44:55:66", int16(-7800), "open"},
				},
			}, nil
		},
	}}

	got, err := s.GetHiddenAccessPoints(context.Background())
	require.NoError(t, err)
	require.Equal(t, []HiddenAccessPoint{
		{Address: "aa:bb:cc:dd:ee:ff", SignalStrength: -6000, Type: NetworkTypePSK},
		{Address: "11:22:33:44:55:66", SignalStrength: -7800, Type: NetworkTypeOpen},
	}, got)
}

func testStation_GetHiddenAccessPoints_Empty(t *testing.T) {
	t.Parallel()

	s := &Station{call: &fakeCaller{
		callFn: func(ctx context.Context, iface, method string, args ...interface{}) ([]interface{}, error) {
			return []interface{}{[][]interface{}{}}, nil
		},
	}}

	got, err := s.GetHiddenAccessPoints(context.Background())
	require.NoError(t, err)
	require.Empty(t, got)
}

func testStation_GetHiddenAccessPoints_BadType(t *testing.T) {
	t.Parallel()

	s := &Station{call: &fakeCaller{
		callFn: func(ctx context.Context, iface, method string, args ...interface{}) ([]interface{}, error) {
			return []interface{}{
				[][]interface{}{
					{"aa:bb:cc:dd:ee:ff", int16(-6000), "bogus"},
				},
			}, nil
		},
	}}

	_, err := s.GetHiddenAccessPoints(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid network type")
}

func testStation_GetHiddenAccessPoints_Err(t *testing.T) {
	t.Parallel()

	s := &Station{call: &fakeCaller{
		callFn: func(ctx context.Context, iface, method string, args ...interface{}) ([]interface{}, error) {
			return nil, fmt.Errorf("dbus failure")
		},
	}}

	_, err := s.GetHiddenAccessPoints(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "dbus failure")
}

func testStation_SubscribePropertiesChanged(t *testing.T) {
	t.Parallel()

	fake := newFakeSignalSource(t)
	ctx := context.Background()
	station := &Station{signals: fake}

	var recv StationPropertiesChanged
	fired := make(chan struct{}, 1)

	_, err := station.SubscribePropertiesChanged(ctx, func(changed StationPropertiesChanged) {
		recv = changed
		fired <- struct{}{}
	})
	require.NoError(t, err)

	changed := map[string]dbus.Variant{
		"State":    dbus.MakeVariant("connecting"),
		"Scanning": dbus.MakeVariant(true),
	}
	invalid := []string{"ConnectedNetwork"}
	fake.emit("org.freedesktop.DBus.Properties", "PropertiesChanged", IwdStationIface, changed, invalid)

	requireFired(t, fired)
	require.Equal(t, changed, recv.Changed)
	require.Equal(t, invalid, recv.Invalidated)
}

func testStation_SubscribeStateChanged(t *testing.T) {
	t.Parallel()

	fake := newFakeSignalSource(t)
	ctx := context.Background()
	station := &Station{signals: fake}

	var recv StationState
	fired := make(chan struct{}, 1)

	_, err := station.SubscribeStateChanged(ctx, func(s StationState) {
		recv = s
		fired <- struct{}{}
	})
	require.NoError(t, err)

	changed := map[string]dbus.Variant{
		"State": dbus.MakeVariant("roaming"),
	}
	fake.emit("org.freedesktop.DBus.Properties", "PropertiesChanged", IwdStationIface, changed, nil)

	requireFired(t, fired)
	require.Equal(t, StationStateRoaming, recv)
}

func testStation_SubscribeStateChanged_IgnoresInvalid(t *testing.T) {
	t.Parallel()

	fake := newFakeSignalSource(t)
	ctx := context.Background()
	station := &Station{signals: fake}

	fired := make(chan struct{}, 1)

	_, err := station.SubscribeStateChanged(ctx, func(StationState) {
		fired <- struct{}{}
	})
	require.NoError(t, err)

	// An unparsable state must not reach the typed handler.
	fake.emit("org.freedesktop.DBus.Properties", "PropertiesChanged", IwdStationIface, map[string]dbus.Variant{
		"State": dbus.MakeVariant("bad-state"),
	}, nil)

	requireNotFired(t, fired)
}

func testStation_SubscribeScanningChanged(t *testing.T) {
	t.Parallel()

	fake := newFakeSignalSource(t)
	ctx := context.Background()
	station := &Station{signals: fake}

	var recv bool
	fired := make(chan struct{}, 1)

	_, err := station.SubscribeScanningChanged(ctx, func(v bool) {
		recv = v
		fired <- struct{}{}
	})
	require.NoError(t, err)

	changed := map[string]dbus.Variant{
		"Scanning": dbus.MakeVariant(true),
	}
	fake.emit("org.freedesktop.DBus.Properties", "PropertiesChanged", IwdStationIface, changed, nil)

	requireFired(t, fired)
	require.True(t, recv)
}

func testStation_SubscribeScanningChanged_IgnoresUnrelated(t *testing.T) {
	t.Parallel()

	fake := newFakeSignalSource(t)
	ctx := context.Background()
	station := &Station{signals: fake}

	fired := make(chan struct{}, 1)

	_, err := station.SubscribeScanningChanged(ctx, func(bool) {
		fired <- struct{}{}
	})
	require.NoError(t, err)

	// Wrong interface: a device PropertiesChanged must not reach a station handler.
	fake.emit("org.freedesktop.DBus.Properties", "PropertiesChanged", IwdDeviceIface, map[string]dbus.Variant{
		"Scanning": dbus.MakeVariant(true),
	}, nil)

	requireNotFired(t, fired)
}

func testStation_SubscribeScanningChanged_Unsubscribe(t *testing.T) {
	t.Parallel()

	fake := newFakeSignalSource(t)
	ctx := context.Background()
	station := &Station{signals: fake}

	fired := make(chan struct{}, 2)

	unsubscribe, err := station.SubscribeScanningChanged(ctx, func(bool) {
		fired <- struct{}{}
	})
	require.NoError(t, err)
	require.NotNil(t, unsubscribe)

	changed := map[string]dbus.Variant{
		"Scanning": dbus.MakeVariant(true),
	}
	fake.emit("org.freedesktop.DBus.Properties", "PropertiesChanged", IwdStationIface, changed, nil)
	requireFired(t, fired)

	require.NoError(t, unsubscribe.Unsubscribe())

	fake.emit("org.freedesktop.DBus.Properties", "PropertiesChanged", IwdStationIface, changed, nil)
	requireNotFired(t, fired)
}

func testStation_FirehoseReceivesAll(t *testing.T) {
	fake := newFakeSignalSource(t)
	station := &Station{signals: fake}

	var recv FirehoseSignal
	fired := make(chan struct{}, 1)

	err := station.Firehose(context.Background(), func(s FirehoseSignal) {
		recv = s
		fired <- struct{}{}
	})
	require.NoError(t, err)

	fake.emit(
		"org.freedesktop.DBus.Properties",
		"PropertiesChanged",
		IwdStationIface,
		map[string]dbus.Variant{"Scanning": dbus.MakeVariant(true)},
		nil,
	)

	requireFired(t, fired)
	require.Equal(t, "org.freedesktop.DBus.Properties", recv.Interface)
	require.Equal(t, "PropertiesChanged", recv.Member)
}

func newGetAllStation(fn func(ctx context.Context, iface string) (map[string]dbus.Variant, error)) *Station {
	return &Station{call: &fakeCaller{getAllFn: fn}}
}

// testStation_NoIntro checks that every init-guarded method reports a clean
// "not initialized" error (rather than panicking) when the Station has no caller.
func testStation_NoIntro(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	for _, tc := range []struct {
		name string
		call func(*Station) error
	}{
		{"GetState", func(s *Station) error { _, err := s.GetState(ctx); return err }},
		{"GetScanning", func(s *Station) error { _, err := s.GetScanning(ctx); return err }},
		{"GetConnectedNetwork", func(s *Station) error { _, err := s.GetConnectedNetwork(ctx); return err }},
		{"GetConnectedAccessPoint", func(s *Station) error { _, err := s.GetConnectedAccessPoint(ctx); return err }},
		{"GetAffinities", func(s *Station) error { _, err := s.GetAffinities(ctx); return err }},
		{"GetProperties", func(s *Station) error { _, err := s.GetProperties(ctx); return err }},
		{"Scan", func(s *Station) error { return s.Scan(ctx) }},
		{"GetOrderedNetworks", func(s *Station) error { _, err := s.GetOrderedNetworks(ctx); return err }},
		{"SetAffinities", func(s *Station) error { return s.SetAffinities(ctx, []string{"/x"}) }},
		{"Disconnect", func(s *Station) error { return s.Disconnect(ctx) }},
		{"ConnectHiddenNetwork", func(s *Station) error { return s.ConnectHiddenNetwork(ctx, "x") }},
		{"GetHiddenAccessPoints", func(s *Station) error { _, err := s.GetHiddenAccessPoints(ctx); return err }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.call(&Station{call: nil})
			require.Error(t, err)
			require.Contains(t, err.Error(), "station is not initialized")
		})
	}
}
