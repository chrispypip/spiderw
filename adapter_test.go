//go:build unit

package spiderw

import (
	"context"
	"errors"
	"maps"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/chrispypip/spiderw/internal/core"
)

func TestAdapter_Public(t *testing.T) {
	ctx := context.Background()

	properties := []struct {
		name       string
		op         func(a *Adapter) (any, error)
		newBackend func() *fakeCoreAdapter
		wantSent   error
	}{
		{
			name: "Powered",
			op:   func(a *Adapter) (any, error) { return a.Powered(ctx) },
			newBackend: func() *fakeCoreAdapter {
				f := &fakeCoreAdapter{}
				f.powered.Store(true)
				return f
			},
		},
		{
			name: "Name",
			op:   func(a *Adapter) (any, error) { return a.Name(ctx) },
			newBackend: func() *fakeCoreAdapter {
				f := &fakeCoreAdapter{}
				f.name.Store("phy0")
				return f
			},
		},
		{
			name: "Model",
			op:   func(a *Adapter) (any, error) { return a.Model(ctx) },
			newBackend: func() *fakeCoreAdapter {
				f := &fakeCoreAdapter{}
				f.model.Store(new("Broadcomm"))
				return f
			},
		},
		{
			name: "Vendor",
			op:   func(a *Adapter) (any, error) { return a.Vendor(ctx) },
			newBackend: func() *fakeCoreAdapter {
				f := &fakeCoreAdapter{}
				f.vendor.Store(new("Intel"))
				return f
			},
		},
		{
			name: "SupportedModes",
			op:   func(a *Adapter) (any, error) { return a.SupportedModes(ctx) },
			newBackend: func() *fakeCoreAdapter {
				modes := []core.Mode{core.ModeAP, core.ModeStation}
				f := &fakeCoreAdapter{}
				f.modes.Store(modes)
				return f
			},
		},
		{
			name: "SupportsMode",
			op:   func(a *Adapter) (any, error) { return a.SupportsMode(ctx, ModeAP) },
			newBackend: func() *fakeCoreAdapter {
				f := &fakeCoreAdapter{}
				f.modes.Store([]core.Mode{core.ModeAP})
				return f
			},
		},
	}

	t.Run("Properties", func(t *testing.T) {
		for _, p := range properties {
			t.Run(p.name, func(t *testing.T) {
				t.Run("Success", func(t *testing.T) {
					f := p.newBackend()
					a := &Adapter{core: f}
					out, err := p.op(a)
					require.NoError(t, err)
					require.NotNil(t, out)
				})

				t.Run("NilReceiver", func(t *testing.T) {
					var a *Adapter
					_, err := p.op(a)
					require.Error(t, err)
					require.ErrorIs(t, err, ErrInternal)
				})

				t.Run("BackendError", func(t *testing.T) {
					// A backend failure surfaces as a translated public *Error
					// carrying ResourceAdapter for every accessor, not just Powered.
					f := p.newBackend()
					f.setErr(core.WrapAdapterUnavailable("op", "boom", errors.New("x")))
					_, err := p.op(&Adapter{core: f})
					require.Error(t, err)
					var pe *Error
					require.ErrorAs(t, err, &pe)
					require.Equal(t, ResourceAdapter, pe.Resource)
				})
			})
		}

		t.Run("ErrorMapping", func(t *testing.T) {
			t.Run("KindUnavailable", func(t *testing.T) {
				f := &fakeCoreAdapter{}
				f.setErr(&core.Error{
					Kind:     core.KindUnavailable,
					Resource: core.ResourceAdapter,
				})
				a := &Adapter{core: f}
				_, err := a.Powered(ctx)
				require.Error(t, err)
				require.ErrorIs(t, err, ErrUnavailable)

				var pe *Error
				require.ErrorAs(t, err, &pe)
				require.Equal(t, ResourceAdapter, pe.Resource)
			})

			t.Run("KindInvalidState", func(t *testing.T) {
				f := &fakeCoreAdapter{}
				f.setErr(&core.Error{
					Kind:     core.KindInvalidState,
					Resource: core.ResourceAdapter,
				})
				a := &Adapter{core: f}
				_, err := a.Powered(ctx)
				require.Error(t, err)
				require.ErrorIs(t, err, ErrInvalidState)
			})

			t.Run("KindOperationFailed", func(t *testing.T) {
				f := &fakeCoreAdapter{}
				f.setErr(&core.Error{
					Kind:     core.KindOperationFailed,
					Resource: core.ResourceAdapter,
				})
				a := &Adapter{core: f}
				_, err := a.Powered(ctx)
				require.Error(t, err)
				require.ErrorIs(t, err, ErrInternal)
			})
		})
	})

	conveniences := []struct {
		name     string
		op       func(a *Adapter) (bool, error)
		wantSent error
	}{
		{
			name: "SupportsStation",
			op:   func(a *Adapter) (bool, error) { return a.SupportsStation(ctx) },
		},
		{
			name: "SupportsAP",
			op:   func(a *Adapter) (bool, error) { return a.SupportsAP(ctx) },
		},
		{
			name: "SupportsAdHoc",
			op:   func(a *Adapter) (bool, error) { return a.SupportsAdHoc(ctx) },
		},
	}

	t.Run("Conveniences", func(t *testing.T) {
		for _, c := range conveniences {
			t.Run(c.name, func(t *testing.T) {
				t.Run("Yes", func(t *testing.T) {
					f := &fakeCoreAdapter{}
					f.modes.Store([]core.Mode{core.ModeStation, core.ModeAP, core.ModeAdHoc})
					t.Run("Success", func(t *testing.T) {
						a := &Adapter{core: f}
						out, err := c.op(a)
						require.NoError(t, err)
						require.True(t, out)
					})
				})
				t.Run("No", func(t *testing.T) {
					f := &fakeCoreAdapter{}
					t.Run("Success", func(t *testing.T) {
						a := &Adapter{core: f}
						out, err := c.op(a)
						require.NoError(t, err)
						require.False(t, out)
					})
				})
			})

			t.Run("NilReceive", func(t *testing.T) {
				var a *Adapter
				_, err := c.op(a)
				require.Error(t, err)
				require.ErrorIs(t, err, ErrInternal)
			})
		}
	})

	t.Run("ErrorMessageStability", func(t *testing.T) {
		f := &fakeCoreAdapter{}
		f.setErr(&core.Error{
			Kind: core.KindOperationFailed,
			Err:  errors.New("x"),
		})
		a := &Adapter{core: f}

		_, err := a.Powered(ctx)
		require.Error(t, err)

		msg1 := err.Error()
		msg2 := err.Error()
		require.Equal(t, msg1, msg2)
	})
}

func TestAdapter_Subscribe_Validation(t *testing.T) {
	ctx := context.Background()

	type tc struct {
		name            string
		call            func() (UnsubscribeFunc, error)
		wantErrInternal bool
		wantErrContains []string
	}

	tests := []tc{
		{
			name: "SubscribePropertiesChanged_NilReceiver",
			call: func() (UnsubscribeFunc, error) {
				var a *Adapter
				return a.SubscribePropertiesChanged(ctx, func(AdapterPropertiesChanged) {})
			},
			wantErrInternal: true,
		},
		{
			name: "SubscribePropertiesChanged_NilCallback",
			call: func() (UnsubscribeFunc, error) {
				a := &Adapter{core: &fakeCoreAdapter{}}
				return a.SubscribePropertiesChanged(ctx, nil)
			},
			wantErrContains: []string{"Adapter.SubscribePropertiesChanged", "callback cannot be nil"},
		},
		{
			name: "SubscribePoweredChanged_NilReceiver",
			call: func() (UnsubscribeFunc, error) {
				var a *Adapter
				return a.SubscribePoweredChanged(ctx, func(bool) {})
			},
			wantErrInternal: true,
		},
		{
			name: "SubscribePoweredChanged_NilCallback",
			call: func() (UnsubscribeFunc, error) {
				a := &Adapter{core: &fakeCoreAdapter{}}
				return a.SubscribePoweredChanged(ctx, nil)
			},
			wantErrContains: []string{"Adapter.SubscribePoweredChanged", "callback cannot be nil"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.call()
			require.Error(t, err)

			if tt.wantErrInternal {
				require.ErrorIs(t, err, ErrInternal)
			}
			for _, s := range tt.wantErrContains {
				require.Contains(t, err.Error(), s)
			}
		})
	}
}

func TestAdapter_SubscribePropertiesChanged_Public_CopiesPayload(t *testing.T) {
	ctx := context.Background()

	t.Run("Invalidated_NilStaysNil", func(t *testing.T) {
		f := &fakeCoreAdapter{}
		f.subPropsEvent.Store(core.AdapterPropertiesChanged{
			Changed:     map[string]any{"Name": "phy0"},
			Invalidated: nil,
		})

		a := &Adapter{core: f}

		var got AdapterPropertiesChanged
		_, err := a.SubscribePropertiesChanged(ctx, func(ev AdapterPropertiesChanged) {
			// Snapshot the only field we care about.
			got.Invalidated = ev.Invalidated
		})
		require.NoError(t, err)

		// Contract: nil stats nil (not converted to empty slice).
		require.Nil(t, got.Invalidated)
	})

	t.Run("Invalidated_EmptyStaysEmptyNonNil", func(t *testing.T) {
		f := &fakeCoreAdapter{}
		emptyButNonNil := make([]string, 0) // non-nil, len=0
		f.subPropsEvent.Store(core.AdapterPropertiesChanged{
			Changed:     map[string]any{"Name": "phy0"},
			Invalidated: emptyButNonNil,
		})

		a := &Adapter{core: f}

		var got AdapterPropertiesChanged
		_, err := a.SubscribePropertiesChanged(ctx, func(ev AdapterPropertiesChanged) {
			got.Invalidated = ev.Invalidated
		})
		require.NoError(t, err)

		// Contract: empty-but-non-nil remains non-nil and len==0
		require.NotNil(t, got.Invalidated)
		require.Empty(t, got.Invalidated)
	})

	t.Run("Invalidated_NonEmptyCopied", func(t *testing.T) {
		f := &fakeCoreAdapter{}
		f.subPropsEvent.Store(core.AdapterPropertiesChanged{
			Changed:     map[string]any{"Name": "phy0"},
			Invalidated: []string{"Model"},
		})

		a := &Adapter{core: f}

		var got AdapterPropertiesChanged
		_, err := a.SubscribePropertiesChanged(ctx, func(ev AdapterPropertiesChanged) {
			// Snapshot so later mutation (if any) wouldn't affect expectations.
			got.Invalidated = append([]string(nil), ev.Invalidated...)

			// Mutate user view; must not mutate underlying core payload.
			if len(ev.Invalidated) > 0 {
				ev.Invalidated[0] = "UserMutate"
			}
		})
		require.NoError(t, err)

		require.Equal(t, []string{"Model"}, got.Invalidated)
		ev := f.subPropsEvent.Load().(core.AdapterPropertiesChanged)
		require.Equal(t, []string{"Model"}, ev.Invalidated)
	})

	f := &fakeCoreAdapter{}

	// The underlying core payload we will ensure is NOT mutated by the user callback.
	coreChanged := map[string]any{"Powered": true, "Name": "phy0"}
	coreInvalidated := []string{"Model"}
	f.subPropsEvent.Store(core.AdapterPropertiesChanged{
		Changed:     coreChanged,
		Invalidated: coreInvalidated,
	})

	a := &Adapter{core: f}

	var got AdapterPropertiesChanged
	_, err := a.SubscribePropertiesChanged(ctx, func(ev AdapterPropertiesChanged) {
		got = ev

		// Snapshot what the user received before mutating it.
		got.Changed = make(map[string]any, len(ev.Changed))
		maps.Copy(got.Changed, ev.Changed)
		got.Invalidated = append([]string(nil), ev.Invalidated...)

		// Simulate typical user behavior: mutate what they receive.
		if ev.Changed != nil {
			ev.Changed["UserMutate"] = 123
		}
		if len(ev.Invalidated) > 0 {
			ev.Invalidated[0] = "UserMutate"
		}
	})
	require.NoError(t, err)

	// Public wrapper correctness.
	require.NotNil(t, got.Changed)
	require.Equal(t, true, got.Changed["Powered"])
	require.Equal(t, "phy0", got.Changed["Name"])
	require.Equal(t, []string{"Model"}, got.Invalidated)

	// Ensure the original core event was NOT mutated via aliasing.
	ev := f.subPropsEvent.Load().(core.AdapterPropertiesChanged)
	require.NotContains(t, ev.Changed, "UserMutate")
	require.Equal(t, []string{"Model"}, ev.Invalidated)
}

func TestAdapter_SubscribePoweredChanged_Public_InvokesCallback(t *testing.T) {
	ctx := context.Background()

	f := &fakeCoreAdapter{}
	f.subPropsEvent.Store(core.AdapterPropertiesChanged{
		Changed: map[string]any{"Powered": true},
	})

	a := &Adapter{core: f}

	called := false
	var got bool
	_, err := a.SubscribePoweredChanged(ctx, func(b bool) {
		called = true
		got = b
	})
	require.NoError(t, err)
	require.True(t, called)
	require.True(t, got)
}

func TestAdapter_Coverage(t *testing.T) {
	ctx := context.Background()

	t.Run("Unsubscribe", func(t *testing.T) {
		t.Run("NilIsNoop", func(t *testing.T) {
			var u UnsubscribeFunc
			require.NoError(t, u.Unsubscribe())
		})
		t.Run("InvokesUnderlying", func(t *testing.T) {
			sentinel := errors.New("unsub boom")
			called := false
			u := UnsubscribeFunc(func() error {
				called = true
				return sentinel
			})
			require.ErrorIs(t, u.Unsubscribe(), sentinel)
			require.True(t, called)
		})
	})

	t.Run("ModeString", func(t *testing.T) {
		require.Equal(t, "station", ModeStation.String())
		require.Equal(t, "ap", ModeAP.String())
	})

	t.Run("NewAdapter_NilCore", func(t *testing.T) {
		require.Nil(t, newAdapter(nil, "/ignored"))
	})

	t.Run("Path", func(t *testing.T) {
		require.Empty(t, (*Adapter)(nil).Path())
		require.Equal(t, "/net/connman/iwd/phy0", newAdapter(&fakeCoreAdapter{}, "/net/connman/iwd/phy0").Path())
	})

	t.Run("SupportedModesBackendError", func(t *testing.T) {
		f := &fakeCoreAdapter{}
		f.setErr(core.WrapAdapterUnavailable("op", "boom", errors.New("x")))
		_, err := (&Adapter{core: f}).SupportedModes(ctx)
		require.Error(t, err)
		var pe *Error
		require.ErrorAs(t, err, &pe)
		require.Equal(t, ResourceAdapter, pe.Resource)
	})

	t.Run("SetPowered", func(t *testing.T) {
		t.Run("Success", func(t *testing.T) {
			require.NoError(t, (&Adapter{core: &fakeCoreAdapter{}}).SetPowered(ctx, true))
		})
		t.Run("NilReceiver", func(t *testing.T) {
			err := (*Adapter)(nil).SetPowered(ctx, true)
			require.ErrorIs(t, err, ErrInternal)
		})
		t.Run("BackendError", func(t *testing.T) {
			f := &fakeCoreAdapter{}
			f.setErr(core.WrapAdapterUnavailable("op", "boom", errors.New("x")))
			err := (&Adapter{core: f}).SetPowered(ctx, true)
			require.Error(t, err)
			var pe *Error
			require.ErrorAs(t, err, &pe)
			require.Equal(t, ResourceAdapter, pe.Resource)
		})
	})

	t.Run("ParseMode", func(t *testing.T) {
		t.Run("Valid", func(t *testing.T) {
			mode, err := ParseMode("station")
			require.NoError(t, err)
			require.Equal(t, ModeStation, mode)
		})
		t.Run("Invalid", func(t *testing.T) {
			mode, err := ParseMode("bogus")
			require.Error(t, err)
			require.Equal(t, ModeUnknown, mode)
			require.ErrorIs(t, err, ErrInvalidArgument)

			var pe *Error
			require.ErrorAs(t, err, &pe)
			require.Equal(t, ResourceAdapter, pe.Resource)
		})
	})

	// The mode-conversion helpers reject values the lower layers should never
	// deliver (read side) or the caller supplies (write side).
	t.Run("SupportedModesInvalidRejected", func(t *testing.T) {
		f := &fakeCoreAdapter{}
		f.modes.Store([]core.Mode{core.Mode("garbage")})
		_, err := (&Adapter{core: f}).SupportedModes(ctx)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrInvalidArgument)
	})

	t.Run("PropertiesInvalidModeRejected", func(t *testing.T) {
		f := &fakeCoreAdapter{}
		f.name.Store("phy0")
		f.modes.Store([]core.Mode{core.Mode("garbage")})
		_, err := (&Adapter{core: f}).Properties(ctx)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrInvalidArgument)
	})

	t.Run("SupportsModeInvalidArgumentRejected", func(t *testing.T) {
		// The public boundary rejects an unrecognized mode before the backend.
		_, err := (&Adapter{core: &fakeCoreAdapter{}}).SupportsMode(ctx, Mode("garbage"))
		require.Error(t, err)
		require.ErrorIs(t, err, ErrInvalidArgument)

		var pe *Error
		require.ErrorAs(t, err, &pe)
		require.Equal(t, ResourceAdapter, pe.Resource)
	})

	t.Run("SubscribeBackendErrors", func(t *testing.T) {
		t.Run("PropertiesChanged", func(t *testing.T) {
			f := &fakeCoreAdapter{}
			f.setErr(core.WrapAdapterUnavailable("op", "boom", errors.New("x")))
			_, err := (&Adapter{core: f}).SubscribePropertiesChanged(ctx, func(AdapterPropertiesChanged) {})
			require.Error(t, err)
			var pe *Error
			require.ErrorAs(t, err, &pe)
			require.Equal(t, ResourceAdapter, pe.Resource)
		})
		t.Run("PoweredChanged", func(t *testing.T) {
			f := &fakeCoreAdapter{}
			f.setErr(core.WrapAdapterUnavailable("op", "boom", errors.New("x")))
			_, err := (&Adapter{core: f}).SubscribePoweredChanged(ctx, func(bool) {})
			require.Error(t, err)
			var pe *Error
			require.ErrorAs(t, err, &pe)
			require.Equal(t, ResourceAdapter, pe.Resource)
		})
	})
}

func TestAdapter_Properties(t *testing.T) {
	ctx := context.Background()

	t.Run("DelegatesAndConvertsModes", func(t *testing.T) {
		f := &fakeCoreAdapter{}
		f.powered.Store(true)
		f.name.Store("phy0")
		f.model.Store(new("Broadcom"))
		f.modes.Store([]core.Mode{core.ModeStation, core.ModeAP})
		a := newAdapter(f, "/net/connman/iwd/phy0")

		props, err := a.Properties(ctx)
		require.NoError(t, err)
		require.True(t, props.Powered)
		require.Equal(t, "phy0", props.Name)
		require.NotNil(t, props.Model)
		require.Equal(t, "Broadcom", *props.Model)
		require.Nil(t, props.Vendor)
		require.Equal(t, []Mode{ModeStation, ModeAP}, props.SupportedModes)
	})

	t.Run("ErrorPropagates", func(t *testing.T) {
		base := errors.New("boom")
		f := &fakeCoreAdapter{}
		f.setErr(base)
		a := newAdapter(f, "/net/connman/iwd/phy0")

		_, err := a.Properties(ctx)
		require.Error(t, err)
		require.ErrorIs(t, err, base)
	})
}
