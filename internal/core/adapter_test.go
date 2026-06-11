//go:build unit

package core

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/require"

	"github.com/chrispypip/spiderw/internal/iwdbus"
)

// NOTE: This file uses a "subset" subtest structure (t.Run trees) so related
// cases are grouped under a small number of top-level tests. This improves
// readability and makes it easy to run targeted slices (e.g. -run TestAdapter/Powered).

func TestAdapter_Core(t *testing.T) {
	ctx := context.Background()

	t.Run("NewAdapter", func(t *testing.T) {
		fake := &fakeIwdbusAdapter{}
		fake.name.Store("phy0")
		tests := []struct {
			name    string
			in      adapterRaw
			wantNil bool
		}{
			{name: "nil", in: nil, wantNil: true},
			{name: "non-nil", in: fake, wantNil: false},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				a := NewAdapter(tc.in)
				if tc.wantNil {
					require.Nil(t, a)
					return
				}
				require.NotNil(t, a)
			})
		}
	})

	t.Run("Powered", func(t *testing.T) {
		t.Run("Uninitialized", func(t *testing.T) {
			tests := []struct {
				name    string
				adapter *Adapter
			}{
				{name: "nil receiver", adapter: nil},
				{name: "inner nil", adapter: &Adapter{raw: nil}},
			}
			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					_, err := tc.adapter.Powered(ctx)
					require.Error(t, err)
					require.True(t, errors.Is(err, ErrAdapterNotInitialized))
					require.True(t, errors.Is(err, ErrCore))
				})
			}
		})

		t.Run("DBusErrorMapping", func(t *testing.T) {
			tests := []struct {
				name     string
				dbusErr  error
				wantKind Kind
			}{
				{name: "connection", dbusErr: iwdbus.ErrDBusConnection, wantKind: KindUnavailable},
				{name: "method", dbusErr: iwdbus.ErrDBusMethod, wantKind: KindUnavailable},
				{name: "introspection", dbusErr: iwdbus.ErrDBusIntrospection, wantKind: KindUnavailable},
				{name: "variant", dbusErr: iwdbus.ErrDBusVariant, wantKind: KindUnavailable},
				{name: "property", dbusErr: iwdbus.ErrDBusProperty, wantKind: KindUnavailable},
			}
			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					f := &fakeIwdbusAdapter{}
					f.setErr(tc.dbusErr)
					a := NewAdapter(f)
					_, err := a.Powered(ctx)
					require.Error(t, err)

					var ce *Error
					require.ErrorAs(t, err, &ce)
					require.Equal(t, tc.wantKind, ce.Kind)
					require.Equal(t, ResourceAdapter, ce.Resource)

					require.True(t, errors.Is(err, ErrCore))
					require.True(t, errors.Is(err, tc.dbusErr))
				})
			}
		})

		t.Run("Success", func(t *testing.T) {
			fake := &fakeIwdbusAdapter{}
			fake.powered.Store(true)
			a := NewAdapter(fake)
			v, err := a.Powered(ctx)
			require.NoError(t, err)
			require.True(t, v)
		})
	})

	t.Run("SetPowered", func(t *testing.T) {
		t.Run("Uninitialized", func(t *testing.T) {
			tests := []struct {
				name    string
				adapter *Adapter
			}{
				{name: "nil receiver", adapter: nil},
				{name: "inner nil", adapter: &Adapter{raw: nil}},
			}
			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					err := tc.adapter.SetPowered(ctx, true)
					require.Error(t, err)
					require.True(t, errors.Is(err, ErrAdapterNotInitialized))
					require.True(t, errors.Is(err, ErrCore))
				})
			}
		})

		t.Run("ErrorMapping", func(t *testing.T) {
			f := &fakeIwdbusAdapter{}
			f.setErr(iwdbus.ErrDBusMethod)
			a := NewAdapter(f)
			err := a.SetPowered(ctx, false)
			require.Error(t, err)

			var ce *Error
			require.ErrorAs(t, err, &ce)
			require.Equal(t, KindUnavailable, ce.Kind)
			require.Equal(t, ResourceAdapter, ce.Resource)
			require.True(t, errors.Is(err, iwdbus.ErrDBusMethod))
		})

		t.Run("Success", func(t *testing.T) {
			f := &fakeIwdbusAdapter{}
			a := NewAdapter(f)

			err := a.SetPowered(ctx, true)
			require.NoError(t, err)

			require.True(t, f.setPoweredCalled.Load())
			require.True(t, f.powered.Load())
		})
	})

	t.Run("Name", func(t *testing.T) {
		t.Run("InvalidFields", func(t *testing.T) {
			tests := []struct {
				name    string
				rawName string
				wantMsg string
			}{
				{name: "empty", rawName: "", wantMsg: "Name"},
				{name: "whitespace", rawName: "   ", wantMsg: "Name"},
			}
			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					fake := &fakeIwdbusAdapter{}
					fake.name.Store(tc.rawName)
					a := NewAdapter(fake)
					_, err := a.Name(ctx)
					require.Error(t, err)

					var ce *Error
					require.ErrorAs(t, err, &ce)
					require.Equal(t, KindInvalidState, ce.Kind)
					require.Equal(t, ResourceAdapter, ce.Resource)
					require.Contains(t, err.Error(), tc.wantMsg)
				})
			}
		})

		t.Run("ErrorMapping", func(t *testing.T) {
			f := &fakeIwdbusAdapter{}
			f.setErr(iwdbus.ErrDBusVariant)
			a := NewAdapter(f)
			_, err := a.Name(ctx)
			require.Error(t, err)

			var ce *Error
			require.ErrorAs(t, err, &ce)
			require.Equal(t, KindUnavailable, ce.Kind)
			require.Equal(t, ResourceAdapter, ce.Resource)
		})

		t.Run("Success", func(t *testing.T) {
			fake := &fakeIwdbusAdapter{}
			fake.name.Store("  phy0  ")
			a := NewAdapter(fake)
			n, err := a.Name(ctx)
			require.NoError(t, err)
			require.Equal(t, "phy0", n)
		})
	})

	t.Run("ModelAndVendor", func(t *testing.T) {
		trim := func(s string) *string { return &s }

		t.Run("Model", func(t *testing.T) {
			tests := []struct {
				name  string
				model *string
				want  *string
			}{
				{name: "nil", model: nil, want: nil},
				{name: "trim", model: trim("  AX200  "), want: trim("AX200")},
				{name: "empty becomes empty", model: trim("   "), want: trim("")},
			}
			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					fake := &fakeIwdbusAdapter{}
					fake.model.Store(tc.model)
					a := NewAdapter(fake)
					out, err := a.Model(ctx)
					require.NoError(t, err)
					require.Equal(t, tc.want, out)
				})
			}
		})

		t.Run("Vendor", func(t *testing.T) {
			tests := []struct {
				name   string
				vendor *string
				want   *string
			}{
				{name: "nil", vendor: nil, want: nil},
				{name: "trim", vendor: trim("  Intel  "), want: trim("Intel")},
			}
			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					fake := &fakeIwdbusAdapter{}
					fake.vendor.Store(tc.vendor)
					a := NewAdapter(fake)
					out, err := a.Vendor(ctx)
					require.NoError(t, err)
					require.Equal(t, tc.want, out)
				})
			}
		})

		t.Run("ErrorMapping", func(t *testing.T) {
			f := &fakeIwdbusAdapter{}
			f.setErr(iwdbus.ErrDBusMethod)
			a := NewAdapter(f)
			_, err := a.Model(ctx)
			require.Error(t, err)
			var ce *Error
			require.ErrorAs(t, err, &ce)
			require.Equal(t, KindUnavailable, ce.Kind)
			require.Equal(t, ResourceAdapter, ce.Resource)
		})
	})

	t.Run("SupportedModes", func(t *testing.T) {
		t.Run("Success", func(t *testing.T) {
			fake := &fakeIwdbusAdapter{}
			fake.modes.Store([]iwdbus.AdapterMode{iwdbus.AdapterModeStation, iwdbus.AdapterModeAP})
			a := NewAdapter(fake)
			out, err := a.SupportedModes(ctx)
			require.NoError(t, err)
			require.Equal(t, []AdapterMode{AdapterModeStation, AdapterModeAP}, out)
		})

		t.Run("UnknownMode", func(t *testing.T) {
			fake := &fakeIwdbusAdapter{}
			fake.modes.Store([]iwdbus.AdapterMode{iwdbus.AdapterMode("bad-mode")})
			a := NewAdapter(fake)
			_, err := a.SupportedModes(ctx)
			require.Error(t, err)

			require.Error(t, err)
			require.Contains(t, err.Error(), "unknown supported mode")
		})
	})

	t.Run("SupportsMode", func(t *testing.T) {
		t.Run("UnknownCoreMode", func(t *testing.T) {
			a := NewAdapter(&fakeIwdbusAdapter{})
			_, err := a.SupportsMode(ctx, AdapterModeUnknown)
			require.Error(t, err)

			var ce *Error
			require.ErrorAs(t, err, &ce)
			require.Equal(t, KindInvalidArgument, ce.Kind)
			require.Equal(t, ResourceAdapter, ce.Resource)
			require.Contains(t, err.Error(), "unknown")
		})

		t.Run("DBusErrorMapping", func(t *testing.T) {
			f := &fakeIwdbusAdapter{}
			f.setErr(iwdbus.ErrDBusMethod)
			a := NewAdapter(f)
			_, err := a.SupportsMode(ctx, AdapterModeStation)
			require.Error(t, err)

			var ce *Error
			require.ErrorAs(t, err, &ce)
			require.Equal(t, KindUnavailable, ce.Kind)
			require.Equal(t, ResourceAdapter, ce.Resource)
		})

		t.Run("Success", func(t *testing.T) {
			f := &fakeIwdbusAdapter{}
			f.modes.Store([]iwdbus.AdapterMode{iwdbus.AdapterModeAP, iwdbus.AdapterModeStation})
			a := NewAdapter(f)
			ok, err := a.SupportsMode(ctx, AdapterModeAP)
			require.NoError(t, err)
			require.True(t, ok)
		})
	})

	t.Run("SubscribePropertiesChanged", func(t *testing.T) {
		t.Run("Uninitialized", func(t *testing.T) {
			tests := []struct {
				name    string
				adapter *Adapter
			}{
				{name: "nil receiver", adapter: nil},
				{name: "inner nil", adapter: &Adapter{raw: nil}},
			}
			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					_, err := tc.adapter.SubscribePropertiesChanged(ctx, func(AdapterPropertiesChanged) {})
					require.Error(t, err)
					require.True(t, errors.Is(err, ErrAdapterNotInitialized))
				})
			}
		})

		t.Run("NilCallback", func(t *testing.T) {
			a := NewAdapter(&fakeIwdbusAdapter{})
			_, err := a.SubscribePropertiesChanged(ctx, nil)
			require.Error(t, err)
			require.Contains(t, err.Error(), "callback cannot be nil")
		})

		t.Run("SuccessNormalizesVariants", func(t *testing.T) {
			f := &fakeIwdbusAdapter{}
			f.subPropsEvent.Store(iwdbus.AdapterPropertiesChanged{
				Changed: map[string]dbus.Variant{
					"Powered": dbus.MakeVariant(true),
					"Name":    dbus.MakeVariant("phy0"),
					"Num":     dbus.MakeVariant(int32(7)),
				},
				Invalidated: []string{"Model"},
			})
			a := NewAdapter(f)

			var got AdapterPropertiesChanged
			_, err := a.SubscribePropertiesChanged(ctx, func(ev AdapterPropertiesChanged) {
				got = ev
			})
			require.NoError(t, err)

			require.Equal(t, []string{"Model"}, got.Invalidated)
			require.Equal(t, true, got.Changed["Powered"])
			require.Equal(t, "phy0", got.Changed["Name"])
			require.Equal(t, int32(7), got.Changed["Num"])
		})
	})

	t.Run("SubscribePoweredChanged", func(t *testing.T) {
		t.Run("Uninitialized", func(t *testing.T) {
			tests := []struct {
				name    string
				adapter *Adapter
			}{
				{name: "nil receiver", adapter: nil},
				{name: "inner nil", adapter: &Adapter{raw: nil}},
			}
			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					_, err := tc.adapter.SubscribePoweredChanged(ctx, func(bool) {})
					require.Error(t, err)
					require.True(t, errors.Is(err, ErrAdapterNotInitialized))
				})
			}
		})

		t.Run("NilCallback", func(t *testing.T) {
			a := NewAdapter(&fakeIwdbusAdapter{})
			_, err := a.SubscribePoweredChanged(ctx, nil)
			require.Error(t, err)
			require.Contains(t, err.Error(), "callback cannot be nil")
		})

		t.Run("Success", func(t *testing.T) {
			f := &fakeIwdbusAdapter{}
			f.powered.Store(false)
			f.subPropsEvent.Store(iwdbus.AdapterPropertiesChanged{
				Changed: map[string]dbus.Variant{
					"Powered": dbus.MakeVariant(true),
				},
			})
			a := NewAdapter(f)

			var got bool
			_, err := a.SubscribePoweredChanged(ctx, func(v bool) { got = v })
			require.NoError(t, err)
			require.True(t, got)
		})
	})

	t.Run("Concurrency", func(t *testing.T) {
		a := newTestAdapter(t)
		ctx := context.Background()

		var wg sync.WaitGroup
		for range 50 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				powered, err := a.Powered(ctx)
				require.NoError(t, err)
				require.True(t, powered)
			}()
		}

		wg.Wait()
	})

	t.Run("ErrorMessageStability", func(t *testing.T) {
		f := &fakeIwdbusAdapter{}
		f.name.Store("")
		a := NewAdapter(f)
		_, err := a.Name(ctx)
		require.Error(t, err)

		msg := err.Error()
		require.Contains(t, msg, "invalid state")
		require.Contains(t, msg, "Adapter.Name")
		require.Contains(t, msg, "Name")
	})
}

func TestAdapter_Properties(t *testing.T) {
	ctx := context.Background()

	t.Run("NormalizesAndValidates", func(t *testing.T) {
		model := "  Broadcom  "
		vendor := "  Acme  "
		f := &fakeIwdbusAdapter{}
		f.powered.Store(true)
		f.name.Store("  phy0  ")
		f.model.Store(&model)
		f.vendor.Store(&vendor)
		f.modes.Store([]iwdbus.AdapterMode{iwdbus.AdapterModeStation, iwdbus.AdapterModeAP})
		a := NewAdapter(f)

		props, err := a.Properties(ctx)
		require.NoError(t, err)
		require.True(t, props.Powered)
		require.Equal(t, "phy0", props.Name)
		require.NotNil(t, props.Model)
		require.Equal(t, "Broadcom", *props.Model)
		require.NotNil(t, props.Vendor)
		require.Equal(t, "Acme", *props.Vendor)
		require.Equal(t, []AdapterMode{AdapterModeStation, AdapterModeAP}, props.SupportedModes)
	})

	t.Run("OptionalsNil", func(t *testing.T) {
		f := &fakeIwdbusAdapter{}
		f.name.Store("phy0")
		f.modes.Store([]iwdbus.AdapterMode{iwdbus.AdapterModeStation})
		a := NewAdapter(f)

		props, err := a.Properties(ctx)
		require.NoError(t, err)
		require.Nil(t, props.Model)
		require.Nil(t, props.Vendor)
	})

	t.Run("EmptyNameInvalidState", func(t *testing.T) {
		f := &fakeIwdbusAdapter{}
		f.name.Store("   ")
		f.modes.Store([]iwdbus.AdapterMode{iwdbus.AdapterModeStation})
		a := NewAdapter(f)

		_, err := a.Properties(ctx)
		require.Error(t, err)
		require.Contains(t, err.Error(), "empty Name")
	})

	t.Run("BackendErrorWrapped", func(t *testing.T) {
		base := errors.New("boom")
		f := &fakeIwdbusAdapter{}
		f.setErr(base)
		a := NewAdapter(f)

		_, err := a.Properties(ctx)
		require.Error(t, err)
		require.ErrorIs(t, err, base)
	})
}
