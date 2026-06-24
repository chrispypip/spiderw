//go:build unit

package iwdbus

import (
	"context"
	"testing"

	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/require"
)

// TestIwdErrorMapping verifies that each recognized iwd D-Bus error name from a
// method call (here Network.Connect) maps to its matchable sentinel while
// preserving the original D-Bus error in the chain for diagnostics.
func TestIwdErrorMapping(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		sentinel error
	}{
		{name: IwdErrorNoAgent, sentinel: ErrNoAgent},
		{name: IwdErrorAborted, sentinel: ErrAborted},
		{name: IwdErrorBusy, sentinel: ErrBusy},
		{name: IwdErrorFailed, sentinel: ErrFailed},
		{name: IwdErrorNotSupported, sentinel: ErrNotSupported},
		{name: IwdErrorTimeout, sentinel: ErrTimeout},
		{name: IwdErrorInProgress, sentinel: ErrInProgress},
		{name: IwdErrorNotConfigured, sentinel: ErrNotConfigured},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dbusErr := dbus.Error{Name: tc.name, Body: []interface{}{"detail text"}}
			n := &Network{call: &fakeCaller{callFn: func(_ context.Context, _, _ string, _ ...interface{}) ([]interface{}, error) {
				return nil, dbusErr
			}}}

			err := n.Connect(context.Background())
			require.Error(t, err)
			// Maps to its sentinel...
			require.ErrorIs(t, err, tc.sentinel)
			// ...stays classified as a method error...
			require.ErrorIs(t, err, ErrDBusMethod)
			// ...and preserves the original D-Bus error for diagnostics.
			var de dbus.Error
			require.ErrorAs(t, err, &de)
			require.Equal(t, tc.name, de.Name)
		})
	}
}

func TestIwdErrorSentinel_Unrecognized(t *testing.T) {
	t.Parallel()

	require.Nil(t, iwdErrorSentinel(dbus.Error{Name: "net.connman.iwd.Unheard"}))
	require.Nil(t, iwdErrorSentinel(context.Canceled))
}
