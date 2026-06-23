//go:build unit

package iwdbus

import (
	"context"
	"fmt"
	"testing"

	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/require"
)

func TestBasicServiceSet_Iwdbus(t *testing.T) {
	t.Parallel()

	t.Run("GetAddress", testBSS_GetAddress)
	t.Run("GetAddress_WrongType", testBSS_GetAddress_WrongType)
	t.Run("GetAddress_NoIntro", testBSS_GetAddress_NoIntro)
	t.Run("GetAddress_Err", testBSS_GetAddress_Err)
	t.Run("GetProperties", testBSS_GetProperties)
	t.Run("GetProperties_Errors", testBSS_GetProperties_Errors)
	t.Run("GetProperties_NoIntro", testBSS_GetProperties_NoIntro)
}

func testBSS_GetAddress(t *testing.T) {
	t.Parallel()

	b := &BasicServiceSet{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			require.Equal(t, IwdBasicServiceSetIface, iface)
			require.Equal(t, "Address", prop)
			return "11:22:33:44:55:66", nil
		},
	}}

	addr, err := b.GetAddress(context.Background())
	require.NoError(t, err)
	require.Equal(t, "11:22:33:44:55:66", addr)
}

func testBSS_GetAddress_WrongType(t *testing.T) {
	t.Parallel()

	b := &BasicServiceSet{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			return 123, nil
		},
	}}

	_, err := b.GetAddress(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "expected string")
}

func testBSS_GetAddress_NoIntro(t *testing.T) {
	t.Parallel()

	b := &BasicServiceSet{call: nil}

	_, err := b.GetAddress(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "basic service set is not initialized")
}

func testBSS_GetAddress_Err(t *testing.T) {
	t.Parallel()

	b := &BasicServiceSet{call: &fakeCaller{
		getPropFn: func(ctx context.Context, iface, prop string) (interface{}, error) {
			return nil, fmt.Errorf("dbus failure")
		},
	}}

	_, err := b.GetAddress(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "dbus failure")
}

func testBSS_GetProperties(t *testing.T) {
	t.Parallel()

	b := &BasicServiceSet{call: &fakeCaller{
		getAllFn: func(_ context.Context, iface string) (map[string]dbus.Variant, error) {
			require.Equal(t, IwdBasicServiceSetIface, iface)
			return map[string]dbus.Variant{
				"Address": dbus.MakeVariant("11:22:33:44:55:66"),
			}, nil
		},
	}}

	props, err := b.GetProperties(context.Background())
	require.NoError(t, err)
	require.Equal(t, "11:22:33:44:55:66", props.Address)
}

func testBSS_GetProperties_Errors(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		props        map[string]dbus.Variant
		callErr      error
		wantContains string
	}{
		{name: "missing Address", props: map[string]dbus.Variant{}, wantContains: "property=Address"},
		{name: "Address wrong type", props: map[string]dbus.Variant{"Address": dbus.MakeVariant(123)}, wantContains: "expected string"},
		{name: "GetAll call error", callErr: fmt.Errorf("dbus failure"), wantContains: "dbus failure"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			b := &BasicServiceSet{call: &fakeCaller{
				getAllFn: func(_ context.Context, _ string) (map[string]dbus.Variant, error) {
					if tc.callErr != nil {
						return nil, tc.callErr
					}
					return tc.props, nil
				},
			}}

			_, err := b.GetProperties(context.Background())
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.wantContains)
		})
	}
}

func testBSS_GetProperties_NoIntro(t *testing.T) {
	t.Parallel()

	b := &BasicServiceSet{call: nil}

	_, err := b.GetProperties(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "basic service set is not initialized")
}
