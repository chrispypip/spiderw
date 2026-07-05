//go:build unit

package iwdbus

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/require"
)

func TestAdapter_Iwdbus(t *testing.T) {
	t.Parallel()

	t.Run("Parse", func(t *testing.T) {
		t.Parallel()
		t.Run("ParseSupportedModes", testParseSupportedModes)
		t.Run("ParseOptionalString", testParseOptionalString)
		t.Run("ParseMode_ValidModes", testParseMode_ValidModes)
		t.Run("ParseMode_Invalid", testParseMode_Invalid)
		t.Run("ParseMode_RoundTrip", testParseMode_RoundTrip)
	})

	t.Run("SupportsMode", func(t *testing.T) {
		t.Parallel()
		t.Run("SupportsMode_Supported", testSupportsMode_Supported)
		t.Run("SupportsMode_NotSupported", testSupportsMode_NotSupported)
		t.Run("SupportsMode_InvalidMode", testSupportsMode_InvalidMode)
	})

	t.Run("AdapterGetters", func(t *testing.T) {
		t.Parallel()
		t.Run("Adapter_GetPowered", testAdapter_GetPowered)
		t.Run("Adapter_GetPoweredTimeout", testAdapter_GetPoweredTimeout)
		t.Run("Adapter_GetPowered_WrongType", testAdapter_GetPowered_WrongType)
		t.Run("Adapter_GetPowered_NoIntro", testAdapter_GetPowered_NoIntro)
		t.Run("Adapter_GetPowered_Err", testAdapter_GetPowered_Err)
		t.Run("Adapter_GetName", testAdapter_GetName)
		t.Run("Adapter_GetNameTimeout", testAdapter_GetNameTimeout)
		t.Run("Adapter_GetName_WrongType", testAdapter_GetName_WrongType)
		t.Run("Adapter_GetName_NoIntro", testAdapter_GetName_NoIntro)
		t.Run("Adapter_GetName_Err", testAdapter_GetName_Err)
		t.Run("Adapter_GetModel_Valid", testAdapter_GetModel_Valid)
		t.Run("Adapter_GetModel_Nil", testAdapter_GetModel_Nil)
		t.Run("Adapter_GetModelTimeout", testAdapter_GetModelTimeout)
		t.Run("Adapter_GetModel_WrongType", testAdapter_GetModel_WrongType)
		t.Run("Adapter_GetModel_NoIntro", testAdapter_GetModel_NoIntro)
		t.Run("Adapter_GetModel_Err", testAdapter_GetModel_Err)
		t.Run("Adapter_GetVendor_Valid", testAdapter_GetVendor_Valid)
		t.Run("Adapter_GetVendor_NoIntro", testAdapter_GetVendor_NoIntro)
		t.Run("Adapter_GetVendor_Nil", testAdapter_GetVendor_Nil)
		t.Run("Adapter_GetVendorTimeout", testAdapter_GetVendorTimeout)
		t.Run("Adapter_GetVendor_WrongType", testAdapter_GetVendor_WrongType)
		t.Run("Adapter_GetVendor_Err", testAdapter_GetVendor_Err)
		t.Run("Adapter_GetSupportedModes", testAdapter_GetSupportedModes)
		t.Run("Adapter_GetSupportedModes_Empty", testAdapter_GetSupportedModes_Empty)
		t.Run("Adapter_GetSupportedModes_Nil", testAdapter_GetSupportedModes_Nil)
		t.Run("Adapter_GetSupportedModesTimeout", testAdapter_GetSupportedModesTimeout)
		t.Run("Adapter_GetSupportedModes_WrongType", testAdapter_GetSupportedModes_WrongType)
		t.Run("Adapter_GetSupportedModes_NoIntro", testAdapter_GetSupportedModes_NoIntro)
		t.Run("Adapter_GetSupportedModes_Err", testAdapter_GetSupportedModes_Err)
	})

	t.Run("AdapterSupports", func(t *testing.T) {
		t.Parallel()
		t.Run("Adapter_SupportsMode_Valid", testAdapter_SupportsMode_Valid)
		t.Run("Adapter_SupportsModeTimeout", testAdapter_SupportsModeTimeout)
		t.Run("Adapter_SupportsMode_Invalid", testAdapter_SupportsMode_Invalid)
		t.Run("Adapter_SupportsMode_GetSupportedModesError", testAdapter_SupportsMode_GetSupportedModesError)
		t.Run("Adapter_SupportsMode_NoIntro", testAdapter_SupportsMode_NoIntro)
		t.Run("Adapter_SupportsMode_Concurrent", testAdapter_SupportsMode_Concurrent)
		t.Run("Adapter_SupportsStation", testAdapter_SupportsStation)
		t.Run("Adapter_SupportsStationMultiple", testAdapter_SupportsStationMultiple)
		t.Run("Adapter_SupportsStation_NoIntro", testAdapter_SupportsStation_NoIntro)
		t.Run("Adapter_SupportsAP", testAdapter_SupportsAP)
		t.Run("Adapter_SupportsAPMultiple", testAdapter_SupportsAPMultiple)
		t.Run("Adapter_SupportsAP_NoIntro", testAdapter_SupportsAP_NoIntro)
		t.Run("Adapter_SupportsAdHoc", testAdapter_SupportsAdHoc)
		t.Run("Adapter_SupportsAdHocMultiple", testAdapter_SupportsAdHocMultiple)
		t.Run("Adapter_SupportsAdHoc_NoIntro", testAdapter_SupportsAdHoc_NoIntro)
		t.Run("Adapter_SupportsStation_Timeout", testAdapter_SupportsStation_Timeout)
		t.Run("Adapter_SupportsAP_Timeout", testAdapter_SupportsAP_Timeout)
		t.Run("Adapter_SupportsAdHoc_Timeout", testAdapter_SupportsAdHoc_Timeout)
	})

	t.Run("AdapterSet", func(t *testing.T) {
		t.Parallel()
		t.Run("Adapter_SetPowered", testAdapter_SetPowered)
		t.Run("Adapter_SetPoweredTimeout", testAdapter_SetPoweredTimeout)
		t.Run("Adapter_SetPowered_Err", testAdapter_SetPowered_Err)
	})

	t.Run("Subscribe", func(t *testing.T) {
		t.Parallel()
		t.Run("Adapter_SubscribePropertiesChanged", testAdapter_SubscribePropertiesChanged)
		t.Run("Adapter_SubscribePoweredChanged", testAdapter_SubscribePoweredChanged)
		t.Run("Adapter_SubscribePoweredChanged_IgnoresUnrelated", testAdapter_SubscribePoweredChanged_IgnoresUnrelated)
		t.Run("Adapter_SubscribePropertiesChanged_Empty", testAdapter_SubscribePropertiesChanged_Empty)
		t.Run("Adapter_SubscribePropertiesChanged_WrongVariantTypes", testAdapter_SubscribePropertiesChanged_WrongVariantTypes)
		t.Run("Adapter_SubscribePropertiesChanged_MultipleHandlers", testAdapter_SubscribePropertiesChanged_MultipleHandlers)
		t.Run("Adapter_SubscribePoweredChanged_Unsubscribe", testAdapter_SubscribePoweredChanged_Unsubscribe)
	})

	t.Run("Firehose", func(t *testing.T) {
		t.Parallel()
		t.Run("Adapter_FirehoseReceivesAll", testAdapter_FirehoseReceivesAll)
		t.Run("Adapter_FirehosePropertiesChanged", testAdapter_FirehosePropertiesChanged)
	})
}

func TestAdapter_GetModel_AbsentOptionalCollapsesToNil(t *testing.T) {
	t.Parallel()

	a := &Adapter{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			return nil, fmt.Errorf("Getting property value failed")
		},
	}}

	model, err := a.GetModel(context.Background())
	require.NoError(t, err)
	require.Nil(t, model)
}

func TestAdapter_GetVendor_AbsentOptionalCollapsesToNil(t *testing.T) {
	t.Parallel()

	a := &Adapter{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			return nil, fmt.Errorf("Getting property value failed")
		},
	}}

	vendor, err := a.GetVendor(context.Background())
	require.NoError(t, err)
	require.Nil(t, vendor)
}

func TestAdapter_GetProperties_OptionalsAbsent(t *testing.T) {
	t.Parallel()

	a := newGetAllAdapter(func(ctx context.Context, iface string) (map[string]dbus.Variant, error) {
		return map[string]dbus.Variant{
			"Powered":        dbus.MakeVariant(false),
			"Name":           dbus.MakeVariant("phy1"),
			"SupportedModes": dbus.MakeVariant([]string{"station"}),
		}, nil
	})

	props, err := a.GetProperties(context.Background())
	require.NoError(t, err)
	require.False(t, props.Powered)
	require.Equal(t, "phy1", props.Name)
	require.Nil(t, props.Model)
	require.Nil(t, props.Vendor)
	require.Equal(t, []Mode{ModeStation}, props.SupportedModes)
}

func TestAdapter_GetProperties_Errors(t *testing.T) {
	t.Parallel()

	full := func() map[string]dbus.Variant {
		return map[string]dbus.Variant{
			"Powered":        dbus.MakeVariant(true),
			"Name":           dbus.MakeVariant("phy0"),
			"SupportedModes": dbus.MakeVariant([]string{"station"}),
		}
	}

	cases := []struct {
		name         string
		props        map[string]dbus.Variant
		callErr      error
		wantContains string
	}{
		{
			name: "missing Powered",
			props: map[string]dbus.Variant{
				"Name":           dbus.MakeVariant("phy0"),
				"SupportedModes": dbus.MakeVariant([]string{"station"}),
			},
			wantContains: "property=Powered",
		},
		{
			name: "missing Name",
			props: map[string]dbus.Variant{
				"Powered":        dbus.MakeVariant(true),
				"SupportedModes": dbus.MakeVariant([]string{"station"}),
			},
			wantContains: "property=Name",
		},
		{
			name: "missing SupportedModes",
			props: map[string]dbus.Variant{
				"Powered": dbus.MakeVariant(true),
				"Name":    dbus.MakeVariant("phy0"),
			},
			wantContains: "property=SupportedModes",
		},
		{
			name: "Powered wrong type",
			props: func() map[string]dbus.Variant {
				m := full()
				m["Powered"] = dbus.MakeVariant("nope")
				return m
			}(),
			wantContains: "expected bool",
		},
		{
			name: "Name wrong type",
			props: func() map[string]dbus.Variant {
				m := full()
				m["Name"] = dbus.MakeVariant(123)
				return m
			}(),
			wantContains: "expected string",
		},
		{
			name: "SupportedModes wrong type",
			props: func() map[string]dbus.Variant {
				m := full()
				m["SupportedModes"] = dbus.MakeVariant(42)
				return m
			}(),
			wantContains: "unexpected type",
		},
		{
			name:         "GetAll call error",
			callErr:      fmt.Errorf("dbus failure"),
			wantContains: "dbus failure",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			a := newGetAllAdapter(func(ctx context.Context, iface string) (map[string]dbus.Variant, error) {
				if tc.callErr != nil {
					return nil, tc.callErr
				}
				return tc.props, nil
			})

			_, err := a.GetProperties(context.Background())
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.wantContains)
		})
	}
}

func testParseSupportedModes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		in              any
		want            []Mode
		wantErrContains []string
		wantRoundTrip   []string
	}{
		{
			name:          "valid string slice",
			in:            []string{"station", "ap", "ad-hoc"},
			want:          []Mode{ModeStation, ModeAP, ModeAdHoc},
			wantRoundTrip: []string{"station", "ap", "ad-hoc"},
		},
		{
			name: "valid interface slice",
			in:   []interface{}{"ap", "ad-hoc"},
			want: []Mode{ModeAP, ModeAdHoc},
		},
		{
			name:            "wrong type",
			in:              123,
			wantErrContains: []string{"dbus variant conversion error", "unexpected type"},
		},
		{
			name:            "invalid string",
			in:              []string{"bad-mode"},
			wantErrContains: []string{"dbus variant conversion error", "invalid mode"},
		},
		{
			name:            "mixed valid invalid",
			in:              []string{"station", "bad-mode"},
			wantErrContains: []string{"dbus variant conversion error", "invalid mode"},
		},
		{
			name:            "interface wrong inner type",
			in:              []interface{}{1234},
			wantErrContains: []string{"dbus variant conversion error", "expected string"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			modes, err := parseSupportedModes(tc.in)
			if len(tc.wantErrContains) > 0 {
				require.Error(t, err)
				for _, sub := range tc.wantErrContains {
					require.Contains(t, err.Error(), sub)
				}
				return
			}
			require.NoError(t, err)
			require.ElementsMatch(t, modes, tc.want)
			if len(tc.wantRoundTrip) > 0 {
				require.Len(t, modes, len(tc.wantRoundTrip))
				for i, want := range tc.wantRoundTrip {
					require.Equal(t, want, modes[i].String())
				}
			}
		})
	}
}

func testParseOptionalString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		in              any
		want            *string
		wantErrContains []string
	}{
		{
			name: "string",
			in:   "Broadcom",
			want: new("Broadcom"),
		},
		{
			name: "variant string",
			in:   dbus.MakeVariant("Intel"),
			want: new("Intel"),
		},
		{
			name: "variant wrong type",
			in:   dbus.MakeVariant(1234),
			wantErrContains: []string{
				"expected string variant",
				"got int inside variant",
			},
		},
		{
			name: "invalid type",
			in:   123,
			wantErrContains: []string{
				"expected string or variant(string)",
				"got int",
			},
		},
		{
			name: "nil",
			in:   nil,
			want: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseOptionalString(tc.in)
			if len(tc.wantErrContains) > 0 {
				require.Error(t, err)
				for _, sub := range tc.wantErrContains {
					require.Contains(t, err.Error(), sub)
				}
				return
			}
			require.NoError(t, err)
			if tc.want == nil {
				require.Nil(t, got)
				return
			}
			require.NotNil(t, got)
			require.Equal(t, *tc.want, *got)
		})
	}
}

func testSupportsMode_Supported(t *testing.T) {
	t.Parallel()

	supported, err := isModeSupported([]Mode{
		ModeStation,
		ModeAP,
		ModeAdHoc,
	}, ModeStation)
	require.NoError(t, err, "failed to parse supported modes")
	require.True(t, supported)
}

func testSupportsMode_NotSupported(t *testing.T) {
	t.Parallel()

	supported, err := isModeSupported([]Mode{
		ModeStation,
		ModeAP,
	}, ModeAdHoc)
	require.NoError(t, err, "failed to parse supported modes")
	require.False(t, supported)
}

func testSupportsMode_InvalidMode(t *testing.T) {
	t.Parallel()

	_, err := isModeSupported([]Mode{ModeAP, ModeAdHoc}, ModeUnknown)
	require.Error(t, err, "expected error")
}

func testParseMode_ValidModes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  Mode
	}{
		{"station", ModeStation},
		{"ap", ModeAP},
		{"ad-hoc", ModeAdHoc},
	}
	for _, tt := range tests {
		mode, err := ParseMode(tt.input)
		require.NoError(t, err, "input=%q", tt.input)
		require.Equal(t, tt.want, mode)
	}
}

func testParseMode_Invalid(t *testing.T) {
	t.Parallel()

	tests := []string{
		"bad-mode",
		"adhoc",
		"STATION",
		"",
	}
	for _, input := range tests {
		mode, err := ParseMode(input)
		require.Error(t, err, "expected error for input=%q", input)
		require.Equal(t, ModeUnknown, mode)
	}
}

func testParseMode_RoundTrip(t *testing.T) {
	t.Parallel()

	modes := []Mode{
		ModeStation,
		ModeAP,
		ModeAdHoc,
	}
	for _, m := range modes {
		s := m.String()
		parsed, err := ParseMode(s)
		require.NoError(t, err, "round trip failed for %q", s)
		require.Equal(t, m, parsed)
	}
}

func testAdapter_GetPowered(t *testing.T) {
	t.Parallel()

	a := &Adapter{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			require.Equal(t, "Powered", prop)
			return true, nil
		},
	}}

	val, err := a.GetPowered(context.Background())
	require.NoError(t, err)
	require.True(t, val)
}

func testAdapter_GetPoweredTimeout(t *testing.T) {
	t.Parallel()

	a := &Adapter{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			select {
			case <-time.After(1 * time.Second):
				return true, nil
			case <-ctx.Done():
				return false, ctx.Err()
			}
		},
	}}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	t.Cleanup(func() { cancel() })

	_, err := a.GetPowered(ctx)
	require.Error(t, err)
}

func testAdapter_GetPowered_WrongType(t *testing.T) {
	t.Parallel()

	a := &Adapter{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			return "not-bool", nil
		},
	}}

	_, err := a.GetPowered(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "dbus variant conversion error")
	require.Contains(t, err.Error(), "expected bool")
}

func testAdapter_GetPowered_NoIntro(t *testing.T) {
	t.Parallel()

	a := &Adapter{call: nil}

	_, err := a.GetPowered(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "adapter is not initialized")
}

func testAdapter_GetPowered_Err(t *testing.T) {
	t.Parallel()

	a := &Adapter{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			return nil, fmt.Errorf("dbus failure")
		},
	}}

	_, err := a.GetPowered(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "dbus failure")
}

func testAdapter_GetName(t *testing.T) {
	t.Parallel()

	a := &Adapter{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			return "phy0", nil
		},
	}}

	name, err := a.GetName(context.Background())
	require.NoError(t, err)
	require.Equal(t, "phy0", name)
}

func testAdapter_GetNameTimeout(t *testing.T) {
	t.Parallel()

	a := &Adapter{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			select {
			case <-time.After(1 * time.Second):
				return "phy0", nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		},
	}}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	t.Cleanup(func() { cancel() })

	_, err := a.GetName(ctx)
	require.Error(t, err)
}

func testAdapter_GetName_WrongType(t *testing.T) {
	t.Parallel()

	a := &Adapter{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			return 123, nil
		},
	}}

	_, err := a.GetName(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "dbus variant conversion error")
	require.Contains(t, err.Error(), "expected string")
}

func testAdapter_GetName_NoIntro(t *testing.T) {
	t.Parallel()

	a := &Adapter{call: nil}

	_, err := a.GetName(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "adapter is not initialized")
}

func testAdapter_GetName_Err(t *testing.T) {
	t.Parallel()

	a := &Adapter{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			return nil, fmt.Errorf("dbus failure")
		},
	}}

	_, err := a.GetName(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "dbus failure")
}

func testAdapter_GetModel_Valid(t *testing.T) {
	t.Parallel()

	a := &Adapter{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			return "Intel", nil
		},
	}}

	model, err := a.GetModel(context.Background())
	require.NoError(t, err)
	require.NotNil(t, model)
	require.Equal(t, "Intel", *model)
}

func testAdapter_GetModel_Nil(t *testing.T) {
	t.Parallel()

	a := &Adapter{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			return nil, nil
		},
	}}

	model, err := a.GetModel(context.Background())
	require.NoError(t, err)
	require.Nil(t, model)
}

func testAdapter_GetModelTimeout(t *testing.T) {
	t.Parallel()

	a := &Adapter{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			select {
			case <-time.After(1 * time.Second):
				return "Intel", nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		},
	}}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	t.Cleanup(func() { cancel() })

	_, err := a.GetModel(ctx)
	require.Error(t, err)
}

func testAdapter_GetModel_WrongType(t *testing.T) {
	t.Parallel()

	a := &Adapter{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			return 123, nil
		},
	}}

	_, err := a.GetModel(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "dbus variant conversion error")
	require.Contains(t, err.Error(), "expected string or variant(string)")
}

func testAdapter_GetModel_NoIntro(t *testing.T) {
	t.Parallel()

	a := &Adapter{call: nil}

	_, err := a.GetModel(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "adapter is not initialized")
}

func testAdapter_GetModel_Err(t *testing.T) {
	t.Parallel()

	a := &Adapter{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			return nil, fmt.Errorf("dbus failure")
		},
	}}

	_, err := a.GetModel(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "dbus failure")
}

func testAdapter_GetVendor_Valid(t *testing.T) {
	t.Parallel()

	a := &Adapter{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			return "Broadcom", nil
		},
	}}

	model, err := a.GetVendor(context.Background())
	require.NoError(t, err)
	require.NotNil(t, model)
	require.Equal(t, "Broadcom", *model)
}

func testAdapter_GetVendor_NoIntro(t *testing.T) {
	t.Parallel()

	a := &Adapter{call: nil}

	_, err := a.GetVendor(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "adapter is not initialized")
}

func testAdapter_GetVendor_Nil(t *testing.T) {
	t.Parallel()

	a := &Adapter{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			return nil, nil
		},
	}}

	model, err := a.GetVendor(context.Background())
	require.NoError(t, err)
	require.Nil(t, model)
}

func testAdapter_GetVendorTimeout(t *testing.T) {
	t.Parallel()

	a := &Adapter{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			select {
			case <-time.After(1 * time.Second):
				return "Broadcom", nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		},
	}}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	t.Cleanup(func() { cancel() })

	_, err := a.GetVendor(ctx)
	require.Error(t, err)
}

func testAdapter_GetVendor_WrongType(t *testing.T) {
	t.Parallel()

	a := &Adapter{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			return 123, nil
		},
	}}

	_, err := a.GetVendor(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "dbus variant conversion error")
	require.Contains(t, err.Error(), "expected string or variant(string)")
}

func testAdapter_GetVendor_Err(t *testing.T) {
	t.Parallel()

	a := &Adapter{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			return nil, fmt.Errorf("dbus failure")
		},
	}}

	_, err := a.GetVendor(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "dbus failure")
}

func testAdapter_GetSupportedModes(t *testing.T) {
	t.Parallel()

	a := &Adapter{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			require.Equal(t, "SupportedModes", prop)
			return []string{"station", "ap"}, nil
		},
	}}

	modes, err := a.GetSupportedModes(context.Background())
	require.NoError(t, err)
	require.Equal(t, []Mode{ModeStation, ModeAP}, modes)
}

func testAdapter_GetSupportedModes_Empty(t *testing.T) {
	t.Parallel()

	a := &Adapter{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			return []string{}, nil
		},
	}}

	modes, err := a.GetSupportedModes(context.Background())
	require.NoError(t, err)
	require.Empty(t, modes)
}

func testAdapter_GetSupportedModes_Nil(t *testing.T) {
	t.Parallel()

	a := &Adapter{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			return nil, nil
		},
	}}

	_, err := a.GetSupportedModes(context.Background())
	require.Error(t, err)
}

func testAdapter_GetSupportedModesTimeout(t *testing.T) {
	t.Parallel()

	a := &Adapter{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			select {
			case <-time.After(1 * time.Second):
				return []string{"station", "ap"}, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		},
	}}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	t.Cleanup(func() { cancel() })

	_, err := a.GetSupportedModes(ctx)
	require.Error(t, err)
}

func testAdapter_GetSupportedModes_WrongType(t *testing.T) {
	t.Parallel()

	a := &Adapter{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			return "not-bool", nil
		},
	}}

	_, err := a.GetSupportedModes(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "dbus variant conversion error")
	require.Contains(t, err.Error(), "unexpected type string")
}

func testAdapter_GetSupportedModes_NoIntro(t *testing.T) {
	t.Parallel()

	a := &Adapter{call: nil}

	_, err := a.GetSupportedModes(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "adapter is not initialized")
}

func testAdapter_GetSupportedModes_Err(t *testing.T) {
	t.Parallel()

	a := &Adapter{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			return nil, fmt.Errorf("dbus failure")
		},
	}}

	_, err := a.GetSupportedModes(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "dbus failure")
}

func testAdapter_SupportsMode_Valid(t *testing.T) {
	t.Parallel()

	a := &Adapter{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			return []string{"station", "ap"}, nil
		},
	}}

	ok, err := a.SupportsMode(context.Background(), ModeStation)
	require.NoError(t, err)
	require.True(t, ok)
}

func testAdapter_SupportsModeTimeout(t *testing.T) {
	t.Parallel()

	a := &Adapter{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			select {
			case <-time.After(1 * time.Second):
				return []string{"station", "ap"}, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		},
	}}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	t.Cleanup(func() { cancel() })

	_, err := a.SupportsMode(ctx, ModeStation)
	require.Error(t, err)
}

func testAdapter_SupportsMode_Invalid(t *testing.T) {
	t.Parallel()

	a := &Adapter{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			return []string{"station"}, nil
		},
	}}

	_, err := a.SupportsMode(context.Background(), ModeUnknown)
	require.Error(t, err)
}

func testAdapter_SupportsMode_GetSupportedModesError(t *testing.T) {
	t.Parallel()

	a := &Adapter{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			return nil, fmt.Errorf("dbus failure")
		},
	}}

	_, err := a.SupportsMode(context.Background(), ModeStation)
	require.Error(t, err)
	require.Contains(t, err.Error(), "dbus failure")
}

func testAdapter_SupportsMode_NoIntro(t *testing.T) {
	t.Parallel()

	a := &Adapter{call: nil}

	_, err := a.SupportsMode(context.Background(), ModeStation)
	require.Error(t, err)
	require.Contains(t, err.Error(), "adapter is not initialized")
}

func testAdapter_SupportsMode_Concurrent(t *testing.T) {
	a := &Adapter{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			return []string{"station", "ap"}, nil
		},
	}}

	wg := sync.WaitGroup{}
	ctx := context.Background()

	for range 50 {
		wg.Go(func() {
			ok, err := a.SupportsMode(ctx, ModeAP)
			require.NoError(t, err)
			require.True(t, ok)
		})
	}

	wg.Wait()
}

func testAdapter_SetPowered(t *testing.T) {
	t.Parallel()

	var called bool
	a := &Adapter{call: &fakeCaller{
		setPropFn: func(ctx context.Context, iface, prop string, val interface{}) error {
			called = true
			require.Equal(t, "Powered", prop)
			require.Equal(t, true, val)
			return nil
		},
	}}

	err := a.SetPowered(context.Background(), true)
	require.NoError(t, err)
	require.True(t, called)
}

func testAdapter_SetPoweredTimeout(t *testing.T) {
	t.Parallel()

	a := &Adapter{call: &fakeCaller{
		setPropFn: func(ctx context.Context, iface, prop string, val interface{}) error {
			select {
			case <-time.After(1 * time.Second):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	}}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	t.Cleanup(func() { cancel() })

	err := a.SetPowered(ctx, true)
	require.Error(t, err)
}

func testAdapter_SetPowered_Err(t *testing.T) {
	t.Parallel()

	a := &Adapter{call: &fakeCaller{
		setPropFn: func(ctx context.Context, iface, prop string, val interface{}) error {
			return fmt.Errorf("dbus failure")
		},
	}}

	err := a.SetPowered(context.Background(), true)
	require.Error(t, err)
	require.Contains(t, err.Error(), "dbus failure")
}

func testAdapter_SupportsStation(t *testing.T) {
	t.Parallel()

	a := &Adapter{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			return []string{"station", "ap"}, nil
		},
	}}

	ok, err := a.SupportsStation(context.Background())
	require.NoError(t, err)
	require.True(t, ok)
}

func testAdapter_SupportsStationMultiple(t *testing.T) {
	t.Parallel()

	a := &Adapter{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			return []string{"station", "station", "ap"}, nil
		},
	}}

	ok, err := a.SupportsStation(context.Background())
	require.NoError(t, err)
	require.True(t, ok)
}

func testAdapter_SupportsStation_NoIntro(t *testing.T) {
	t.Parallel()

	a := &Adapter{call: nil}

	_, err := a.SupportsStation(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "adapter is not initialized")
}

func testAdapter_SupportsAP(t *testing.T) {
	t.Parallel()

	a := &Adapter{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			return []string{"station", "ap"}, nil
		},
	}}

	ok, err := a.SupportsAP(context.Background())
	require.NoError(t, err)
	require.True(t, ok)
}

func testAdapter_SupportsAPMultiple(t *testing.T) {
	t.Parallel()

	a := &Adapter{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			return []string{"station", "ap", "ap"}, nil
		},
	}}

	ok, err := a.SupportsStation(context.Background())
	require.NoError(t, err)
	require.True(t, ok)
}

func testAdapter_SupportsAP_NoIntro(t *testing.T) {
	t.Parallel()

	a := &Adapter{call: nil}

	_, err := a.SupportsAP(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "adapter is not initialized")
}

func testAdapter_SupportsAdHoc(t *testing.T) {
	t.Parallel()

	a := &Adapter{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			return []string{"station", "ap"}, nil
		},
	}}

	ok, err := a.SupportsAdHoc(context.Background())
	require.NoError(t, err)
	require.False(t, ok)
}

func testAdapter_SupportsAdHocMultiple(t *testing.T) {
	t.Parallel()

	a := &Adapter{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			return []string{"station", "ad-hoc", "ad-hoc"}, nil
		},
	}}

	ok, err := a.SupportsStation(context.Background())
	require.NoError(t, err)
	require.True(t, ok)
}

func testAdapter_SupportsAdHoc_NoIntro(t *testing.T) {
	t.Parallel()

	a := &Adapter{call: nil}

	_, err := a.SupportsAdHoc(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "adapter is not initialized")
}

func testAdapter_SupportsStation_Timeout(t *testing.T) {
	t.Parallel()

	a := &Adapter{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			select {
			case <-time.After(1 * time.Second):
				return []string{"station", "ap"}, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		},
	}}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	t.Cleanup(func() { cancel() })

	_, err := a.SupportsStation(ctx)
	require.Error(t, err)
}

func testAdapter_SupportsAP_Timeout(t *testing.T) {
	t.Parallel()

	a := &Adapter{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			select {
			case <-time.After(1 * time.Second):
				return []string{"station", "ap"}, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		},
	}}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	t.Cleanup(func() { cancel() })

	_, err := a.SupportsAP(ctx)
	require.Error(t, err)
}

func testAdapter_SupportsAdHoc_Timeout(t *testing.T) {
	t.Parallel()

	a := &Adapter{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			select {
			case <-time.After(1 * time.Second):
				return []string{"station", "ap"}, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		},
	}}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	t.Cleanup(func() { cancel() })

	_, err := a.SupportsAdHoc(ctx)
	require.Error(t, err)
}

func testAdapter_SubscribePropertiesChanged(t *testing.T) {
	t.Parallel()

	fake := newFakeSignalSource(t)
	ctx := context.Background()
	adapter := &Adapter{signals: fake}

	var recv AdapterPropertiesChanged
	fired := make(chan struct{}, 1)

	_, err := adapter.SubscribePropertiesChanged(ctx, func(changed AdapterPropertiesChanged) {
		recv = changed
		fired <- struct{}{}
	})
	require.NoError(t, err)

	changed := map[string]dbus.Variant{
		"Powered": dbus.MakeVariant(true),
		"Name":    dbus.MakeVariant("phy0"),
	}
	invalid := []string{"Vendor"}
	fake.emit("org.freedesktop.DBus.Properties", "PropertiesChanged", IwdAdapterIface, changed, invalid)

	requireFired(t, fired)
	require.Equal(t, changed, recv.Changed)
	require.Equal(t, invalid, recv.Invalidated)
}

func testAdapter_SubscribePoweredChanged(t *testing.T) {
	t.Parallel()

	fake := newFakeSignalSource(t)
	ctx := context.Background()
	adapter := &Adapter{signals: fake}

	var recv bool
	fired := make(chan struct{}, 1)

	_, err := adapter.SubscribePoweredChanged(ctx, func(v bool) {
		recv = v
		fired <- struct{}{}
	})
	require.NoError(t, err)

	changed := map[string]dbus.Variant{
		"Powered": dbus.MakeVariant(false),
	}
	fake.emit("org.freedesktop.DBus.Properties", "PropertiesChanged", IwdAdapterIface, changed, nil)

	requireFired(t, fired)
	require.False(t, recv)
}

func testAdapter_SubscribePoweredChanged_Unsubscribe(t *testing.T) {
	t.Parallel()

	fake := newFakeSignalSource(t)
	ctx := context.Background()
	adapter := &Adapter{signals: fake}

	fired := make(chan struct{}, 2)

	unsubscribe, err := adapter.SubscribePoweredChanged(ctx, func(bool) {
		fired <- struct{}{}
	})

	require.NoError(t, err)
	require.NotNil(t, unsubscribe)

	changed := map[string]dbus.Variant{
		"Powered": dbus.MakeVariant(true),
	}
	fake.emit("org.freedesktop.DBus.Properties", "PropertiesChanged", IwdAdapterIface, changed, nil)
	requireFired(t, fired)

	require.NoError(t, unsubscribe.Unsubscribe())
	require.NoError(t, unsubscribe.Unsubscribe())

	fake.emit("org.freedesktop.DBus.Properties", "PropertiesChanged", IwdAdapterIface, changed, nil)
	requireNotFired(t, fired)
}

func testAdapter_SubscribePoweredChanged_IgnoresUnrelated(t *testing.T) {
	t.Parallel()

	fake := newFakeSignalSource(t)
	ctx := context.Background()
	adapter := &Adapter{signals: fake}

	fired := make(chan struct{}, 1)

	_, err := adapter.SubscribePoweredChanged(ctx, func(v bool) {
		fired <- struct{}{}
	})
	require.NoError(t, err)

	fake.emit("org.freedesktop.DBus.Properties", "PropertiesChanged", IwdAdapterIface, map[string]dbus.Variant{
		"Name": dbus.MakeVariant("ignore-me"),
	}, nil)

	requireNotFired(t, fired)
}

func testAdapter_SubscribePropertiesChanged_Empty(t *testing.T) {
	t.Parallel()

	fake := newFakeSignalSource(t)
	ctx := context.Background()
	adapter := &Adapter{signals: fake}

	var recv AdapterPropertiesChanged
	fired := make(chan struct{}, 1)

	_, err := adapter.SubscribePropertiesChanged(ctx, func(ev AdapterPropertiesChanged) {
		recv = ev
		fired <- struct{}{}
	})
	require.NoError(t, err)

	fake.emit("org.freedesktop.DBus.Properties", "PropertiesChanged", IwdAdapterIface, map[string]dbus.Variant{}, []string{})

	requireFired(t, fired)
	require.Empty(t, recv.Changed)
	require.Empty(t, recv.Invalidated)
}

func testAdapter_SubscribePropertiesChanged_WrongVariantTypes(t *testing.T) {
	t.Parallel()

	fake := newFakeSignalSource(t)
	ctx := context.Background()
	adapter := &Adapter{signals: fake}

	var recv AdapterPropertiesChanged
	fired := make(chan struct{}, 1)

	_, err := adapter.SubscribePropertiesChanged(ctx, func(ev AdapterPropertiesChanged) {
		recv = ev
		fired <- struct{}{}
	})
	require.NoError(t, err)

	changed := map[string]dbus.Variant{
		"Powered": dbus.MakeVariant("not-a-bool"),
		"Name":    dbus.MakeVariant(1234),
	}
	fake.emit("org.freedesktop.DBus.Properties", "PropertiesChanged", IwdAdapterIface, changed, nil)

	requireFired(t, fired)
	require.Equal(t, changed, recv.Changed)
}

func testAdapter_SubscribePropertiesChanged_MultipleHandlers(t *testing.T) {
	t.Parallel()

	fake := newFakeSignalSource(t)
	ctx := context.Background()
	adapter := &Adapter{signals: fake}

	a := make(chan struct{}, 1)
	b := make(chan struct{}, 1)

	_, _ = adapter.SubscribePropertiesChanged(ctx, func(ev AdapterPropertiesChanged) {
		a <- struct{}{}
	})
	_, _ = adapter.SubscribePropertiesChanged(ctx, func(ev AdapterPropertiesChanged) {
		b <- struct{}{}
	})

	fake.emit("org.freedesktop.DBus.Properties", "PropertiesChanged", IwdAdapterIface, map[string]dbus.Variant{
		"Powered": dbus.MakeVariant(true),
	}, nil)

	requireFired(t, a)
	requireFired(t, b)
}

func testAdapter_FirehoseReceivesAll(t *testing.T) {
	fake := newFakeSignalSource(t)
	adapter := &Adapter{signals: fake}

	var recv FirehoseSignal
	fired := make(chan struct{}, 1)

	err := adapter.Firehose(context.Background(), func(s FirehoseSignal) {
		recv = s
		fired <- struct{}{}
	})
	require.NoError(t, err)

	fake.emit(
		IwdAdapterIface,
		"PoweredChanged",
		map[string]dbus.Variant{"Powered": dbus.MakeVariant(false)},
		nil,
	)

	requireFired(t, fired)

	require.Equal(t, IwdAdapterIface, recv.Interface)
	require.Equal(t, "PoweredChanged", recv.Member)

	// Body is passed through untouched
	require.Len(t, recv.Body, 2)

	v, ok := recv.Body[0].(map[string]dbus.Variant)
	require.True(t, ok)
	require.Equal(t, dbus.MakeVariant(false), v["Powered"])
}

func testAdapter_FirehosePropertiesChanged(t *testing.T) {
	fake := newFakeSignalSource(t)
	adapter := &Adapter{signals: fake}

	fired := make(chan struct{}, 1)
	var recv FirehoseSignal

	_ = adapter.Firehose(context.Background(), func(s FirehoseSignal) {
		recv = s
		fired <- struct{}{}
	})

	changed := map[string]dbus.Variant{"Powered": dbus.MakeVariant(false)}
	invalid := []string{"Vendor"}

	fake.emit("org.freedesktop.DBus.Properties", "PropertiesChanged", IwdAdapterIface, changed, invalid)

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

	require.Equal(t, IwdAdapterIface, s)
	require.Equal(t, changed, v)
	require.Equal(t, invalid, ss)
}

func newGetAllAdapter(fn func(ctx context.Context, iface string) (map[string]dbus.Variant, error)) *Adapter {
	return &Adapter{call: &fakeCaller{getAllFn: fn}}
}
