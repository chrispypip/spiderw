//go:build unit

package iwdbus

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/require"
)

func TestDevice_Iwdbus(t *testing.T) {
	t.Parallel()

	t.Run("Getters", func(t *testing.T) {
		t.Parallel()
		t.Run("Device_GetName", testDevice_GetName)
		t.Run("Device_GetName_WrongType", testDevice_GetName_WrongType)
		t.Run("Device_GetName_NoIntro", testDevice_GetName_NoIntro)
		t.Run("Device_GetName_Err", testDevice_GetName_Err)
		t.Run("Device_GetNameTimeout", testDevice_GetNameTimeout)
		t.Run("Device_GetAddress", testDevice_GetAddress)
		t.Run("Device_GetAddress_WrongType", testDevice_GetAddress_WrongType)
		t.Run("Device_GetAddress_NoIntro", testDevice_GetAddress_NoIntro)
		t.Run("Device_GetPowered", testDevice_GetPowered)
		t.Run("Device_GetPowered_WrongType", testDevice_GetPowered_WrongType)
		t.Run("Device_GetPowered_NoIntro", testDevice_GetPowered_NoIntro)
		t.Run("Device_GetMode", testDevice_GetMode)
		t.Run("Device_GetMode_Invalid", testDevice_GetMode_Invalid)
		t.Run("Device_GetMode_WrongType", testDevice_GetMode_WrongType)
		t.Run("Device_GetMode_NoIntro", testDevice_GetMode_NoIntro)
		t.Run("Device_GetAdapter", testDevice_GetAdapter)
		t.Run("Device_GetAdapter_String", testDevice_GetAdapter_String)
		t.Run("Device_GetAdapter_WrongType", testDevice_GetAdapter_WrongType)
		t.Run("Device_GetAdapter_NoIntro", testDevice_GetAdapter_NoIntro)
	})

	t.Run("Set", func(t *testing.T) {
		t.Parallel()
		t.Run("Device_SetPowered", testDevice_SetPowered)
		t.Run("Device_SetPowered_Err", testDevice_SetPowered_Err)
		t.Run("Device_SetPowered_NoIntro", testDevice_SetPowered_NoIntro)
		t.Run("Device_SetMode", testDevice_SetMode)
		t.Run("Device_SetMode_Invalid", testDevice_SetMode_Invalid)
		t.Run("Device_SetMode_Err", testDevice_SetMode_Err)
		t.Run("Device_SetMode_NoIntro", testDevice_SetMode_NoIntro)
	})

	t.Run("Properties", func(t *testing.T) {
		t.Parallel()
		t.Run("Device_GetProperties", testDevice_GetProperties)
		t.Run("Device_GetProperties_Errors", testDevice_GetProperties_Errors)
		t.Run("Device_GetProperties_NoIntro", testDevice_GetProperties_NoIntro)
	})

	t.Run("Subscribe", func(t *testing.T) {
		t.Parallel()
		t.Run("Device_SubscribePropertiesChanged", testDevice_SubscribePropertiesChanged)
		t.Run("Device_SubscribePoweredChanged", testDevice_SubscribePoweredChanged)
		t.Run("Device_SubscribePoweredChanged_IgnoresUnrelated", testDevice_SubscribePoweredChanged_IgnoresUnrelated)
		t.Run("Device_SubscribePoweredChanged_Unsubscribe", testDevice_SubscribePoweredChanged_Unsubscribe)
		t.Run("Device_SubscribeModeChanged", testDevice_SubscribeModeChanged)
		t.Run("Device_SubscribeModeChanged_IgnoresInvalid", testDevice_SubscribeModeChanged_IgnoresInvalid)
	})

	t.Run("Firehose", func(t *testing.T) {
		t.Parallel()
		t.Run("Device_FirehoseReceivesAll", testDevice_FirehoseReceivesAll)
		t.Run("Device_FirehosePropertiesChanged", testDevice_FirehosePropertiesChanged)
	})
}

func testDevice_GetName(t *testing.T) {
	t.Parallel()

	d := &Device{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			require.Equal(t, IwdDeviceIface, iface)
			require.Equal(t, "Name", prop)
			return "wlan0", nil
		},
	}}

	name, err := d.GetName(context.Background())
	require.NoError(t, err)
	require.Equal(t, "wlan0", name)
}

func testDevice_GetName_WrongType(t *testing.T) {
	t.Parallel()

	d := &Device{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			return 123, nil
		},
	}}

	_, err := d.GetName(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "dbus variant conversion error")
	require.Contains(t, err.Error(), "expected string")
}

func testDevice_GetName_NoIntro(t *testing.T) {
	t.Parallel()

	d := &Device{call: nil}

	_, err := d.GetName(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "device is not initialized")
}

func testDevice_GetName_Err(t *testing.T) {
	t.Parallel()

	d := &Device{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			return nil, fmt.Errorf("dbus failure")
		},
	}}

	_, err := d.GetName(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "dbus failure")
}

func testDevice_GetNameTimeout(t *testing.T) {
	t.Parallel()

	d := &Device{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			select {
			case <-time.After(1 * time.Second):
				return "wlan0", nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		},
	}}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	t.Cleanup(func() { cancel() })

	_, err := d.GetName(ctx)
	require.Error(t, err)
}

func testDevice_GetAddress(t *testing.T) {
	t.Parallel()

	d := &Device{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			require.Equal(t, "Address", prop)
			return "aa:bb:cc:dd:ee:ff", nil
		},
	}}

	addr, err := d.GetAddress(context.Background())
	require.NoError(t, err)
	require.Equal(t, "aa:bb:cc:dd:ee:ff", addr)
}

func testDevice_GetAddress_WrongType(t *testing.T) {
	t.Parallel()

	d := &Device{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			return 123, nil
		},
	}}

	_, err := d.GetAddress(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "expected string")
}

func testDevice_GetAddress_NoIntro(t *testing.T) {
	t.Parallel()

	d := &Device{call: nil}

	_, err := d.GetAddress(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "device is not initialized")
}

func testDevice_GetPowered(t *testing.T) {
	t.Parallel()

	d := &Device{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			require.Equal(t, "Powered", prop)
			return true, nil
		},
	}}

	val, err := d.GetPowered(context.Background())
	require.NoError(t, err)
	require.True(t, val)
}

func testDevice_GetPowered_WrongType(t *testing.T) {
	t.Parallel()

	d := &Device{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			return "not-bool", nil
		},
	}}

	_, err := d.GetPowered(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "expected bool")
}

func testDevice_GetPowered_NoIntro(t *testing.T) {
	t.Parallel()

	d := &Device{call: nil}

	_, err := d.GetPowered(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "device is not initialized")
}

func testDevice_GetMode(t *testing.T) {
	t.Parallel()

	d := &Device{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			require.Equal(t, "Mode", prop)
			return "station", nil
		},
	}}

	mode, err := d.GetMode(context.Background())
	require.NoError(t, err)
	require.Equal(t, ModeStation, mode)
}

func testDevice_GetMode_Invalid(t *testing.T) {
	t.Parallel()

	d := &Device{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			return "bad-mode", nil
		},
	}}

	_, err := d.GetMode(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid mode")
}

func testDevice_GetMode_WrongType(t *testing.T) {
	t.Parallel()

	d := &Device{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			return 42, nil
		},
	}}

	_, err := d.GetMode(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "expected string")
}

func testDevice_GetMode_NoIntro(t *testing.T) {
	t.Parallel()

	d := &Device{call: nil}

	_, err := d.GetMode(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "device is not initialized")
}

func testDevice_GetAdapter(t *testing.T) {
	t.Parallel()

	d := &Device{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			require.Equal(t, "Adapter", prop)
			return dbus.ObjectPath("/net/connman/iwd/phy0"), nil
		},
	}}

	path, err := d.GetAdapter(context.Background())
	require.NoError(t, err)
	require.Equal(t, dbus.ObjectPath("/net/connman/iwd/phy0"), path)
}

func testDevice_GetAdapter_String(t *testing.T) {
	t.Parallel()

	d := &Device{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			return "/net/connman/iwd/phy0", nil
		},
	}}

	path, err := d.GetAdapter(context.Background())
	require.NoError(t, err)
	require.Equal(t, dbus.ObjectPath("/net/connman/iwd/phy0"), path)
}

func testDevice_GetAdapter_WrongType(t *testing.T) {
	t.Parallel()

	d := &Device{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			return 123, nil
		},
	}}

	_, err := d.GetAdapter(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "expected object path")
}

func testDevice_GetAdapter_NoIntro(t *testing.T) {
	t.Parallel()

	d := &Device{call: nil}

	_, err := d.GetAdapter(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "device is not initialized")
}

func testDevice_SetPowered(t *testing.T) {
	t.Parallel()

	var called bool
	d := &Device{call: &fakeCaller{
		setPropFn: func(ctx context.Context, iface, prop string, val interface{}) error {
			called = true
			require.Equal(t, IwdDeviceIface, iface)
			require.Equal(t, "Powered", prop)
			require.Equal(t, true, val)
			return nil
		},
	}}

	err := d.SetPowered(context.Background(), true)
	require.NoError(t, err)
	require.True(t, called)
}

func testDevice_SetPowered_Err(t *testing.T) {
	t.Parallel()

	d := &Device{call: &fakeCaller{
		setPropFn: func(ctx context.Context, iface, prop string, val interface{}) error {
			return fmt.Errorf("dbus failure")
		},
	}}

	err := d.SetPowered(context.Background(), true)
	require.Error(t, err)
	require.Contains(t, err.Error(), "dbus failure")
}

func testDevice_SetPowered_NoIntro(t *testing.T) {
	t.Parallel()

	d := &Device{call: nil}

	err := d.SetPowered(context.Background(), true)
	require.Error(t, err)
	require.Contains(t, err.Error(), "device is not initialized")
}

func testDevice_SetMode(t *testing.T) {
	t.Parallel()

	var called bool
	d := &Device{call: &fakeCaller{
		setPropFn: func(ctx context.Context, iface, prop string, val interface{}) error {
			called = true
			require.Equal(t, "Mode", prop)
			require.Equal(t, "ap", val)
			return nil
		},
	}}

	err := d.SetMode(context.Background(), ModeAP)
	require.NoError(t, err)
	require.True(t, called)
}

func testDevice_SetMode_Invalid(t *testing.T) {
	t.Parallel()

	d := &Device{call: &fakeCaller{
		setPropFn: func(ctx context.Context, iface, prop string, val interface{}) error {
			t.Fatal("SetProperty should not be called for an invalid mode")
			return nil
		},
	}}

	err := d.SetMode(context.Background(), ModeUnknown)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid mode")
}

func testDevice_SetMode_Err(t *testing.T) {
	t.Parallel()

	d := &Device{call: &fakeCaller{
		setPropFn: func(ctx context.Context, iface, prop string, val interface{}) error {
			return fmt.Errorf("dbus failure")
		},
	}}

	err := d.SetMode(context.Background(), ModeStation)
	require.Error(t, err)
	require.Contains(t, err.Error(), "dbus failure")
}

func testDevice_SetMode_NoIntro(t *testing.T) {
	t.Parallel()

	d := &Device{call: nil}

	err := d.SetMode(context.Background(), ModeStation)
	require.Error(t, err)
	require.Contains(t, err.Error(), "device is not initialized")
}

func testDevice_GetProperties(t *testing.T) {
	t.Parallel()

	d := newGetAllDevice(func(_ context.Context, iface string) (map[string]dbus.Variant, error) {
		require.Equal(t, IwdDeviceIface, iface)
		return map[string]dbus.Variant{
			"Name":    dbus.MakeVariant("wlan0"),
			"Address": dbus.MakeVariant("aa:bb:cc:dd:ee:ff"),
			"Powered": dbus.MakeVariant(true),
			"Mode":    dbus.MakeVariant("station"),
			"Adapter": dbus.MakeVariant(dbus.ObjectPath("/net/connman/iwd/phy0")),
		}, nil
	})

	props, err := d.GetProperties(context.Background())
	require.NoError(t, err)
	require.Equal(t, "wlan0", props.Name)
	require.Equal(t, "aa:bb:cc:dd:ee:ff", props.Address)
	require.True(t, props.Powered)
	require.Equal(t, ModeStation, props.Mode)
	require.Equal(t, dbus.ObjectPath("/net/connman/iwd/phy0"), props.Adapter)
}

func testDevice_GetProperties_Errors(t *testing.T) {
	t.Parallel()

	full := func() map[string]dbus.Variant {
		return map[string]dbus.Variant{
			"Name":    dbus.MakeVariant("wlan0"),
			"Address": dbus.MakeVariant("aa:bb:cc:dd:ee:ff"),
			"Powered": dbus.MakeVariant(true),
			"Mode":    dbus.MakeVariant("station"),
			"Adapter": dbus.MakeVariant(dbus.ObjectPath("/net/connman/iwd/phy0")),
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
		{name: "missing Name", props: without("Name"), wantContains: "property=Name"},
		{name: "missing Address", props: without("Address"), wantContains: "property=Address"},
		{name: "missing Powered", props: without("Powered"), wantContains: "property=Powered"},
		{name: "missing Mode", props: without("Mode"), wantContains: "property=Mode"},
		{name: "missing Adapter", props: without("Adapter"), wantContains: "property=Adapter"},
		{name: "Name wrong type", props: with("Name", dbus.MakeVariant(123)), wantContains: "expected string"},
		{name: "Address wrong type", props: with("Address", dbus.MakeVariant(123)), wantContains: "expected string"},
		{name: "Powered wrong type", props: with("Powered", dbus.MakeVariant("nope")), wantContains: "expected bool"},
		{name: "Mode invalid", props: with("Mode", dbus.MakeVariant("bad-mode")), wantContains: "invalid mode"},
		{name: "Adapter wrong type", props: with("Adapter", dbus.MakeVariant(42)), wantContains: "expected object path"},
		{name: "GetAll call error", callErr: fmt.Errorf("dbus failure"), wantContains: "dbus failure"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			d := newGetAllDevice(func(_ context.Context, _ string) (map[string]dbus.Variant, error) {
				if tc.callErr != nil {
					return nil, tc.callErr
				}
				return tc.props, nil
			})

			_, err := d.GetProperties(context.Background())
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.wantContains)
		})
	}
}

func testDevice_GetProperties_NoIntro(t *testing.T) {
	t.Parallel()

	d := &Device{call: nil}

	_, err := d.GetProperties(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "device is not initialized")
}

func testDevice_SubscribePropertiesChanged(t *testing.T) {
	t.Parallel()

	fake := newFakeSignalSource(t)
	ctx := context.Background()
	device := &Device{signals: fake}

	var recv DevicePropertiesChanged
	fired := make(chan struct{}, 1)

	_, err := device.SubscribePropertiesChanged(ctx, func(changed DevicePropertiesChanged) {
		recv = changed
		fired <- struct{}{}
	})
	require.NoError(t, err)

	changed := map[string]dbus.Variant{
		"Powered": dbus.MakeVariant(true),
		"Mode":    dbus.MakeVariant("ap"),
	}
	invalid := []string{"Address"}
	fake.emit("org.freedesktop.DBus.Properties", "PropertiesChanged", IwdDeviceIface, changed, invalid)

	requireFired(t, fired)
	require.Equal(t, changed, recv.Changed)
	require.Equal(t, invalid, recv.Invalidated)
}

func testDevice_SubscribePoweredChanged(t *testing.T) {
	t.Parallel()

	fake := newFakeSignalSource(t)
	ctx := context.Background()
	device := &Device{signals: fake}

	var recv bool
	fired := make(chan struct{}, 1)

	_, err := device.SubscribePoweredChanged(ctx, func(v bool) {
		recv = v
		fired <- struct{}{}
	})
	require.NoError(t, err)

	changed := map[string]dbus.Variant{
		"Powered": dbus.MakeVariant(false),
	}
	fake.emit("org.freedesktop.DBus.Properties", "PropertiesChanged", IwdDeviceIface, changed, nil)

	requireFired(t, fired)
	require.False(t, recv)
}

func testDevice_SubscribePoweredChanged_IgnoresUnrelated(t *testing.T) {
	t.Parallel()

	fake := newFakeSignalSource(t)
	ctx := context.Background()
	device := &Device{signals: fake}

	fired := make(chan struct{}, 1)

	_, err := device.SubscribePoweredChanged(ctx, func(v bool) {
		fired <- struct{}{}
	})
	require.NoError(t, err)

	// Wrong interface: an adapter PropertiesChanged must not reach a device handler.
	fake.emit("org.freedesktop.DBus.Properties", "PropertiesChanged", IwdAdapterIface, map[string]dbus.Variant{
		"Powered": dbus.MakeVariant(true),
	}, nil)

	requireNotFired(t, fired)
}

func testDevice_SubscribePoweredChanged_Unsubscribe(t *testing.T) {
	t.Parallel()

	fake := newFakeSignalSource(t)
	ctx := context.Background()
	device := &Device{signals: fake}

	fired := make(chan struct{}, 2)

	unsubscribe, err := device.SubscribePoweredChanged(ctx, func(bool) {
		fired <- struct{}{}
	})
	require.NoError(t, err)
	require.NotNil(t, unsubscribe)

	changed := map[string]dbus.Variant{
		"Powered": dbus.MakeVariant(true),
	}
	fake.emit("org.freedesktop.DBus.Properties", "PropertiesChanged", IwdDeviceIface, changed, nil)
	requireFired(t, fired)

	require.NoError(t, unsubscribe.Unsubscribe())

	fake.emit("org.freedesktop.DBus.Properties", "PropertiesChanged", IwdDeviceIface, changed, nil)
	requireNotFired(t, fired)
}

func testDevice_SubscribeModeChanged(t *testing.T) {
	t.Parallel()

	fake := newFakeSignalSource(t)
	ctx := context.Background()
	device := &Device{signals: fake}

	var recv Mode
	fired := make(chan struct{}, 1)

	_, err := device.SubscribeModeChanged(ctx, func(m Mode) {
		recv = m
		fired <- struct{}{}
	})
	require.NoError(t, err)

	changed := map[string]dbus.Variant{
		"Mode": dbus.MakeVariant("ap"),
	}
	fake.emit("org.freedesktop.DBus.Properties", "PropertiesChanged", IwdDeviceIface, changed, nil)

	requireFired(t, fired)
	require.Equal(t, ModeAP, recv)
}

func testDevice_SubscribeModeChanged_IgnoresInvalid(t *testing.T) {
	t.Parallel()

	fake := newFakeSignalSource(t)
	ctx := context.Background()
	device := &Device{signals: fake}

	fired := make(chan struct{}, 1)

	_, err := device.SubscribeModeChanged(ctx, func(Mode) {
		fired <- struct{}{}
	})
	require.NoError(t, err)

	// An unparseable mode must not reach the typed handler.
	fake.emit("org.freedesktop.DBus.Properties", "PropertiesChanged", IwdDeviceIface, map[string]dbus.Variant{
		"Mode": dbus.MakeVariant("bad-mode"),
	}, nil)

	requireNotFired(t, fired)
}

func testDevice_FirehoseReceivesAll(t *testing.T) {
	fake := newFakeSignalSource(t)
	device := &Device{signals: fake}

	var recv FirehoseSignal
	fired := make(chan struct{}, 1)

	err := device.Firehose(context.Background(), func(s FirehoseSignal) {
		recv = s
		fired <- struct{}{}
	})
	require.NoError(t, err)

	fake.emit(
		IwdDeviceIface,
		"PoweredChanged",
		map[string]dbus.Variant{"Powered": dbus.MakeVariant(false)},
		nil,
	)

	requireFired(t, fired)
	require.Equal(t, IwdDeviceIface, recv.Interface)
	require.Equal(t, "PoweredChanged", recv.Member)
}

func testDevice_FirehosePropertiesChanged(t *testing.T) {
	fake := newFakeSignalSource(t)
	device := &Device{signals: fake}

	fired := make(chan struct{}, 1)
	var recv FirehoseSignal

	_ = device.Firehose(context.Background(), func(s FirehoseSignal) {
		recv = s
		fired <- struct{}{}
	})

	changed := map[string]dbus.Variant{"Mode": dbus.MakeVariant("ap")}
	invalid := []string{"Address"}

	fake.emit("org.freedesktop.DBus.Properties", "PropertiesChanged", IwdDeviceIface, changed, invalid)

	requireFired(t, fired)

	require.Equal(t, "org.freedesktop.DBus.Properties", recv.Interface)
	require.Equal(t, "PropertiesChanged", recv.Member)
	require.Len(t, recv.Body, 3)

	s, ok := recv.Body[0].(string)
	require.True(t, ok)

	v, ok := recv.Body[1].(map[string]dbus.Variant)
	require.True(t, ok)

	ss, ok := recv.Body[2].([]string)
	require.True(t, ok)

	require.Equal(t, IwdDeviceIface, s)
	require.Equal(t, changed, v)
	require.Equal(t, invalid, ss)
}

func newGetAllDevice(fn func(ctx context.Context, iface string) (map[string]dbus.Variant, error)) *Device {
	return &Device{call: &fakeCaller{getAllFn: fn}}
}
