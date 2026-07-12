//go:build unit

package iwdbus

import (
	"context"
	"fmt"
	"testing"

	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/require"
)

func TestAccessPoint_Iwdbus(t *testing.T) {
	t.Parallel()

	t.Run("Getters", func(t *testing.T) {
		t.Parallel()
		t.Run("AccessPoint_GetStarted", testAccessPoint_GetStarted)
		t.Run("AccessPoint_GetScanning", testAccessPoint_GetScanning)
		t.Run("AccessPoint_GetScanning_Absent", testAccessPoint_GetScanning_Absent)
		t.Run("AccessPoint_GetName", testAccessPoint_GetName)
		t.Run("AccessPoint_GetName_Absent", testAccessPoint_GetName_Absent)
		t.Run("AccessPoint_GetFrequency", testAccessPoint_GetFrequency)
		t.Run("AccessPoint_GetFrequency_Absent", testAccessPoint_GetFrequency_Absent)
		t.Run("AccessPoint_GetPairwiseCiphers", testAccessPoint_GetPairwiseCiphers)
		t.Run("AccessPoint_GetPairwiseCiphers_Absent", testAccessPoint_GetPairwiseCiphers_Absent)
		t.Run("AccessPoint_GetGroupCipher", testAccessPoint_GetGroupCipher)
		t.Run("AccessPoint_GetGroupCipher_Absent", testAccessPoint_GetGroupCipher_Absent)
		t.Run("AccessPoint_GetterWrongTypes", testAccessPoint_GetterWrongTypes)
	})

	t.Run("ParseHelpers", func(t *testing.T) {
		t.Parallel()
		t.Run("AccessPoint_ParseOptionalString", testAccessPoint_ParseOptionalString)
		t.Run("AccessPoint_ParseFrequency", testAccessPoint_ParseFrequency)
		t.Run("AccessPoint_ParseCiphers", testAccessPoint_ParseCiphers)
	})

	t.Run("Properties", func(t *testing.T) {
		t.Parallel()
		t.Run("AccessPoint_GetProperties", testAccessPoint_GetProperties)
		t.Run("AccessPoint_GetProperties_Minimal", testAccessPoint_GetProperties_Minimal)
		t.Run("AccessPoint_GetProperties_Errors", testAccessPoint_GetProperties_Errors)
	})

	t.Run("Operations", func(t *testing.T) {
		t.Parallel()
		t.Run("AccessPoint_Start", testAccessPoint_Start)
		t.Run("AccessPoint_Start_Err", testAccessPoint_Start_Err)
		t.Run("AccessPoint_Start_AlreadyExistsMatchable", testAccessPoint_Start_AlreadyExistsMatchable)
		t.Run("AccessPoint_StartProfile", testAccessPoint_StartProfile)
		t.Run("AccessPoint_StartProfile_NotFoundMatchable", testAccessPoint_StartProfile_NotFoundMatchable)
		t.Run("AccessPoint_Stop", testAccessPoint_Stop)
		t.Run("AccessPoint_Stop_Err", testAccessPoint_Stop_Err)
		t.Run("AccessPoint_Scan", testAccessPoint_Scan)
		t.Run("AccessPoint_Scan_InProgressMatchable", testAccessPoint_Scan_InProgressMatchable)
		t.Run("AccessPoint_GetOrderedNetworks", testAccessPoint_GetOrderedNetworks)
		t.Run("AccessPoint_GetOrderedNetworks_UnknownSecurity", testAccessPoint_GetOrderedNetworks_UnknownSecurity)
		t.Run("AccessPoint_GetOrderedNetworks_Empty", testAccessPoint_GetOrderedNetworks_Empty)
		t.Run("AccessPoint_GetOrderedNetworks_BadType", testAccessPoint_GetOrderedNetworks_BadType)
		t.Run("AccessPoint_GetOrderedNetworks_Err", testAccessPoint_GetOrderedNetworks_Err)
	})

	t.Run("NotInitialized", testAccessPoint_NoIntro)

	t.Run("Subscribe", func(t *testing.T) {
		t.Parallel()
		t.Run("AccessPoint_SubscribePropertiesChanged", testAccessPoint_SubscribePropertiesChanged)
		t.Run("AccessPoint_SubscribeStartedChanged", testAccessPoint_SubscribeStartedChanged)
		t.Run("AccessPoint_SubscribeStartedChanged_IgnoresUnrelated", testAccessPoint_SubscribeStartedChanged_IgnoresUnrelated)
		t.Run("AccessPoint_SubscribeScanningChanged", testAccessPoint_SubscribeScanningChanged)
		t.Run("AccessPoint_SubscribeNilCallback", testAccessPoint_SubscribeNilCallback)
		t.Run("AccessPoint_SubscribePropertiesChanged_IgnoresMalformed", testAccessPoint_SubscribePropertiesChanged_IgnoresMalformed)
		t.Run("AccessPoint_Firehose", testAccessPoint_Firehose)
		t.Run("AccessPoint_Firehose_NilCallback", testAccessPoint_Firehose_NilCallback)
	})
}

func testAccessPoint_GetStarted(t *testing.T) {
	t.Parallel()
	a := &AccessPoint{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			require.Equal(t, IwdAccessPointIface, iface)
			require.Equal(t, "Started", prop)
			return true, nil
		},
	}}
	started, err := a.GetStarted(context.Background())
	require.NoError(t, err)
	require.True(t, started)
}

func testAccessPoint_GetScanning(t *testing.T) {
	t.Parallel()
	a := &AccessPoint{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			require.Equal(t, "Scanning", prop)
			return false, nil
		},
	}}
	scanning, err := a.GetScanning(context.Background())
	require.NoError(t, err)
	require.False(t, scanning)
}

func testAccessPoint_GetScanning_Absent(t *testing.T) {
	t.Parallel()
	// A stopped AP omits Scanning; the getter reports it as (false, nil) rather
	// than surfacing iwd's property-read error.
	a := &AccessPoint{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			require.Equal(t, "Scanning", prop)
			return nil, fmt.Errorf("Getting property value failed")
		},
	}}
	scanning, err := a.GetScanning(context.Background())
	require.NoError(t, err)
	require.False(t, scanning)
}

func testAccessPoint_GetName(t *testing.T) {
	t.Parallel()
	a := &AccessPoint{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			require.Equal(t, "Name", prop)
			return "MyAP", nil
		},
	}}
	name, err := a.GetName(context.Background())
	require.NoError(t, err)
	require.NotNil(t, name)
	require.Equal(t, "MyAP", *name)
}

func testAccessPoint_GetName_Absent(t *testing.T) {
	t.Parallel()
	// iwd omits Name while the AP is not running; report it as (nil, nil).
	a := &AccessPoint{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			return nil, fmt.Errorf("Getting property value failed")
		},
	}}
	name, err := a.GetName(context.Background())
	require.NoError(t, err)
	require.Nil(t, name)
}

func testAccessPoint_GetFrequency(t *testing.T) {
	t.Parallel()
	a := &AccessPoint{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			require.Equal(t, "Frequency", prop)
			return uint32(2412), nil
		},
	}}
	freq, err := a.GetFrequency(context.Background())
	require.NoError(t, err)
	require.NotNil(t, freq)
	require.Equal(t, uint32(2412), *freq)
}

func testAccessPoint_GetFrequency_Absent(t *testing.T) {
	t.Parallel()
	a := &AccessPoint{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			return nil, fmt.Errorf("Getting property value failed")
		},
	}}
	freq, err := a.GetFrequency(context.Background())
	require.NoError(t, err)
	require.Nil(t, freq)
}

func testAccessPoint_GetPairwiseCiphers(t *testing.T) {
	t.Parallel()
	a := &AccessPoint{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			require.Equal(t, "PairwiseCiphers", prop)
			return []string{"CCMP", "TKIP"}, nil
		},
	}}
	ciphers, err := a.GetPairwiseCiphers(context.Background())
	require.NoError(t, err)
	require.Equal(t, []string{"CCMP", "TKIP"}, ciphers)
}

func testAccessPoint_GetPairwiseCiphers_Absent(t *testing.T) {
	t.Parallel()
	a := &AccessPoint{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			return nil, fmt.Errorf("Getting property value failed")
		},
	}}
	ciphers, err := a.GetPairwiseCiphers(context.Background())
	require.NoError(t, err)
	require.Nil(t, ciphers)
}

func testAccessPoint_GetGroupCipher(t *testing.T) {
	t.Parallel()
	a := &AccessPoint{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			require.Equal(t, "GroupCipher", prop)
			return "CCMP", nil
		},
	}}
	group, err := a.GetGroupCipher(context.Background())
	require.NoError(t, err)
	require.NotNil(t, group)
	require.Equal(t, "CCMP", *group)
}

func testAccessPoint_GetGroupCipher_Absent(t *testing.T) {
	t.Parallel()
	// iwd omits GroupCipher while the AP is not running; report it as (nil, nil).
	a := &AccessPoint{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			require.Equal(t, "GroupCipher", prop)
			return nil, fmt.Errorf("Getting property value failed")
		},
	}}
	group, err := a.GetGroupCipher(context.Background())
	require.NoError(t, err)
	require.Nil(t, group)
}

func testAccessPoint_GetterWrongTypes(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		badValue interface{}
		call     func(context.Context, *AccessPoint) error
		wantHint string
	}{
		{"Started", 1, func(ctx context.Context, a *AccessPoint) error { _, err := a.GetStarted(ctx); return err }, "expected bool"},
		{"Scanning", "no", func(ctx context.Context, a *AccessPoint) error { _, err := a.GetScanning(ctx); return err }, "expected bool"},
		{"Name", 5, func(ctx context.Context, a *AccessPoint) error { _, err := a.GetName(ctx); return err }, "expected string"},
		{"Frequency", "hz", func(ctx context.Context, a *AccessPoint) error { _, err := a.GetFrequency(ctx); return err }, "expected uint32"},
		{"PairwiseCiphers", "x", func(ctx context.Context, a *AccessPoint) error { _, err := a.GetPairwiseCiphers(ctx); return err }, "expected string array"},
		{"GroupCipher", 9, func(ctx context.Context, a *AccessPoint) error { _, err := a.GetGroupCipher(ctx); return err }, "expected string"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a := &AccessPoint{call: &fakeCaller{
				getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
					return tc.badValue, nil
				},
			}}
			err := tc.call(context.Background(), a)
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.wantHint)
		})
	}
}

func testAccessPoint_GetProperties(t *testing.T) {
	t.Parallel()
	a := &AccessPoint{call: &fakeCaller{
		getAllFn: func(ctx context.Context, iface string) (map[string]dbus.Variant, error) {
			require.Equal(t, IwdAccessPointIface, iface)
			return map[string]dbus.Variant{
				"Started":         dbus.MakeVariant(true),
				"Scanning":        dbus.MakeVariant(false),
				"Name":            dbus.MakeVariant("MyAP"),
				"Frequency":       dbus.MakeVariant(uint32(5180)),
				"PairwiseCiphers": dbus.MakeVariant([]string{"CCMP"}),
				"GroupCipher":     dbus.MakeVariant("CCMP"),
			}, nil
		},
	}}
	props, err := a.GetProperties(context.Background())
	require.NoError(t, err)
	require.True(t, props.Started)
	require.False(t, props.Scanning)
	require.Equal(t, "MyAP", *props.Name)
	require.Equal(t, uint32(5180), *props.Frequency)
	require.Equal(t, []string{"CCMP"}, props.PairwiseCiphers)
	require.Equal(t, "CCMP", *props.GroupCipher)
}

func testAccessPoint_GetProperties_Minimal(t *testing.T) {
	t.Parallel()
	// A stopped AP reports only Started; iwd omits Scanning and the other
	// optionals, so Scanning must collapse to false rather than erroring. This is
	// the hardware-observed shape (a Raspberry Pi reported exactly this and the
	// original strict parser failed on the absent Scanning).
	a := &AccessPoint{call: &fakeCaller{
		getAllFn: func(ctx context.Context, iface string) (map[string]dbus.Variant, error) {
			return map[string]dbus.Variant{
				"Started": dbus.MakeVariant(false),
			}, nil
		},
	}}
	props, err := a.GetProperties(context.Background())
	require.NoError(t, err)
	require.False(t, props.Started)
	require.False(t, props.Scanning)
	require.Nil(t, props.Name)
	require.Nil(t, props.Frequency)
	require.Nil(t, props.PairwiseCiphers)
	require.Nil(t, props.GroupCipher)
}

func testAccessPoint_GetProperties_Errors(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		all  map[string]dbus.Variant
		hint string
	}{
		{"missing Started", map[string]dbus.Variant{"Scanning": dbus.MakeVariant(false)}, "Started"},
		{"bad Started", map[string]dbus.Variant{"Started": dbus.MakeVariant("x"), "Scanning": dbus.MakeVariant(false)}, "expected bool"},
		{"bad Scanning", map[string]dbus.Variant{"Started": dbus.MakeVariant(true), "Scanning": dbus.MakeVariant("x")}, "expected bool"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a := &AccessPoint{call: &fakeCaller{
				getAllFn: func(ctx context.Context, iface string) (map[string]dbus.Variant, error) {
					return tc.all, nil
				},
			}}
			_, err := a.GetProperties(context.Background())
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.hint)
		})
	}
}

func testAccessPoint_Start(t *testing.T) {
	t.Parallel()
	var called bool
	a := &AccessPoint{call: &fakeCaller{
		callFn: func(ctx context.Context, iface, method string, args ...interface{}) ([]interface{}, error) {
			called = true
			require.Equal(t, IwdAccessPointIface, iface)
			require.Equal(t, "Start", method)
			require.Equal(t, []interface{}{"MyAP", "s3cretpass"}, args)
			return nil, nil
		},
	}}
	require.NoError(t, a.Start(context.Background(), "MyAP", "s3cretpass"))
	require.True(t, called)
}

func testAccessPoint_Start_Err(t *testing.T) {
	t.Parallel()
	a := &AccessPoint{call: &fakeCaller{
		callFn: func(ctx context.Context, iface, method string, args ...interface{}) ([]interface{}, error) {
			return nil, fmt.Errorf("dbus failure")
		},
	}}
	err := a.Start(context.Background(), "MyAP", "s3cretpass")
	require.Error(t, err)
	require.Contains(t, err.Error(), "dbus failure")
}

func testAccessPoint_Start_AlreadyExistsMatchable(t *testing.T) {
	t.Parallel()
	a := &AccessPoint{call: &fakeCaller{
		callFn: func(ctx context.Context, iface, method string, args ...interface{}) ([]interface{}, error) {
			return nil, dbus.Error{Name: IwdErrorAlreadyExists, Body: []interface{}{"already running"}}
		},
	}}
	err := a.Start(context.Background(), "MyAP", "s3cretpass")
	require.Error(t, err)
	require.ErrorIs(t, err, ErrAlreadyExists)
	require.ErrorIs(t, err, ErrDBusMethod)
}

func testAccessPoint_StartProfile(t *testing.T) {
	t.Parallel()
	var called bool
	a := &AccessPoint{call: &fakeCaller{
		callFn: func(ctx context.Context, iface, method string, args ...interface{}) ([]interface{}, error) {
			called = true
			require.Equal(t, "StartProfile", method)
			require.Equal(t, []interface{}{"HomeAP"}, args)
			return nil, nil
		},
	}}
	require.NoError(t, a.StartProfile(context.Background(), "HomeAP"))
	require.True(t, called)
}

func testAccessPoint_StartProfile_NotFoundMatchable(t *testing.T) {
	t.Parallel()
	a := &AccessPoint{call: &fakeCaller{
		callFn: func(ctx context.Context, iface, method string, args ...interface{}) ([]interface{}, error) {
			return nil, dbus.Error{Name: IwdErrorNotFound, Body: []interface{}{"no such profile"}}
		},
	}}
	err := a.StartProfile(context.Background(), "Missing")
	require.Error(t, err)
	require.ErrorIs(t, err, ErrNotFound)
}

func testAccessPoint_Stop(t *testing.T) {
	t.Parallel()
	var called bool
	a := &AccessPoint{call: &fakeCaller{
		callFn: func(ctx context.Context, iface, method string, args ...interface{}) ([]interface{}, error) {
			called = true
			require.Equal(t, "Stop", method)
			require.Empty(t, args)
			return nil, nil
		},
	}}
	require.NoError(t, a.Stop(context.Background()))
	require.True(t, called)
}

func testAccessPoint_Stop_Err(t *testing.T) {
	t.Parallel()
	a := &AccessPoint{call: &fakeCaller{
		callFn: func(ctx context.Context, iface, method string, args ...interface{}) ([]interface{}, error) {
			return nil, fmt.Errorf("dbus failure")
		},
	}}
	require.Error(t, a.Stop(context.Background()))
}

func testAccessPoint_Scan(t *testing.T) {
	t.Parallel()
	var called bool
	a := &AccessPoint{call: &fakeCaller{
		callFn: func(ctx context.Context, iface, method string, args ...interface{}) ([]interface{}, error) {
			called = true
			require.Equal(t, "Scan", method)
			return nil, nil
		},
	}}
	require.NoError(t, a.Scan(context.Background()))
	require.True(t, called)
}

func testAccessPoint_Scan_InProgressMatchable(t *testing.T) {
	t.Parallel()
	// iwd reports a busy AP scan as net.connman.iwd.InProgress.
	a := &AccessPoint{call: &fakeCaller{
		callFn: func(ctx context.Context, iface, method string, args ...interface{}) ([]interface{}, error) {
			return nil, dbus.Error{Name: IwdErrorInProgress, Body: []interface{}{"scan in progress"}}
		},
	}}
	err := a.Scan(context.Background())
	require.Error(t, err)
	require.ErrorIs(t, err, ErrInProgress)
}

func testAccessPoint_GetOrderedNetworks(t *testing.T) {
	t.Parallel()
	a := &AccessPoint{call: &fakeCaller{
		callFn: func(ctx context.Context, iface, method string, args ...interface{}) ([]interface{}, error) {
			require.Equal(t, "GetOrderedNetworks", method)
			// aa{sv}: array of {Name, SignalStrength, Security} dicts.
			return []interface{}{
				[]map[string]dbus.Variant{
					{
						"Name":           dbus.MakeVariant("OpenNet"),
						"SignalStrength": dbus.MakeVariant(int16(-6000)),
						"Type":           dbus.MakeVariant("open"),
					},
					{
						"Name":           dbus.MakeVariant("SecuredNet"),
						"SignalStrength": dbus.MakeVariant(int16(-7200)),
						"Type":           dbus.MakeVariant("psk"),
					},
				},
			}, nil
		},
	}}
	got, err := a.GetOrderedNetworks(context.Background())
	require.NoError(t, err)
	require.Equal(t, []AccessPointOrderedNetwork{
		{Name: "OpenNet", SignalStrength: -6000, Type: NetworkTypeOpen},
		{Name: "SecuredNet", SignalStrength: -7200, Type: NetworkTypePSK},
	}, got)
}

func testAccessPoint_GetOrderedNetworks_UnknownSecurity(t *testing.T) {
	t.Parallel()
	// A neighbor whose Security iwd cannot classify (empty or unrecognized) must
	// parse to NetworkTypeUnknown, not fail the whole reply — the hardware bug
	// where one unclassifiable neighbor broke `access-point networks`.
	a := &AccessPoint{call: &fakeCaller{
		callFn: func(ctx context.Context, iface, method string, args ...interface{}) ([]interface{}, error) {
			return []interface{}{
				[]map[string]dbus.Variant{
					{
						"Name":           dbus.MakeVariant("OpenNet"),
						"SignalStrength": dbus.MakeVariant(int16(-6000)),
						"Type":           dbus.MakeVariant("open"),
					},
					{
						"Name":           dbus.MakeVariant("EmptyNet"),
						"SignalStrength": dbus.MakeVariant(int16(-8100)),
						"Type":           dbus.MakeVariant(""),
					},
					{
						"Name":           dbus.MakeVariant("WeirdNet"),
						"SignalStrength": dbus.MakeVariant(int16(-8200)),
						"Type":           dbus.MakeVariant("wpa9000"),
					},
				},
			}, nil
		},
	}}
	got, err := a.GetOrderedNetworks(context.Background())
	require.NoError(t, err)
	require.Equal(t, []AccessPointOrderedNetwork{
		{Name: "OpenNet", SignalStrength: -6000, Type: NetworkTypeOpen},
		{Name: "EmptyNet", SignalStrength: -8100, Type: NetworkTypeUnknown},
		{Name: "WeirdNet", SignalStrength: -8200, Type: NetworkTypeUnknown},
	}, got)
}

func testAccessPoint_GetOrderedNetworks_Empty(t *testing.T) {
	t.Parallel()
	a := &AccessPoint{call: &fakeCaller{
		callFn: func(ctx context.Context, iface, method string, args ...interface{}) ([]interface{}, error) {
			return []interface{}{[]map[string]dbus.Variant{}}, nil
		},
	}}
	got, err := a.GetOrderedNetworks(context.Background())
	require.NoError(t, err)
	require.Empty(t, got)
}

func testAccessPoint_GetOrderedNetworks_BadType(t *testing.T) {
	t.Parallel()
	a := &AccessPoint{call: &fakeCaller{
		callFn: func(ctx context.Context, iface, method string, args ...interface{}) ([]interface{}, error) {
			return []interface{}{
				[]map[string]dbus.Variant{
					{"Name": dbus.MakeVariant("Bad"), "SignalStrength": dbus.MakeVariant("not-int16")},
				},
			}, nil
		},
	}}
	_, err := a.GetOrderedNetworks(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "expected int16")
}

func testAccessPoint_GetOrderedNetworks_Err(t *testing.T) {
	t.Parallel()
	a := &AccessPoint{call: &fakeCaller{
		callFn: func(ctx context.Context, iface, method string, args ...interface{}) ([]interface{}, error) {
			return nil, dbus.Error{Name: IwdErrorNotAvailable, Body: []interface{}{"no scan data"}}
		},
	}}
	_, err := a.GetOrderedNetworks(context.Background())
	require.Error(t, err)
	require.ErrorIs(t, err, ErrNotAvailable)
}

func testAccessPoint_NoIntro(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	for _, tc := range []struct {
		name string
		call func(*AccessPoint) error
	}{
		{"GetStarted", func(a *AccessPoint) error { _, err := a.GetStarted(ctx); return err }},
		{"GetScanning", func(a *AccessPoint) error { _, err := a.GetScanning(ctx); return err }},
		{"GetName", func(a *AccessPoint) error { _, err := a.GetName(ctx); return err }},
		{"GetFrequency", func(a *AccessPoint) error { _, err := a.GetFrequency(ctx); return err }},
		{"GetPairwiseCiphers", func(a *AccessPoint) error { _, err := a.GetPairwiseCiphers(ctx); return err }},
		{"GetGroupCipher", func(a *AccessPoint) error { _, err := a.GetGroupCipher(ctx); return err }},
		{"GetProperties", func(a *AccessPoint) error { _, err := a.GetProperties(ctx); return err }},
		{"Start", func(a *AccessPoint) error { return a.Start(ctx, "x", "12345678") }},
		{"StartProfile", func(a *AccessPoint) error { return a.StartProfile(ctx, "x") }},
		{"Stop", func(a *AccessPoint) error { return a.Stop(ctx) }},
		{"Scan", func(a *AccessPoint) error { return a.Scan(ctx) }},
		{"GetOrderedNetworks", func(a *AccessPoint) error { _, err := a.GetOrderedNetworks(ctx); return err }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.call(&AccessPoint{call: nil})
			require.Error(t, err)
			require.Contains(t, err.Error(), "access point is not initialized")
		})
	}
}

func testAccessPoint_SubscribePropertiesChanged(t *testing.T) {
	t.Parallel()
	fake := newFakeSignalSource(t)
	a := &AccessPoint{signals: fake}

	var recv AccessPointPropertiesChanged
	fired := make(chan struct{}, 1)
	_, err := a.SubscribePropertiesChanged(context.Background(), func(changed AccessPointPropertiesChanged) {
		recv = changed
		fired <- struct{}{}
	})
	require.NoError(t, err)

	changed := map[string]dbus.Variant{"Started": dbus.MakeVariant(true), "Name": dbus.MakeVariant("MyAP")}
	invalid := []string{"Frequency"}
	fake.emit("org.freedesktop.DBus.Properties", "PropertiesChanged", IwdAccessPointIface, changed, invalid)

	requireFired(t, fired)
	require.Equal(t, changed, recv.Changed)
	require.Equal(t, invalid, recv.Invalidated)
}

func testAccessPoint_SubscribeStartedChanged(t *testing.T) {
	t.Parallel()
	fake := newFakeSignalSource(t)
	a := &AccessPoint{signals: fake}

	got := make(chan bool, 1)
	_, err := a.SubscribeStartedChanged(context.Background(), func(started bool) { got <- started })
	require.NoError(t, err)

	fake.emit("org.freedesktop.DBus.Properties", "PropertiesChanged", IwdAccessPointIface,
		map[string]dbus.Variant{"Started": dbus.MakeVariant(true)}, []string{})

	require.True(t, <-got)
}

func testAccessPoint_SubscribeStartedChanged_IgnoresUnrelated(t *testing.T) {
	t.Parallel()
	fake := newFakeSignalSource(t)
	a := &AccessPoint{signals: fake}

	fired := make(chan struct{}, 1)
	_, err := a.SubscribeStartedChanged(context.Background(), func(bool) { fired <- struct{}{} })
	require.NoError(t, err)

	// A change that does not include Started must not fire the callback.
	fake.emit("org.freedesktop.DBus.Properties", "PropertiesChanged", IwdAccessPointIface,
		map[string]dbus.Variant{"Scanning": dbus.MakeVariant(true)}, []string{})

	select {
	case <-fired:
		t.Fatal("callback fired for an unrelated property change")
	default:
	}
}

func testAccessPoint_SubscribeScanningChanged(t *testing.T) {
	t.Parallel()
	fake := newFakeSignalSource(t)
	a := &AccessPoint{signals: fake}

	got := make(chan bool, 1)
	_, err := a.SubscribeScanningChanged(context.Background(), func(scanning bool) { got <- scanning })
	require.NoError(t, err)

	fake.emit("org.freedesktop.DBus.Properties", "PropertiesChanged", IwdAccessPointIface,
		map[string]dbus.Variant{"Scanning": dbus.MakeVariant(true)}, []string{})

	require.True(t, <-got)
}

func testAccessPoint_SubscribeNilCallback(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		call func(*AccessPoint) error
	}{
		{"PropertiesChanged", func(a *AccessPoint) error {
			_, err := a.SubscribePropertiesChanged(context.Background(), nil)
			return err
		}},
		{"StartedChanged", func(a *AccessPoint) error {
			_, err := a.SubscribeStartedChanged(context.Background(), nil)
			return err
		}},
		{"ScanningChanged", func(a *AccessPoint) error {
			_, err := a.SubscribeScanningChanged(context.Background(), nil)
			return err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a := &AccessPoint{signals: newFakeSignalSource(t)}
			err := tc.call(a)
			require.Error(t, err)
			require.Contains(t, err.Error(), "fn cannot be nil")
		})
	}
}

func testAccessPoint_SubscribePropertiesChanged_IgnoresMalformed(t *testing.T) {
	t.Parallel()
	fake := newFakeSignalSource(t)
	a := &AccessPoint{signals: fake}

	fired := make(chan struct{}, 1)
	_, err := a.SubscribePropertiesChanged(context.Background(), func(AccessPointPropertiesChanged) {
		fired <- struct{}{}
	})
	require.NoError(t, err)

	// A signal for a different interface, and a short/garbled body, must be
	// ignored rather than firing the callback or panicking.
	fake.emit("org.freedesktop.DBus.Properties", "PropertiesChanged", "net.connman.iwd.Station",
		map[string]dbus.Variant{"State": dbus.MakeVariant("connected")}, []string{})
	fake.emit("org.freedesktop.DBus.Properties", "PropertiesChanged", IwdAccessPointIface)

	select {
	case <-fired:
		t.Fatal("callback fired for a malformed or unrelated signal")
	default:
	}
}

func testAccessPoint_Firehose(t *testing.T) {
	t.Parallel()
	fake := newFakeSignalSource(t)
	a := &AccessPoint{signals: fake}

	var recv FirehoseSignal
	fired := make(chan struct{}, 1)
	err := a.Firehose(context.Background(), func(s FirehoseSignal) {
		recv = s
		fired <- struct{}{}
	})
	require.NoError(t, err)

	fake.emit("org.freedesktop.DBus.Properties", "PropertiesChanged", IwdAccessPointIface,
		map[string]dbus.Variant{"Started": dbus.MakeVariant(true)}, nil)

	requireFired(t, fired)
	require.Equal(t, "org.freedesktop.DBus.Properties", recv.Interface)
	require.Equal(t, "PropertiesChanged", recv.Member)
}

func testAccessPoint_Firehose_NilCallback(t *testing.T) {
	t.Parallel()
	a := &AccessPoint{signals: newFakeSignalSource(t)}
	err := a.Firehose(context.Background(), nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "fn cannot be nil")
}

func testAccessPoint_ParseOptionalString(t *testing.T) {
	t.Parallel()

	got, err := parseOptionalAccessPointString("Name", nil)
	require.NoError(t, err)
	require.Nil(t, got, "nil value yields nil pointer")

	got, err = parseOptionalAccessPointString("Name", "")
	require.NoError(t, err)
	require.Nil(t, got, "empty string yields nil pointer")

	got, err = parseOptionalAccessPointString("Name", "MyAP")
	require.NoError(t, err)
	require.Equal(t, "MyAP", *got)

	got, err = parseOptionalAccessPointString("Name", dbus.MakeVariant("Wrapped"))
	require.NoError(t, err)
	require.Equal(t, "Wrapped", *got, "a wrapped variant is unwrapped")

	_, err = parseOptionalAccessPointString("Name", 42)
	require.Error(t, err)
	require.Contains(t, err.Error(), "expected string")
}

func testAccessPoint_ParseFrequency(t *testing.T) {
	t.Parallel()

	got, err := parseAccessPointFrequency(nil)
	require.NoError(t, err)
	require.Nil(t, got)

	got, err = parseAccessPointFrequency(uint32(5180))
	require.NoError(t, err)
	require.Equal(t, uint32(5180), *got)

	got, err = parseAccessPointFrequency(dbus.MakeVariant(uint32(2412)))
	require.NoError(t, err)
	require.Equal(t, uint32(2412), *got, "a wrapped variant is unwrapped")

	_, err = parseAccessPointFrequency("2412")
	require.Error(t, err)
	require.Contains(t, err.Error(), "expected uint32")
}

func testAccessPoint_ParseCiphers(t *testing.T) {
	t.Parallel()

	got, err := parseAccessPointCiphers(nil)
	require.NoError(t, err)
	require.Nil(t, got)

	got, err = parseAccessPointCiphers([]string{"CCMP", "TKIP"})
	require.NoError(t, err)
	require.Equal(t, []string{"CCMP", "TKIP"}, got)

	got, err = parseAccessPointCiphers(dbus.MakeVariant([]string{"GCMP-256"}))
	require.NoError(t, err)
	require.Equal(t, []string{"GCMP-256"}, got, "a wrapped variant is unwrapped")

	_, err = parseAccessPointCiphers("CCMP")
	require.Error(t, err)
	require.Contains(t, err.Error(), "expected string array")
}
