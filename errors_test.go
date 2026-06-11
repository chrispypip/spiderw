//go:build unit

package spiderw

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/chrispypip/spiderw/internal/core"
)

// TestErrors_NoDuplicateFrame guards against the public Error restating the
// frame of a wrapped core.Error (label, Op, Details), which previously rendered
// every layer twice. The core.Error must still be discoverable via errors.As.
func TestErrors_NoDuplicateFrame(t *testing.T) {
	inner := errors.New("dbus property error: iface=net.connman.iwd.Adapter, property=Model: boom")
	ce := &core.Error{
		Kind:     core.KindUnavailable,
		Resource: core.ResourceAdapter,
		Op:       "Adapter.Model",
		Details:  "failed querying iwd Adapter model",
		Err:      inner,
	}

	pub := wrapPublicError("Adapter.Model", ce)
	msg := pub.Error()

	require.Equal(t, 1, strings.Count(msg, "adapter unavailable"), "duplicated label: %s", msg)
	require.Equal(t, 1, strings.Count(msg, "Op=Adapter.Model"), "duplicated Op: %s", msg)
	require.Equal(t, 1, strings.Count(msg, "failed querying iwd Adapter model"), "duplicated Details: %s", msg)
	require.Contains(t, msg, "boom")

	// Documented contract: underlying core and raw errors stay discoverable.
	var got *core.Error
	require.ErrorAs(t, pub, &got)
	require.ErrorIs(t, pub, inner)
}

func TestErrors_Public(t *testing.T) {
	t.Run("Sentinels", func(t *testing.T) {
		sentinels := []error{
			ErrUnavailable,
			ErrInvalidArgument,
			ErrInvalidState,
			ErrInternal,
		}

		for _, s := range sentinels {
			t.Run(s.Error(), func(t *testing.T) {
				require.ErrorIs(t, s, s, "sentinel must compare to itself")
			})
		}
	})

	t.Run("SentinelForKind", func(t *testing.T) {
		tests := []struct {
			kind     Kind
			expected error
		}{
			{KindUnavailable, ErrUnavailable},
			{KindInvalidArgument, ErrInvalidArgument},
			{KindInvalidState, ErrInvalidState},
			{KindInternal, ErrInternal},
		}

		for _, tc := range tests {
			t.Run(string(tc.kind), func(t *testing.T) {
				require.Equal(t, tc.expected, sentinelForKind(tc.kind))
			})
		}
	})

	t.Run("ErrorType", func(t *testing.T) {
		t.Run("ErrorFormatting", func(t *testing.T) {
			base := errors.New("boom")
			err := &Error{Kind: KindUnavailable, Resource: ResourceAdapter, Op: "AdapterInit", Details: "extra", Err: base}

			msg := err.Error()
			require.Contains(t, msg, "adapter unavailable")
			require.Contains(t, msg, "Op=AdapterInit")
			require.Contains(t, msg, "boom")
			require.Contains(t, msg, "extra")
		})

		t.Run("UnwrapChainContainsAPIAndSentinelAndUnderlying", func(t *testing.T) {
			under := errors.New("xyz")
			err := &Error{Kind: KindUnavailable, Resource: ResourceNetwork, Op: "OpX", Err: under}

			require.ErrorIs(t, err, ErrSpiderw)
			require.ErrorIs(t, err, ErrUnavailable)
			require.ErrorIs(t, err, under)
		})
	})

	t.Run("wrapPublicError", func(t *testing.T) {
		t.Run("CoreMapping", func(t *testing.T) {
			tests := []struct {
				name         string
				coreKind     core.Kind
				coreResource core.Resource
				public       error
				resource     Resource
			}{
				{"daemon unavailable", core.KindUnavailable, core.ResourceDaemon, ErrUnavailable, ResourceDaemon},
				{"adapter unavailable", core.KindUnavailable, core.ResourceAdapter, ErrUnavailable, ResourceAdapter},
				{"network unavailable", core.KindUnavailable, core.ResourceNetwork, ErrUnavailable, ResourceNetwork},
				{"invalid state", core.KindInvalidState, core.ResourceAdapter, ErrInvalidState, ResourceAdapter},
			}

			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					ce := &core.Error{Kind: tc.coreKind, Resource: tc.coreResource, Op: "dummy", Err: errors.New("boom")}
					err := wrapPublicError("OpX", ce)
					require.Error(t, err)
					require.ErrorIs(t, err, tc.public)

					var pe *Error
					require.ErrorAs(t, err, &pe)
					require.Equal(t, tc.resource, pe.Resource)
				})
			}
		})

		t.Run("UnknownErrorBecomesInternal", func(t *testing.T) {
			base := errors.New("weird")
			err := wrapPublicError("OpX", base)
			require.Error(t, err)
			require.ErrorIs(t, err, ErrInternal)
			require.ErrorIs(t, err, base)
		})

		t.Run("NilUnderlyingReturnsNil", func(t *testing.T) {
			require.NoError(t, wrapPublicError("OpX", nil))
		})

		t.Run("MessageStability", func(t *testing.T) {
			base := errors.New("x")
			err := wrapPublicError("OpX", base)
			require.Error(t, err)

			m1 := err.Error()
			m2 := err.Error()
			require.Equal(t, m1, m2)
		})
	})
}
