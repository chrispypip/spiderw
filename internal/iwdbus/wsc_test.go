//go:build unit

package iwdbus

import (
	"context"
	"fmt"
	"testing"

	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/require"
)

func TestSimpleConfiguration_Iwdbus(t *testing.T) {
	t.Parallel()

	t.Run("PushButton", func(t *testing.T) {
		t.Parallel()
		t.Run("SimpleConfiguration_PushButton", testSimpleConfiguration_PushButton)
		t.Run("SimpleConfiguration_PushButton_Err", testSimpleConfiguration_PushButton_Err)
	})

	t.Run("Pin", func(t *testing.T) {
		t.Parallel()
		t.Run("SimpleConfiguration_GeneratePin", testSimpleConfiguration_GeneratePin)
		t.Run("SimpleConfiguration_GeneratePin_Err", testSimpleConfiguration_GeneratePin_Err)
		t.Run("SimpleConfiguration_GeneratePin_BadReply", testSimpleConfiguration_GeneratePin_BadReply)
		t.Run("SimpleConfiguration_StartPin", testSimpleConfiguration_StartPin)
		t.Run("SimpleConfiguration_StartPin_Err", testSimpleConfiguration_StartPin_Err)
	})

	t.Run("Cancel", func(t *testing.T) {
		t.Parallel()
		t.Run("SimpleConfiguration_Cancel", testSimpleConfiguration_Cancel)
		t.Run("SimpleConfiguration_Cancel_Err", testSimpleConfiguration_Cancel_Err)
	})

	t.Run("Errors", func(t *testing.T) {
		t.Parallel()
		t.Run("SimpleConfiguration_WSCErrorsMatchable", testSimpleConfiguration_WSCErrorsMatchable)
		t.Run("SimpleConfiguration_StartPin_InvalidFormatMatchable", testSimpleConfiguration_StartPin_InvalidFormatMatchable)
	})

	t.Run("NotInitialized", testSimpleConfiguration_NoIntro)
}

func testSimpleConfiguration_PushButton(t *testing.T) {
	t.Parallel()

	var called bool
	c := &SimpleConfiguration{call: &fakeCaller{
		callFn: func(ctx context.Context, iface, method string, args ...interface{}) ([]interface{}, error) {
			called = true
			require.Equal(t, IwdSimpleConfigurationIface, iface)
			require.Equal(t, "PushButton", method)
			require.Empty(t, args)
			return nil, nil
		},
	}}

	require.NoError(t, c.PushButton(context.Background()))
	require.True(t, called)
}

func testSimpleConfiguration_PushButton_Err(t *testing.T) {
	t.Parallel()

	c := &SimpleConfiguration{call: &fakeCaller{
		callFn: func(ctx context.Context, iface, method string, args ...interface{}) ([]interface{}, error) {
			return nil, fmt.Errorf("dbus failure")
		},
	}}

	err := c.PushButton(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "dbus failure")
}

// testSimpleConfiguration_WSCErrorsMatchable proves the WSC-specific errors,
// which are scoped to the SimpleConfiguration interface (not the top-level iwd
// namespace), are registered and surface as matchable sentinels rather than
// falling back to a generic method error.
func testSimpleConfiguration_WSCErrorsMatchable(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		errName string
		want    error
		call    func(context.Context, *SimpleConfiguration) error
	}{
		{"SessionOverlap", IwdErrorWSCSessionOverlap, ErrWSCSessionOverlap, func(ctx context.Context, c *SimpleConfiguration) error { return c.PushButton(ctx) }},
		{"NoCredentials", IwdErrorWSCNoCredentials, ErrWSCNoCredentials, func(ctx context.Context, c *SimpleConfiguration) error { return c.PushButton(ctx) }},
		{"NotReachable", IwdErrorWSCNotReachable, ErrWSCNotReachable, func(ctx context.Context, c *SimpleConfiguration) error { return c.PushButton(ctx) }},
		{"WalkTimeExpired", IwdErrorWSCWalkTimeExpired, ErrWSCWalkTimeExpired, func(ctx context.Context, c *SimpleConfiguration) error { return c.PushButton(ctx) }},
		{"TimeExpired", IwdErrorWSCTimeExpired, ErrWSCTimeExpired, func(ctx context.Context, c *SimpleConfiguration) error { return c.StartPin(ctx, "01234565") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := &SimpleConfiguration{call: &fakeCaller{
				callFn: func(ctx context.Context, iface, method string, args ...interface{}) ([]interface{}, error) {
					return nil, dbus.Error{Name: tc.errName, Body: []interface{}{"boom"}}
				},
			}}

			err := tc.call(context.Background(), c)
			require.Error(t, err)
			require.ErrorIs(t, err, tc.want, "expected %v, got %v", tc.want, err)
			require.ErrorIs(t, err, ErrDBusMethod, "should still classify as a method error")
		})
	}
}

func testSimpleConfiguration_GeneratePin(t *testing.T) {
	t.Parallel()

	c := &SimpleConfiguration{call: &fakeCaller{
		callFn: func(ctx context.Context, iface, method string, args ...interface{}) ([]interface{}, error) {
			require.Equal(t, IwdSimpleConfigurationIface, iface)
			require.Equal(t, "GeneratePin", method)
			require.Empty(t, args)
			return []interface{}{"12345670"}, nil
		},
	}}

	pin, err := c.GeneratePin(context.Background())
	require.NoError(t, err)
	require.Equal(t, "12345670", pin)
}

func testSimpleConfiguration_GeneratePin_Err(t *testing.T) {
	t.Parallel()

	c := &SimpleConfiguration{call: &fakeCaller{
		callFn: func(ctx context.Context, iface, method string, args ...interface{}) ([]interface{}, error) {
			return nil, fmt.Errorf("dbus failure")
		},
	}}

	_, err := c.GeneratePin(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "dbus failure")
}

func testSimpleConfiguration_GeneratePin_BadReply(t *testing.T) {
	t.Parallel()

	// A reply whose shape is not a single string must be reported, not silently
	// accepted as an empty PIN.
	c := &SimpleConfiguration{call: &fakeCaller{
		callFn: func(ctx context.Context, iface, method string, args ...interface{}) ([]interface{}, error) {
			return []interface{}{true}, nil
		},
	}}

	_, err := c.GeneratePin(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "unexpected reply shape")
}

func testSimpleConfiguration_StartPin(t *testing.T) {
	t.Parallel()

	var called bool
	c := &SimpleConfiguration{call: &fakeCaller{
		callFn: func(ctx context.Context, iface, method string, args ...interface{}) ([]interface{}, error) {
			called = true
			require.Equal(t, IwdSimpleConfigurationIface, iface)
			require.Equal(t, "StartPin", method)
			require.Equal(t, []interface{}{"01234565"}, args)
			return nil, nil
		},
	}}

	require.NoError(t, c.StartPin(context.Background(), "01234565"))
	require.True(t, called)
}

func testSimpleConfiguration_StartPin_Err(t *testing.T) {
	t.Parallel()

	c := &SimpleConfiguration{call: &fakeCaller{
		callFn: func(ctx context.Context, iface, method string, args ...interface{}) ([]interface{}, error) {
			return nil, fmt.Errorf("dbus failure")
		},
	}}

	err := c.StartPin(context.Background(), "01234565")
	require.Error(t, err)
	require.Contains(t, err.Error(), "dbus failure")
}

func testSimpleConfiguration_StartPin_InvalidFormatMatchable(t *testing.T) {
	t.Parallel()

	// iwd rejects a malformed PIN with net.connman.iwd.InvalidFormat, which must
	// surface as a matchable sentinel.
	c := &SimpleConfiguration{call: &fakeCaller{
		callFn: func(ctx context.Context, iface, method string, args ...interface{}) ([]interface{}, error) {
			return nil, dbus.Error{
				Name: IwdErrorInvalidFormat,
				Body: []interface{}{"Invalid format"},
			}
		},
	}}

	err := c.StartPin(context.Background(), "bad")
	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidFormat, "expected ErrInvalidFormat, got %v", err)
	require.ErrorIs(t, err, ErrDBusMethod, "should still classify as a method error")
}

func testSimpleConfiguration_Cancel(t *testing.T) {
	t.Parallel()

	var called bool
	c := &SimpleConfiguration{call: &fakeCaller{
		callFn: func(ctx context.Context, iface, method string, args ...interface{}) ([]interface{}, error) {
			called = true
			require.Equal(t, IwdSimpleConfigurationIface, iface)
			require.Equal(t, "Cancel", method)
			require.Empty(t, args)
			return nil, nil
		},
	}}

	require.NoError(t, c.Cancel(context.Background()))
	require.True(t, called)
}

func testSimpleConfiguration_Cancel_Err(t *testing.T) {
	t.Parallel()

	c := &SimpleConfiguration{call: &fakeCaller{
		callFn: func(ctx context.Context, iface, method string, args ...interface{}) ([]interface{}, error) {
			return nil, fmt.Errorf("dbus failure")
		},
	}}

	err := c.Cancel(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "dbus failure")
}

func testSimpleConfiguration_NoIntro(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	for _, tc := range []struct {
		name string
		call func(*SimpleConfiguration) error
	}{
		{"PushButton", func(c *SimpleConfiguration) error { return c.PushButton(ctx) }},
		{"GeneratePin", func(c *SimpleConfiguration) error { _, err := c.GeneratePin(ctx); return err }},
		{"StartPin", func(c *SimpleConfiguration) error { return c.StartPin(ctx, "01234565") }},
		{"Cancel", func(c *SimpleConfiguration) error { return c.Cancel(ctx) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.call(&SimpleConfiguration{call: nil})
			require.Error(t, err)
			require.Contains(t, err.Error(), "simple configuration is not initialized")
		})
	}
}
