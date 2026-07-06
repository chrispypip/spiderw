//go:build unit

package spiderw

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/chrispypip/spiderw/internal/core"
)

func TestDevice_Public(t *testing.T) {
	ctx := context.Background()

	newFullBackend := func() *fakeCoreDevice {
		f := &fakeCoreDevice{}
		f.name.Store("wlan0")
		f.address.Store("aa:bb:cc:dd:ee:ff")
		f.powered.Store(true)
		f.mode.Store(core.ModeStation)
		f.adapter.Store("/net/connman/iwd/phy0")
		return f
	}

	t.Run("Path", func(t *testing.T) {
		require.Equal(t, "", (*Device)(nil).Path())
		require.Equal(t, "", newDevice(nil, "/ignored").Path())
		require.Equal(t, "/net/connman/iwd/phy0/wlan0", newDevice(newFullBackend(), "/net/connman/iwd/phy0/wlan0").Path())
	})

	// Read accessors: success returns the backend value, a nil receiver maps to
	// an internal error, and a backend failure maps to a public device error.
	reads := []struct {
		name string
		op   func(d *Device) (any, error)
	}{
		{name: "Name", op: func(d *Device) (any, error) { return d.Name(ctx) }},
		{name: "Address", op: func(d *Device) (any, error) { return d.Address(ctx) }},
		{name: "Powered", op: func(d *Device) (any, error) { return d.Powered(ctx) }},
		{name: "Mode", op: func(d *Device) (any, error) { return d.Mode(ctx) }},
		{name: "Adapter", op: func(d *Device) (any, error) { return d.Adapter(ctx) }},
		{name: "Properties", op: func(d *Device) (any, error) { return d.Properties(ctx) }},
	}

	for _, r := range reads {
		t.Run(r.name, func(t *testing.T) {
			t.Run("Success", func(t *testing.T) {
				out, err := r.op(&Device{core: newFullBackend()})
				require.NoError(t, err)
				require.NotNil(t, out)
			})

			t.Run("NilReceiver", func(t *testing.T) {
				_, err := r.op((*Device)(nil))
				require.Error(t, err)
				require.True(t, errors.Is(err, ErrInternal))
			})

			t.Run("BackendError", func(t *testing.T) {
				f := newFullBackend()
				f.setErr(core.WrapDeviceUnavailable("op", "boom", core.WrapInvalidState(core.ResourceDevice, "op", "boom", errors.New("x"))))
				_, err := r.op(&Device{core: f})
				require.Error(t, err)

				var pe *Error
				require.ErrorAs(t, err, &pe)
				require.Equal(t, ResourceDevice, pe.Resource)
			})
		})
	}

	t.Run("Values", func(t *testing.T) {
		d := &Device{core: newFullBackend()}

		name, err := d.Name(ctx)
		require.NoError(t, err)
		require.Equal(t, "wlan0", name)

		addr, err := d.Address(ctx)
		require.NoError(t, err)
		require.Equal(t, "aa:bb:cc:dd:ee:ff", addr)

		mode, err := d.Mode(ctx)
		require.NoError(t, err)
		require.Equal(t, ModeStation, mode)

		adapter, err := d.Adapter(ctx)
		require.NoError(t, err)
		require.Equal(t, "/net/connman/iwd/phy0", adapter)

		props, err := d.Properties(ctx)
		require.NoError(t, err)
		require.Equal(t, "wlan0", props.Name)
		require.Equal(t, "aa:bb:cc:dd:ee:ff", props.Address)
		require.True(t, props.Powered)
		require.Equal(t, ModeStation, props.Mode)
		// Scalar Adapter() stays a raw path; the bundle field is a resolved ref
		// (Name empty here with no resolver).
		require.Equal(t, AdapterRef{Path: "/net/connman/iwd/phy0"}, props.Adapter)
	})

	t.Run("SetPowered", func(t *testing.T) {
		t.Run("Success", func(t *testing.T) {
			f := &fakeCoreDevice{}
			f.powered.Store(true)
			require.NoError(t, (&Device{core: f}).SetPowered(ctx, false))
			require.True(t, f.setPoweredCalled.Load())
			require.False(t, f.powered.Load())
		})

		t.Run("NilReceiver", func(t *testing.T) {
			err := (*Device)(nil).SetPowered(ctx, true)
			require.Error(t, err)
			require.True(t, errors.Is(err, ErrInternal))
		})
	})

	t.Run("SetMode", func(t *testing.T) {
		t.Run("Success", func(t *testing.T) {
			f := &fakeCoreDevice{}
			require.NoError(t, (&Device{core: f}).SetMode(ctx, ModeAP))
			require.True(t, f.setModeCalled.Load())
			require.Equal(t, core.ModeAP, f.mode.Load().(core.Mode))
		})

		t.Run("NilReceiver", func(t *testing.T) {
			err := (*Device)(nil).SetMode(ctx, ModeAP)
			require.Error(t, err)
			require.True(t, errors.Is(err, ErrInternal))
		})

		t.Run("InvalidModeRejectedAtBoundary", func(t *testing.T) {
			f := &fakeCoreDevice{}
			err := (&Device{core: f}).SetMode(ctx, Mode("garbage"))
			require.Error(t, err)
			require.True(t, errors.Is(err, ErrInvalidArgument))

			var pe *Error
			require.ErrorAs(t, err, &pe)
			require.Equal(t, ResourceDevice, pe.Resource)

			// The public boundary rejects before reaching the backend.
			require.False(t, f.setModeCalled.Load())
		})
	})

	t.Run("ModeReadValidation", func(t *testing.T) {
		// A mode the lower layers should never deliver is rejected on read
		// rather than surfaced as an unrecognized public Mode.
		f := &fakeCoreDevice{}
		f.mode.Store(core.Mode("garbage"))
		_, err := (&Device{core: f}).Mode(ctx)
		require.Error(t, err)
		require.True(t, errors.Is(err, ErrInvalidArgument))
	})

	t.Run("SubscribePropertiesChanged", func(t *testing.T) {
		t.Run("NilCallback", func(t *testing.T) {
			_, err := (&Device{core: &fakeCoreDevice{}}).SubscribePropertiesChanged(ctx, nil)
			require.Error(t, err)
			var pe *Error
			require.ErrorAs(t, err, &pe)
			require.Equal(t, KindInvalidArgument, pe.Kind)
			require.Equal(t, ResourceDevice, pe.Resource)
		})

		t.Run("NilReceiver", func(t *testing.T) {
			_, err := (*Device)(nil).SubscribePropertiesChanged(ctx, func(DevicePropertiesChanged) {})
			require.Error(t, err)
			require.True(t, errors.Is(err, ErrInternal))
		})

		t.Run("DeliversEvent", func(t *testing.T) {
			f := &fakeCoreDevice{}
			f.subPropsEvent.Store(core.DevicePropertiesChanged{
				Changed:     map[string]any{"Mode": "ap"},
				Invalidated: []string{"Address"},
			})

			var got DevicePropertiesChanged
			_, err := (&Device{core: f}).SubscribePropertiesChanged(ctx, func(ev DevicePropertiesChanged) {
				got = ev
			})
			require.NoError(t, err)
			require.Equal(t, "ap", got.Changed["Mode"])
			require.Equal(t, []string{"Address"}, got.Invalidated)
		})
	})

	t.Run("SubscribePoweredChanged", func(t *testing.T) {
		t.Run("NilCallback", func(t *testing.T) {
			_, err := (&Device{core: &fakeCoreDevice{}}).SubscribePoweredChanged(ctx, nil)
			require.Error(t, err)
		})

		t.Run("DeliversEvent", func(t *testing.T) {
			f := &fakeCoreDevice{}
			f.subPropsEvent.Store(core.DevicePropertiesChanged{
				Changed: map[string]any{"Powered": false},
			})

			got, fired := true, false
			_, err := (&Device{core: f}).SubscribePoweredChanged(ctx, func(b bool) {
				got = b
				fired = true
			})
			require.NoError(t, err)
			require.True(t, fired)
			require.False(t, got)
		})
	})

	t.Run("SubscribeModeChanged", func(t *testing.T) {
		t.Run("NilCallback", func(t *testing.T) {
			_, err := (&Device{core: &fakeCoreDevice{}}).SubscribeModeChanged(ctx, nil)
			require.Error(t, err)
		})

		t.Run("NilReceiver", func(t *testing.T) {
			_, err := (*Device)(nil).SubscribeModeChanged(ctx, func(Mode) {})
			require.Error(t, err)
			require.True(t, errors.Is(err, ErrInternal))
		})

		t.Run("DeliversEventConvertedToPublicMode", func(t *testing.T) {
			f := &fakeCoreDevice{}
			f.subPropsEvent.Store(core.DevicePropertiesChanged{
				Changed: map[string]any{"Mode": "ad-hoc"},
			})

			var got Mode
			_, err := (&Device{core: f}).SubscribeModeChanged(ctx, func(m Mode) {
				got = m
			})
			require.NoError(t, err)
			require.Equal(t, ModeAdHoc, got)
		})
	})
}
