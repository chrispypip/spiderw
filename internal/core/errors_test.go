//go:build unit

package core

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/chrispypip/spiderw/internal/iwdbus"
)

// fakeError simulates non-core errors.
type fakeError struct{ msg string }

func (e fakeError) Error() string { return e.msg }

// Tests in this file are organized using a subset subtest structure.
// This keeps related table-driven cases grouped under stable headings.

func TestErrors_Core(t *testing.T) {
	t.Run("Error", func(t *testing.T) {
		t.Run("UnwrapAndFormatting", func(t *testing.T) {
			tests := []struct {
				name     string
				err      *Error
				wantSubs []string
				wantIs   []error
			}{
				{
					name: "unwrap contains ErrCore and underlying",
					err: &Error{
						Kind: KindOperationFailed,
						Op:   "OpX",
						Err:  fakeError{"x"},
					},
					wantIs: []error{ErrCore, fakeError{"x"}},
				},
				{
					name: "message contains kind/op/details/underlying",
					err: &Error{
						Kind:     KindUnavailable,
						Resource: ResourceAdapter,
						Op:       "InitAdapter",
						Details:  "failed to talk to adapter",
						Err:      fakeError{"boom"},
					},
					wantSubs: []string{"adapter unavailable", "Op=InitAdapter", "failed to talk to adapter", "boom"},
				},
			}

			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					require.NotNil(t, tc.err)

					for _, w := range tc.wantIs {
						require.ErrorIs(t, tc.err, w, "expected errors.Is(...)")
					}
					for _, sub := range tc.wantSubs {
						require.Contains(t, tc.err.Error(), sub)
					}
				})
			}
		})

		t.Run("AsFindsErrorType", func(t *testing.T) {
			err := WrapAdapterUnavailable("Op", "d", iwdbus.ErrDBusMethod)
			require.Error(t, err)

			var ce *Error
			require.ErrorAs(t, err, &ce)
			require.Equal(t, KindUnavailable, ce.Kind)
			require.Equal(t, ResourceAdapter, ce.Resource)
		})

		t.Run("ErrorAsSelf", func(t *testing.T) {
			ce := &Error{Kind: KindUnavailable, Resource: ResourceAdapter, Op: "OpX", Err: fakeError{"boom"}}

			var out *Error
			require.ErrorAs(t, ce, &out)
			require.Equal(t, ce, out)
		})
	})

	t.Run("Wrap", func(t *testing.T) {
		t.Run("MappingFunctions", func(t *testing.T) {
			type wrapFn func(op, details string, err error) error

			tests := []struct {
				name     string
				fn       wrapFn
				op       string
				details  string
				inputErr error
				wantKind Kind
				wantRes  Resource
			}{
				{
					name:     "daemon unavailable maps dbus connection",
					fn:       WrapDaemonUnavailable,
					op:       "GetInfo",
					details:  "daemon check",
					inputErr: iwdbus.ErrDBusConnection,
					wantKind: KindUnavailable,
					wantRes:  ResourceDaemon,
				},
				{
					name:     "daemon unavailable does not map property",
					fn:       WrapDaemonUnavailable,
					op:       "GetInfo",
					details:  "daemon check",
					inputErr: iwdbus.ErrDBusProperty,
					wantKind: KindOperationFailed,
					wantRes:  ResourceDaemon,
				},
				{
					name:     "adapter unavailable maps dbus property",
					fn:       WrapAdapterUnavailable,
					op:       "InitAdapter",
					details:  "adapter check",
					inputErr: iwdbus.ErrDBusProperty,
					wantKind: KindUnavailable,
					wantRes:  ResourceAdapter,
				},
				{
					name:     "network unavailable maps dbus property",
					fn:       WrapNetworkUnavailable,
					op:       "Scan",
					details:  "network check",
					inputErr: iwdbus.ErrDBusProperty,
					wantKind: KindUnavailable,
					wantRes:  ResourceNetwork,
				},
				{
					name:     "non-dbus error defaults to operation failed",
					fn:       WrapAdapterUnavailable,
					op:       "InitAdapter",
					details:  "adapter check",
					inputErr: errors.New("x"),
					wantKind: KindOperationFailed,
					wantRes:  ResourceAdapter,
				},
			}

			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					err := tc.fn(tc.op, tc.details, tc.inputErr)
					require.Error(t, err)

					var ce *Error
					require.ErrorAs(t, err, &ce)
					require.Equal(t, tc.wantKind, ce.Kind)
					require.Equal(t, tc.wantRes, ce.Resource)

					require.ErrorIs(t, err, ErrCore)
					require.ErrorIs(t, err, tc.inputErr)
				})
			}
		})

		t.Run("SimpleWrappers", func(t *testing.T) {
			type wrapFn func(op, details string, err error) error

			tests := []struct {
				name     string
				fn       wrapFn
				wantKind Kind
				wantRes  Resource
			}{
				{name: "invalid state", fn: func(op, details string, err error) error {
					return WrapInvalidState(ResourceDaemon, op, details, err)
				}, wantKind: KindInvalidState, wantRes: ResourceDaemon},
				{name: "operation failed", fn: func(op, details string, err error) error {
					return WrapOperationFailed(ResourceAdapter, op, details, err)
				}, wantKind: KindOperationFailed, wantRes: ResourceAdapter},
			}

			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					base := fakeError{"boom"}
					err := tc.fn("OpX", "details", base)
					require.Error(t, err)

					var ce *Error
					require.ErrorAs(t, err, &ce)
					require.Equal(t, tc.wantKind, ce.Kind)
					require.Equal(t, tc.wantRes, ce.Resource)

					require.ErrorIs(t, err, ErrCore)
					require.ErrorIs(t, err, base)
					require.Contains(t, err.Error(), "Op=OpX")
				})
			}
		})

		t.Run("Idempotence", func(t *testing.T) {
			base := iwdbus.ErrDBusConnection

			e1 := WrapAdapterUnavailable("Op", "x", base)
			e2 := WrapAdapterUnavailable("Op", "x", e1)

			var ce *Error
			require.ErrorAs(t, e2, &ce)
			require.Equal(t, KindUnavailable, ce.Kind)
			require.Equal(t, ResourceAdapter, ce.Resource)
			require.ErrorIs(t, e2, ErrCore)
			require.ErrorIs(t, e2, iwdbus.ErrDBusConnection)
		})

		t.Run("NilErrorsReturnNil", func(t *testing.T) {
			tests := []struct {
				name string
				fn   func() error
			}{
				{name: "WrapDaemonUnavailable", fn: func() error { return WrapDaemonUnavailable("x", "y", nil) }},
				{name: "WrapAdapterUnavailable", fn: func() error { return WrapAdapterUnavailable("x", "y", nil) }},
				{name: "WrapNetworkUnavailable", fn: func() error { return WrapNetworkUnavailable("x", "y", nil) }},
				{name: "WrapInvalidState", fn: func() error { return WrapInvalidState(ResourceDaemon, "x", "y", nil) }},
				{name: "WrapOperationFailed", fn: func() error { return WrapOperationFailed(ResourceAdapter, "x", "y", nil) }},
			}

			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					require.NoError(t, tc.fn())
				})
			}
		})

		t.Run("MessageStability", func(t *testing.T) {
			base := iwdbus.ErrDBusProperty
			err := WrapAdapterUnavailable("Op", "details", base)

			msg := err.Error()
			require.Contains(t, msg, "adapter unavailable")
			require.Contains(t, msg, "Op=Op")
			require.Contains(t, msg, "details")
			require.Contains(t, msg, "dbus property error")
		})

		t.Run("MessageWithoutDetails", func(t *testing.T) {
			// With no Details, Error() omits the trailing parenthetical.
			err := &Error{Kind: KindUnavailable, Resource: ResourceAdapter, Op: "Op", Err: iwdbus.ErrDBusProperty}
			msg := err.Error()
			require.Contains(t, msg, "adapter unavailable: Op=Op")
			require.NotContains(t, msg, "(")
		})

		t.Run("LabelWithoutResource", func(t *testing.T) {
			// A ResourceUnknown error labels with the bare kind, no resource prefix.
			require.Equal(t, string(KindUnavailable), errorLabel(KindUnavailable, ResourceUnknown))

			err := &Error{Kind: KindOperationFailed, Resource: ResourceUnknown, Op: "Op", Err: iwdbus.ErrDBusProperty}
			require.Contains(t, err.Error(), string(KindOperationFailed)+": Op=Op")
		})

		t.Run("WrapAgentUnavailable", func(t *testing.T) {
			err := WrapAgentUnavailable("Op", "details", iwdbus.ErrDBusMethod)
			require.Error(t, err)
			var ce *Error
			require.ErrorAs(t, err, &ce)
			require.Equal(t, ResourceAgent, ce.Resource)
			require.ErrorIs(t, err, iwdbus.ErrDBusMethod)
		})

		t.Run("WithNestedIwdBus", func(t *testing.T) {
			low := fakeError{"boom"}
			inner := &iwdbus.Error{
				Kind:    iwdbus.ErrDBusConnection,
				Context: "iface=X method=Y",
				Err:     low,
			}

			err := WrapAdapterUnavailable("Op", "details", inner)
			require.ErrorIs(t, err, iwdbus.ErrDBusConnection)
			require.ErrorIs(t, err, low)
		})
	})
}
