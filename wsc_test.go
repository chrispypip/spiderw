//go:build unit

package spiderw

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/chrispypip/spiderw/internal/core"
)

func TestSimpleConfiguration_Public(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	newWSCStation := func(construct func(ctx context.Context, path string) (core.SimpleConfigurationIface, error)) *Station {
		return newStation(&fakeCoreStation{}, "/net/connman/iwd/0/3", "wlan0").withSimpleConfiguration(construct)
	}

	t.Run("SimpleConfiguration", func(t *testing.T) {
		t.Parallel()

		t.Run("Success", func(t *testing.T) {
			t.Parallel()
			var gotPath string
			construct := func(ctx context.Context, path string) (core.SimpleConfigurationIface, error) {
				gotPath = path
				return &fakeCoreSimpleConfig{}, nil
			}
			h, err := newWSCStation(construct).SimpleConfiguration(ctx)
			require.NoError(t, err)
			require.NotNil(t, h)
			require.Equal(t, "/net/connman/iwd/0/3", gotPath)
		})

		t.Run("ConstructError", func(t *testing.T) {
			t.Parallel()
			wantErr := errors.New("wsc unavailable")
			construct := func(ctx context.Context, path string) (core.SimpleConfigurationIface, error) {
				return nil, wantErr
			}
			h, err := newWSCStation(construct).SimpleConfiguration(ctx)
			require.Nil(t, h)
			require.ErrorIs(t, err, wantErr)
		})

		t.Run("NilCoreIsInternal", func(t *testing.T) {
			t.Parallel()
			construct := func(ctx context.Context, path string) (core.SimpleConfigurationIface, error) {
				return nil, nil // an errorless nil must not yield a usable handle
			}
			h, err := newWSCStation(construct).SimpleConfiguration(ctx)
			require.Nil(t, h)
			require.ErrorIs(t, err, ErrInternal)
		})

		t.Run("UnsupportedStation", func(t *testing.T) {
			t.Parallel()
			// A bare station (no injected hook) cannot use WSC.
			s := newStation(&fakeCoreStation{}, "/net/connman/iwd/0/3", "wlan0")
			h, err := s.SimpleConfiguration(ctx)
			require.Nil(t, h)
			require.ErrorIs(t, err, ErrInternal)
		})

		t.Run("NilStation", func(t *testing.T) {
			t.Parallel()
			var s *Station
			h, err := s.SimpleConfiguration(ctx)
			require.Nil(t, h)
			require.ErrorIs(t, err, ErrInternal)
		})
	})

	t.Run("Handle", func(t *testing.T) {
		t.Parallel()

		newHandle := func(t *testing.T, fake core.SimpleConfigurationIface) *SimpleConfiguration {
			t.Helper()
			construct := func(ctx context.Context, path string) (core.SimpleConfigurationIface, error) {
				return fake, nil
			}
			h, err := newWSCStation(construct).SimpleConfiguration(ctx)
			require.NoError(t, err)
			return h
		}

		t.Run("PushButtonDelegates", func(t *testing.T) {
			t.Parallel()
			f := &fakeCoreSimpleConfig{}
			require.NoError(t, newHandle(t, f).PushButton(ctx))
			require.Equal(t, []string{"PushButton"}, f.callList())
		})

		t.Run("PushButtonError", func(t *testing.T) {
			t.Parallel()
			wantErr := errors.New("overlap")
			f := &fakeCoreSimpleConfig{pushErr: wantErr}
			require.ErrorIs(t, newHandle(t, f).PushButton(ctx), wantErr)
		})

		t.Run("GeneratePinReturnsPin", func(t *testing.T) {
			t.Parallel()
			f := &fakeCoreSimpleConfig{genPin: "12345670"}
			pin, err := newHandle(t, f).GeneratePin(ctx)
			require.NoError(t, err)
			require.Equal(t, "12345670", pin)
		})

		t.Run("GeneratePinError", func(t *testing.T) {
			t.Parallel()
			wantErr := errors.New("no pin")
			f := &fakeCoreSimpleConfig{genErr: wantErr}
			_, err := newHandle(t, f).GeneratePin(ctx)
			require.ErrorIs(t, err, wantErr)
		})

		t.Run("StartPinDelegates", func(t *testing.T) {
			t.Parallel()
			f := &fakeCoreSimpleConfig{}
			require.NoError(t, newHandle(t, f).StartPin(ctx, "1234-5670"))
			require.Equal(t, []string{"StartPin"}, f.callList())
			// The public layer forwards the PIN verbatim; core does the normalizing.
			require.Equal(t, "1234-5670", f.pinStarted())
		})

		t.Run("StartPinError", func(t *testing.T) {
			t.Parallel()
			wantErr := errors.New("no credentials")
			f := &fakeCoreSimpleConfig{startErr: wantErr}
			require.ErrorIs(t, newHandle(t, f).StartPin(ctx, "12345670"), wantErr)
		})

		t.Run("CancelDelegates", func(t *testing.T) {
			t.Parallel()
			f := &fakeCoreSimpleConfig{}
			require.NoError(t, newHandle(t, f).Cancel(ctx))
			require.Equal(t, []string{"Cancel"}, f.callList())
		})

		t.Run("CancelError", func(t *testing.T) {
			t.Parallel()
			wantErr := errors.New("not connected")
			f := &fakeCoreSimpleConfig{cancelErr: wantErr}
			require.ErrorIs(t, newHandle(t, f).Cancel(ctx), wantErr)
		})

		t.Run("NilHandleIsInternal", func(t *testing.T) {
			t.Parallel()
			var h *SimpleConfiguration
			require.ErrorIs(t, h.PushButton(ctx), ErrInternal)
			require.ErrorIs(t, h.StartPin(ctx, "12345670"), ErrInternal)
			require.ErrorIs(t, h.Cancel(ctx), ErrInternal)
			_, err := h.GeneratePin(ctx)
			require.ErrorIs(t, err, ErrInternal)
		})
	})
}
