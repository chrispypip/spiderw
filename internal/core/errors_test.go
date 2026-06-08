//go:build unit

package core

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/chrispypip/spiderw/internal/iwdbus"
)

// fakeErr simulates non-core errors.
type fakeErr struct{ msg string }

func (e fakeErr) Error() string { return e.msg }

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
						Err:  fakeErr{"x"},
					},
					wantIs: []error{ErrCore, fakeErr{"x"}},
				},
				{
					name: "message contains kind/op/details/underlying",
					err: &Error{
						Kind:     KindUnavailable,
						Resource: ResourceAdapter,
						Op:       "InitAdapter",
						Details:  "failed to talk to adapter",
						Err:      fakeErr{"boom"},
					},
					wantSubs: []string{"adapter unavailable", "Op=InitAdapter", "failed to talk to adapter", "boom"},
				},
			}

			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					require.NotNil(t, tc.err)

					for _, w := range tc.wantIs {
						require.True(t, errors.Is(tc.err, w), "expected errors.Is(...)")
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
			ce := &Error{Kind: KindUnavailable, Resource: ResourceAdapter, Op: "OpX", Err: fakeErr{"boom"}}

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
					name:     "network unavailable excludes property",
					fn:       WrapNetworkUnavailable,
					op:       "Scan",
					details:  "network check",
					inputErr: iwdbus.ErrDBusProperty,
					wantKind: KindOperationFailed,
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

					require.True(t, errors.Is(err, ErrCore))
					require.True(t, errors.Is(err, tc.inputErr))
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
					base := fakeErr{"boom"}
					err := tc.fn("OpX", "details", base)
					require.Error(t, err)

					var ce *Error
					require.ErrorAs(t, err, &ce)
					require.Equal(t, tc.wantKind, ce.Kind)
					require.Equal(t, tc.wantRes, ce.Resource)

					require.True(t, errors.Is(err, ErrCore))
					require.True(t, errors.Is(err, base))
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
			require.True(t, errors.Is(e2, ErrCore))
			require.True(t, errors.Is(e2, iwdbus.ErrDBusConnection))
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
			require.True(t, strings.Contains(msg, "adapter unavailable"))
			require.True(t, strings.Contains(msg, "Op=Op"))
			require.True(t, strings.Contains(msg, "details"))
			require.True(t, strings.Contains(msg, "dbus property error"))
		})

		t.Run("WithNestedIwdBus", func(t *testing.T) {
			low := fakeErr{"boom"}
			inner := &iwdbus.Error{
				Kind:    iwdbus.ErrDBusConnection,
				Context: "iface=X method=Y",
				Err:     low,
			}

			err := WrapAdapterUnavailable("Op", "details", inner)
			require.True(t, errors.Is(err, iwdbus.ErrDBusConnection))
			require.True(t, errors.Is(err, low))
		})
	})
}
