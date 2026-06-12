//go:build unit

package core

import (
	"context"
	"errors"
	"testing"

	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/require"

	"github.com/chrispypip/spiderw/internal/iwdbus"
)

// This file mirrors adapter_test.go's grouped t.Run subtree structure.

func TestDevice_Core(t *testing.T) {
	ctx := context.Background()

	t.Run("NewDevice", func(t *testing.T) {
		tests := []struct {
			name    string
			in      deviceRaw
			wantNil bool
		}{
			{name: "nil", in: nil, wantNil: true},
			{name: "non-nil", in: &fakeIwdbusDevice{}, wantNil: false},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				d := NewDevice(tc.in)
				if tc.wantNil {
					require.Nil(t, d)
					return
				}
				require.NotNil(t, d)
			})
		}
	})

	t.Run("Name", func(t *testing.T) {
		t.Run("Uninitialized", func(t *testing.T) {
			_, err := (*Device)(nil).Name(ctx)
			require.Error(t, err)
			require.True(t, errors.Is(err, ErrDeviceNotInitialized))
			require.True(t, errors.Is(err, ErrCore))
		})

		t.Run("DBusErrorMapping", func(t *testing.T) {
			tests := []struct {
				name     string
				dbusErr  error
				wantKind Kind
			}{
				{name: "connection", dbusErr: iwdbus.ErrDBusConnection, wantKind: KindUnavailable},
				{name: "method", dbusErr: iwdbus.ErrDBusMethod, wantKind: KindUnavailable},
				{name: "property", dbusErr: iwdbus.ErrDBusProperty, wantKind: KindUnavailable},
			}
			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					f := &fakeIwdbusDevice{}
					f.name.Store("wlan0")
					f.setErr(tc.dbusErr)
					_, err := NewDevice(f).Name(ctx)
					require.Error(t, err)

					var ce *Error
					require.ErrorAs(t, err, &ce)
					require.Equal(t, tc.wantKind, ce.Kind)
					require.Equal(t, ResourceDevice, ce.Resource)
					require.True(t, errors.Is(err, tc.dbusErr))
				})
			}
		})

		t.Run("EmptyIsInvalidState", func(t *testing.T) {
			f := &fakeIwdbusDevice{}
			f.name.Store("   ")
			_, err := NewDevice(f).Name(ctx)
			require.Error(t, err)
			var ce *Error
			require.ErrorAs(t, err, &ce)
			require.Equal(t, KindInvalidState, ce.Kind)
			require.Equal(t, ResourceDevice, ce.Resource)
		})

		t.Run("Success", func(t *testing.T) {
			name, err := newTestDevice(t).Name(ctx)
			require.NoError(t, err)
			require.Equal(t, "wlan0", name)
		})
	})

	t.Run("Address", func(t *testing.T) {
		t.Run("Uninitialized", func(t *testing.T) {
			_, err := (*Device)(nil).Address(ctx)
			require.Error(t, err)
			require.True(t, errors.Is(err, ErrDeviceNotInitialized))
		})

		t.Run("EmptyIsInvalidState", func(t *testing.T) {
			f := &fakeIwdbusDevice{}
			f.address.Store("")
			_, err := NewDevice(f).Address(ctx)
			require.Error(t, err)
			var ce *Error
			require.ErrorAs(t, err, &ce)
			require.Equal(t, KindInvalidState, ce.Kind)
		})

		t.Run("Success", func(t *testing.T) {
			addr, err := newTestDevice(t).Address(ctx)
			require.NoError(t, err)
			require.Equal(t, "aa:bb:cc:dd:ee:ff", addr)
		})
	})

	t.Run("Powered", func(t *testing.T) {
		t.Run("Uninitialized", func(t *testing.T) {
			_, err := (&Device{raw: nil}).Powered(ctx)
			require.Error(t, err)
			require.True(t, errors.Is(err, ErrDeviceNotInitialized))
		})

		t.Run("Error", func(t *testing.T) {
			f := &fakeIwdbusDevice{}
			f.setErr(iwdbus.ErrDBusMethod)
			_, err := NewDevice(f).Powered(ctx)
			require.Error(t, err)
			require.True(t, errors.Is(err, iwdbus.ErrDBusMethod))
		})

		t.Run("Success", func(t *testing.T) {
			v, err := newTestDevice(t).Powered(ctx)
			require.NoError(t, err)
			require.True(t, v)
		})
	})

	t.Run("SetPowered", func(t *testing.T) {
		t.Run("Uninitialized", func(t *testing.T) {
			err := (*Device)(nil).SetPowered(ctx, true)
			require.Error(t, err)
			require.True(t, errors.Is(err, ErrDeviceNotInitialized))
		})

		t.Run("Error", func(t *testing.T) {
			f := &fakeIwdbusDevice{}
			f.setErr(iwdbus.ErrDBusMethod)
			err := NewDevice(f).SetPowered(ctx, false)
			require.Error(t, err)
		})

		t.Run("Success", func(t *testing.T) {
			f := &fakeIwdbusDevice{}
			f.powered.Store(true)
			require.NoError(t, NewDevice(f).SetPowered(ctx, false))
			require.True(t, f.setPoweredCalled.Load())
			require.False(t, f.powered.Load())
		})
	})

	t.Run("Mode", func(t *testing.T) {
		t.Run("Uninitialized", func(t *testing.T) {
			_, err := (*Device)(nil).Mode(ctx)
			require.Error(t, err)
			require.True(t, errors.Is(err, ErrDeviceNotInitialized))
		})

		t.Run("Error", func(t *testing.T) {
			f := &fakeIwdbusDevice{}
			f.mode.Store(iwdbus.ModeStation)
			f.setErr(iwdbus.ErrDBusProperty)
			_, err := NewDevice(f).Mode(ctx)
			require.Error(t, err)
		})

		t.Run("UnknownIsInvalidState", func(t *testing.T) {
			f := &fakeIwdbusDevice{}
			f.mode.Store(iwdbus.Mode("bogus"))
			_, err := NewDevice(f).Mode(ctx)
			require.Error(t, err)
			var ce *Error
			require.ErrorAs(t, err, &ce)
			require.Equal(t, KindInvalidState, ce.Kind)
			require.Equal(t, ResourceDevice, ce.Resource)
		})

		t.Run("Success", func(t *testing.T) {
			mode, err := newTestDevice(t).Mode(ctx)
			require.NoError(t, err)
			require.Equal(t, ModeStation, mode)
		})
	})

	t.Run("SetMode", func(t *testing.T) {
		t.Run("Uninitialized", func(t *testing.T) {
			err := (*Device)(nil).SetMode(ctx, ModeAP)
			require.Error(t, err)
			require.True(t, errors.Is(err, ErrDeviceNotInitialized))
		})

		t.Run("InvalidArgument", func(t *testing.T) {
			f := &fakeIwdbusDevice{}
			err := NewDevice(f).SetMode(ctx, ModeUnknown)
			require.Error(t, err)
			var ce *Error
			require.ErrorAs(t, err, &ce)
			require.Equal(t, KindInvalidArgument, ce.Kind)
			require.Equal(t, ResourceDevice, ce.Resource)
			require.False(t, f.setModeCalled.Load())
		})

		t.Run("Error", func(t *testing.T) {
			f := &fakeIwdbusDevice{}
			f.setErr(iwdbus.ErrDBusMethod)
			err := NewDevice(f).SetMode(ctx, ModeAP)
			require.Error(t, err)
		})

		t.Run("Success", func(t *testing.T) {
			f := &fakeIwdbusDevice{}
			require.NoError(t, NewDevice(f).SetMode(ctx, ModeAP))
			require.True(t, f.setModeCalled.Load())
			require.Equal(t, iwdbus.ModeAP, f.mode.Load().(iwdbus.Mode))
		})
	})

	t.Run("Adapter", func(t *testing.T) {
		t.Run("Uninitialized", func(t *testing.T) {
			_, err := (*Device)(nil).Adapter(ctx)
			require.Error(t, err)
			require.True(t, errors.Is(err, ErrDeviceNotInitialized))
		})

		t.Run("EmptyIsInvalidState", func(t *testing.T) {
			f := &fakeIwdbusDevice{}
			f.adapter.Store(dbus.ObjectPath(""))
			_, err := NewDevice(f).Adapter(ctx)
			require.Error(t, err)
			var ce *Error
			require.ErrorAs(t, err, &ce)
			require.Equal(t, KindInvalidState, ce.Kind)
		})

		t.Run("Success", func(t *testing.T) {
			path, err := newTestDevice(t).Adapter(ctx)
			require.NoError(t, err)
			require.Equal(t, "/net/connman/iwd/phy0", path)
		})
	})

	t.Run("Properties", func(t *testing.T) {
		t.Run("Uninitialized", func(t *testing.T) {
			_, err := (*Device)(nil).Properties(ctx)
			require.Error(t, err)
			require.True(t, errors.Is(err, ErrDeviceNotInitialized))
		})

		t.Run("Error", func(t *testing.T) {
			f := &fakeIwdbusDevice{}
			f.setErr(iwdbus.ErrDBusMethod)
			_, err := NewDevice(f).Properties(ctx)
			require.Error(t, err)
		})

		t.Run("InvalidState", func(t *testing.T) {
			tests := []struct {
				name  string
				mutfn func(f *fakeIwdbusDevice)
			}{
				{name: "empty name", mutfn: func(f *fakeIwdbusDevice) { f.name.Store("  ") }},
				{name: "empty address", mutfn: func(f *fakeIwdbusDevice) { f.address.Store("") }},
				{name: "empty adapter", mutfn: func(f *fakeIwdbusDevice) { f.adapter.Store(dbus.ObjectPath("")) }},
				{name: "bad mode", mutfn: func(f *fakeIwdbusDevice) { f.mode.Store(iwdbus.Mode("nope")) }},
			}
			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					f := &fakeIwdbusDevice{}
					f.name.Store("wlan0")
					f.address.Store("aa:bb:cc:dd:ee:ff")
					f.mode.Store(iwdbus.ModeStation)
					f.adapter.Store(dbus.ObjectPath("/net/connman/iwd/phy0"))
					tc.mutfn(f)

					_, err := NewDevice(f).Properties(ctx)
					require.Error(t, err)
					var ce *Error
					require.ErrorAs(t, err, &ce)
					require.Equal(t, KindInvalidState, ce.Kind)
					require.Equal(t, ResourceDevice, ce.Resource)
				})
			}
		})

		t.Run("Success", func(t *testing.T) {
			props, err := newTestDevice(t).Properties(ctx)
			require.NoError(t, err)
			require.Equal(t, "wlan0", props.Name)
			require.Equal(t, "aa:bb:cc:dd:ee:ff", props.Address)
			require.True(t, props.Powered)
			require.Equal(t, ModeStation, props.Mode)
			require.Equal(t, "/net/connman/iwd/phy0", props.Adapter)
		})
	})

	t.Run("SubscribePropertiesChanged", func(t *testing.T) {
		t.Run("Uninitialized", func(t *testing.T) {
			_, err := (*Device)(nil).SubscribePropertiesChanged(ctx, func(DevicePropertiesChanged) {})
			require.Error(t, err)
			require.True(t, errors.Is(err, ErrDeviceNotInitialized))
		})

		t.Run("NilCallback", func(t *testing.T) {
			_, err := NewDevice(&fakeIwdbusDevice{}).SubscribePropertiesChanged(ctx, nil)
			require.Error(t, err)
			var ce *Error
			require.ErrorAs(t, err, &ce)
			require.Equal(t, KindInvalidArgument, ce.Kind)
		})

		t.Run("NormalizesEvent", func(t *testing.T) {
			f := &fakeIwdbusDevice{}
			f.subPropsEvent.Store(iwdbus.DevicePropertiesChanged{
				Changed:     map[string]dbus.Variant{"Mode": dbus.MakeVariant("ap")},
				Invalidated: []string{"Address"},
			})

			var got DevicePropertiesChanged
			_, err := NewDevice(f).SubscribePropertiesChanged(ctx, func(ev DevicePropertiesChanged) {
				got = ev
			})
			require.NoError(t, err)
			require.Equal(t, "ap", got.Changed["Mode"])
			require.Equal(t, []string{"Address"}, got.Invalidated)
		})
	})

	t.Run("SubscribePoweredChanged", func(t *testing.T) {
		t.Run("NilCallback", func(t *testing.T) {
			_, err := NewDevice(&fakeIwdbusDevice{}).SubscribePoweredChanged(ctx, nil)
			require.Error(t, err)
		})

		t.Run("DeliversEvent", func(t *testing.T) {
			f := &fakeIwdbusDevice{}
			f.subPropsEvent.Store(iwdbus.DevicePropertiesChanged{
				Changed: map[string]dbus.Variant{"Powered": dbus.MakeVariant(false)},
			})

			var got, fired = true, false
			_, err := NewDevice(f).SubscribePoweredChanged(ctx, func(b bool) {
				got = b
				fired = true
			})
			require.NoError(t, err)
			require.True(t, fired)
			require.False(t, got)
		})
	})

	t.Run("SubscribeModeChanged", func(t *testing.T) {
		t.Run("Uninitialized", func(t *testing.T) {
			_, err := (*Device)(nil).SubscribeModeChanged(ctx, func(Mode) {})
			require.Error(t, err)
			require.True(t, errors.Is(err, ErrDeviceNotInitialized))
		})

		t.Run("NilCallback", func(t *testing.T) {
			_, err := NewDevice(&fakeIwdbusDevice{}).SubscribeModeChanged(ctx, nil)
			require.Error(t, err)
		})

		t.Run("DeliversEvent", func(t *testing.T) {
			f := &fakeIwdbusDevice{}
			f.subPropsEvent.Store(iwdbus.DevicePropertiesChanged{
				Changed: map[string]dbus.Variant{"Mode": dbus.MakeVariant("ad-hoc")},
			})

			var got Mode
			_, err := NewDevice(f).SubscribeModeChanged(ctx, func(m Mode) {
				got = m
			})
			require.NoError(t, err)
			require.Equal(t, ModeAdHoc, got)
		})
	})
}
