//go:build unit

package core

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/chrispypip/spiderw/internal/iwdbus"
)

func TestSimpleConfiguration_Core(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("New", func(t *testing.T) {
		t.Parallel()
		require.Nil(t, NewSimpleConfiguration(nil))
		require.NotNil(t, NewSimpleConfiguration(&fakeSimpleConfigRaw{}))
	})

	t.Run("PushButton", func(t *testing.T) {
		t.Parallel()

		t.Run("Success", func(t *testing.T) {
			t.Parallel()
			f := &fakeSimpleConfigRaw{}
			require.NoError(t, NewSimpleConfiguration(f).PushButton(ctx))
			require.Equal(t, []string{"PushButton"}, f.callList())
		})

		t.Run("OverlapSentinelPreserved", func(t *testing.T) {
			t.Parallel()
			// Mimic iwdbus wrapping: a D-Bus method failure that carries the WSC
			// session-overlap sentinel. Core must classify it Unavailable while
			// keeping the sentinel matchable.
			f := &fakeSimpleConfigRaw{pushErr: fmt.Errorf("%w: %w", iwdbus.ErrDBusMethod, iwdbus.ErrWSCSessionOverlap)}
			err := NewSimpleConfiguration(f).PushButton(ctx)
			require.Error(t, err)
			require.ErrorIs(t, err, iwdbus.ErrWSCSessionOverlap)

			var ce *Error
			require.ErrorAs(t, err, &ce)
			require.Equal(t, KindUnavailable, ce.Kind)
			require.Equal(t, ResourceSimpleConfiguration, ce.Resource)
		})
	})

	t.Run("GeneratePin", func(t *testing.T) {
		t.Parallel()

		t.Run("Success", func(t *testing.T) {
			t.Parallel()
			f := &fakeSimpleConfigRaw{genPin: "12345670"}
			pin, err := NewSimpleConfiguration(f).GeneratePin(ctx)
			require.NoError(t, err)
			require.Equal(t, "12345670", pin)
		})

		t.Run("Error", func(t *testing.T) {
			t.Parallel()
			f := &fakeSimpleConfigRaw{genErr: iwdbus.ErrDBusMethod}
			_, err := NewSimpleConfiguration(f).GeneratePin(ctx)
			require.Error(t, err)

			var ce *Error
			require.ErrorAs(t, err, &ce)
			require.Equal(t, ResourceSimpleConfiguration, ce.Resource)
		})
	})

	t.Run("StartPin", func(t *testing.T) {
		t.Parallel()

		t.Run("NormalizesAndForwards", func(t *testing.T) {
			t.Parallel()
			for _, tc := range []struct {
				name string
				in   string
				want string
			}{
				{"plain8", "12345670", "12345670"},
				{"hyphenated", "1234-5670", "12345670"},
				{"spaced", "1234 5670", "12345670"},
				{"plain4", "1234", "1234"},
			} {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					f := &fakeSimpleConfigRaw{}
					require.NoError(t, NewSimpleConfiguration(f).StartPin(ctx, tc.in))
					require.Equal(t, tc.want, f.pinStarted())
				})
			}
		})

		t.Run("InvalidPinFailsLocally", func(t *testing.T) {
			t.Parallel()
			for _, tc := range []struct {
				name string
				in   string
			}{
				{"empty", ""},
				{"separators only", " - "},
				{"non-digit", "12ab5670"},
				{"too short", "123"},
				{"odd length", "1234567"},
			} {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					f := &fakeSimpleConfigRaw{}
					err := NewSimpleConfiguration(f).StartPin(ctx, tc.in)
					require.Error(t, err)

					var ce *Error
					require.ErrorAs(t, err, &ce)
					require.Equal(t, KindInvalidArgument, ce.Kind)
					require.Equal(t, ResourceSimpleConfiguration, ce.Resource)
					require.Empty(t, f.callList(), "iwd must not be called for an invalid PIN")
				})
			}
		})

		t.Run("IwdErrorWrapped", func(t *testing.T) {
			t.Parallel()
			f := &fakeSimpleConfigRaw{startErr: fmt.Errorf("%w: %w", iwdbus.ErrDBusMethod, iwdbus.ErrInvalidFormat)}
			err := NewSimpleConfiguration(f).StartPin(ctx, "12345670")
			require.Error(t, err)
			require.ErrorIs(t, err, iwdbus.ErrInvalidFormat)
		})
	})

	t.Run("Cancel", func(t *testing.T) {
		t.Parallel()

		t.Run("Success", func(t *testing.T) {
			t.Parallel()
			f := &fakeSimpleConfigRaw{}
			require.NoError(t, NewSimpleConfiguration(f).Cancel(ctx))
		})

		t.Run("Error", func(t *testing.T) {
			t.Parallel()
			f := &fakeSimpleConfigRaw{cancelErr: iwdbus.ErrDBusMethod}
			require.Error(t, NewSimpleConfiguration(f).Cancel(ctx))
		})
	})

	t.Run("NotInitialized", func(t *testing.T) {
		t.Parallel()
		for _, tc := range []struct {
			name string
			call func(*SimpleConfiguration) error
		}{
			{"PushButton", func(c *SimpleConfiguration) error { return c.PushButton(ctx) }},
			{"GeneratePin", func(c *SimpleConfiguration) error { _, err := c.GeneratePin(ctx); return err }},
			{"StartPin", func(c *SimpleConfiguration) error { return c.StartPin(ctx, "12345670") }},
			{"Cancel", func(c *SimpleConfiguration) error { return c.Cancel(ctx) }},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				var c *SimpleConfiguration
				err := tc.call(c)
				require.Error(t, err)
				require.ErrorIs(t, err, ErrSimpleConfigurationNotInitialized)

				var ce *Error
				require.ErrorAs(t, err, &ce)
				require.Equal(t, KindInvalidState, ce.Kind)
				require.Equal(t, ResourceSimpleConfiguration, ce.Resource)
			})
		}
	})
}
