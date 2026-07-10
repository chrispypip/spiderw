//go:build unit

package iwdbus

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type fakeError struct{ msg string }

func (f fakeError) Error() string { return f.msg }

func TestErrors_Iwdbus(t *testing.T) {
	t.Parallel()

	t.Run("Wrap", func(t *testing.T) {
		t.Parallel()
		t.Run("NilError_ReturnsNil", func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name string
				fn   func(error) error
			}{
				{name: "connection", fn: func(err error) error { return WrapConnection("Connect", err) }},
				{name: "method", fn: func(err error) error { return WrapMethod("net.connman.iwd.Station", "Scan", err) }},
				{name: "property", fn: func(err error) error { return WrapProperty("net.connman.iwd.Adapter", "Powered", err) }},
				{name: "introspection", fn: func(err error) error { return WrapIntrospection("/net/connman/iwd/phy0", err) }},
				{name: "variant", fn: func(err error) error { return WrapVariant("SupportedModes", err) }},
			}

			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					require.NoError(t, tc.fn(nil))
				})
			}
		})

		t.Run("SetsSentinel", func(t *testing.T) {
			t.Parallel()

			baseConn := errors.New("socket closed")
			base := fakeError{"boom"}

			tests := []struct {
				name     string
				makeErr  func() error
				sentinel error
				base     error
			}{
				{
					name:     "connection",
					makeErr:  func() error { return WrapConnection("Connect", baseConn) },
					sentinel: ErrDBusConnection,
					base:     baseConn,
				},
				{
					name:     "method",
					makeErr:  func() error { return WrapMethod("net.connman.iwd.Station", "Scan", base) },
					sentinel: ErrDBusMethod,
					base:     base,
				},
				{
					name:     "property",
					makeErr:  func() error { return WrapProperty("net.connman.iwd.Adapter", "Powered", base) },
					sentinel: ErrDBusProperty,
					base:     base,
				},
				{
					name:     "introspection",
					makeErr:  func() error { return WrapIntrospection("/net/connman/iwd/phy0", base) },
					sentinel: ErrDBusIntrospection,
					base:     base,
				},
				{
					name:     "variant",
					makeErr:  func() error { return WrapVariant("SupportedModes", base) },
					sentinel: ErrDBusVariant,
					base:     base,
				},
			}

			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()

					err := tc.makeErr()
					require.Error(t, err)
					require.ErrorIs(t, err, tc.sentinel)
					require.ErrorIs(t, err, tc.base)
				})
			}
		})

		t.Run("GoldenMessages", func(t *testing.T) {
			t.Parallel()

			baseErr := fakeError{"boom"}

			tests := []struct {
				name         string
				err          error
				wantContains []string
				sentinel     error
			}{
				{
					name:         "connection",
					err:          WrapConnection("Connect", baseErr),
					wantContains: []string{"dbus connection error", "op=Connect", "boom"},
					sentinel:     ErrDBusConnection,
				},
				{
					name:         "method",
					err:          WrapMethod("net.connman.iwd.Station", "Scan", baseErr),
					wantContains: []string{"dbus method error", "iface=net.connman.iwd.Station, method=Scan", "boom"},
					sentinel:     ErrDBusMethod,
				},
				{
					name:         "property",
					err:          WrapProperty("net.connman.iwd.Adapter", "Powered", baseErr),
					wantContains: []string{"dbus property error", "iface=net.connman.iwd.Adapter, property=Powered", "boom"},
					sentinel:     ErrDBusProperty,
				},
				{
					name:         "introspection",
					err:          WrapIntrospection("/net/connman/iwd/phy0", baseErr),
					wantContains: []string{"dbus introspection error", "path=/net/connman/iwd/phy0", "boom"},
					sentinel:     ErrDBusIntrospection,
				},
				{
					name:         "variant",
					err:          WrapVariant("SupportedModes", baseErr),
					wantContains: []string{"dbus variant conversion error", "variant=SupportedModes", "boom"},
					sentinel:     ErrDBusVariant,
				},
			}

			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()

					require.Error(t, tc.err)
					require.ErrorIs(t, tc.err, tc.sentinel)

					msg := tc.err.Error()
					for _, sub := range tc.wantContains {
						require.Contains(t, msg, sub)
					}
				})
			}
		})

		t.Run("UnwrapContainsBase", func(t *testing.T) {
			t.Parallel()

			base := fakeError{"low-level dbus error"}
			errs := []error{
				WrapConnection("Connect", base),
				WrapMethod("net.connman.iwd.Adapter", "GetInfo", base),
				WrapProperty("net.connman.iwd.Adapter", "Powered", base),
				WrapIntrospection("/net/connman/iwd/phy0", base),
				WrapVariant("SupportedModes", base),
			}

			for _, err := range errs {
				require.Error(t, err)
				require.ErrorIs(t, err, base, "expected base error %q to be in error chain: %v", base, err)
			}
		})

		t.Run("ReturnsDBusErrorType", func(t *testing.T) {
			t.Parallel()

			err := WrapMethod("iface", "Method", fakeError{"x"})
			require.Error(t, err)

			var derr *Error
			require.ErrorAs(t, err, &derr, "expected Wrap* to return *dbusError")
		})

		t.Run("ContainsBothSentinelAndBase", func(t *testing.T) {
			t.Parallel()

			base := fakeError{"base"}
			err := WrapMethod("iface", "Method", base)
			require.ErrorIs(t, err, ErrDBusMethod)
			require.ErrorIs(t, err, base)
		})

		t.Run("ExactFormatContainsStablePrefix", func(t *testing.T) {
			t.Parallel()

			err := WrapVariant("X", fakeError{"boom"})
			got := err.Error()
			want := "dbus variant conversion error"
			require.Contains(t, got, want, "Error() must remain stable for public API guarantees")
		})

		t.Run("NotEqualByValue", func(t *testing.T) {
			t.Parallel()

			err := WrapMethod("iface", "Method", fakeError{"boom"})
			//nolint:errorlint // intentionally asserts the wrapped error is not == the sentinel; errors.Is is verified on the next line.
			require.NotEqual(t, err, ErrDBusMethod)
			require.ErrorIs(t, err, ErrDBusMethod)
		})

		t.Run("NestedWrapping_StillFindsSentinel", func(t *testing.T) {
			t.Parallel()

			base := fakeError{"boom"}
			e1 := WrapMethod("iface", "Method", base)
			e2 := WrapMethod("iface", "Method", e1)
			require.ErrorIs(t, e2, ErrDBusMethod)
			require.ErrorIs(t, e2, base)
		})

		t.Run("WrapIdempotence", func(t *testing.T) {
			t.Parallel()

			base := fakeError{"boom"}
			original := WrapMethod("iface", "Method", base)
			wrapped := WrapMethod("iface", "Method", original)
			require.ErrorIs(t, wrapped, ErrDBusMethod)
			require.ErrorIs(t, wrapped, base)
		})
	})
}
