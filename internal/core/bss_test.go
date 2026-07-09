//go:build unit

package core

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/chrispypip/spiderw/internal/iwdbus"
)

type fakeIwdbusBSS struct {
	address string
	err     error
}

func (f *fakeIwdbusBSS) GetAddress(ctx context.Context) (string, error) {
	return f.address, f.err
}

func (f *fakeIwdbusBSS) GetProperties(ctx context.Context) (*iwdbus.BasicServiceSetProperties, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &iwdbus.BasicServiceSetProperties{Address: f.address}, nil
}

func TestBasicServiceSet_Core(t *testing.T) {
	t.Parallel()

	t.Run("Address", func(t *testing.T) {
		t.Parallel()

		b := NewBasicServiceSet(&fakeIwdbusBSS{address: "  11:22:33:44:55:66  "})
		require.NotNil(t, b)

		addr, err := b.Address(context.Background())
		require.NoError(t, err)
		require.Equal(t, "11:22:33:44:55:66", addr, "Address should be trimmed")
	})

	t.Run("Address_Empty", func(t *testing.T) {
		t.Parallel()

		b := NewBasicServiceSet(&fakeIwdbusBSS{address: "   "})

		_, err := b.Address(context.Background())
		require.Error(t, err)
		require.Contains(t, err.Error(), "empty Address")
	})

	t.Run("Address_BackendError", func(t *testing.T) {
		t.Parallel()

		b := NewBasicServiceSet(&fakeIwdbusBSS{err: iwdbus.WrapProperty(iwdbus.IwdBasicServiceSetIface, "Address", errors.New("dbus failure"))})

		_, err := b.Address(context.Background())
		require.Error(t, err)
		require.Contains(t, err.Error(), "dbus failure")
	})

	t.Run("Properties", func(t *testing.T) {
		t.Parallel()

		b := NewBasicServiceSet(&fakeIwdbusBSS{address: "11:22:33:44:55:66"})

		props, err := b.Properties(context.Background())
		require.NoError(t, err)
		require.Equal(t, "11:22:33:44:55:66", props.Address)
	})

	t.Run("Properties_Empty", func(t *testing.T) {
		t.Parallel()

		b := NewBasicServiceSet(&fakeIwdbusBSS{address: ""})

		_, err := b.Properties(context.Background())
		require.Error(t, err)
		require.Contains(t, err.Error(), "empty Address")
	})

	t.Run("Properties_BackendError", func(t *testing.T) {
		t.Parallel()

		b := NewBasicServiceSet(&fakeIwdbusBSS{err: iwdbus.WrapProperty(iwdbus.IwdBasicServiceSetIface, "Address", errors.New("dbus failure"))})

		_, err := b.Properties(context.Background())
		require.Error(t, err)
		var ce *Error
		require.ErrorAs(t, err, &ce)
		require.Equal(t, ResourceBasicServiceSet, ce.Resource)
		require.Contains(t, err.Error(), "dbus failure")
	})

	t.Run("NilReceiver", func(t *testing.T) {
		t.Parallel()

		var b *BasicServiceSet
		for _, tc := range []struct {
			name string
			call func() error
		}{
			{"Address", func() error { _, err := b.Address(context.Background()); return err }},
			{"Properties", func() error { _, err := b.Properties(context.Background()); return err }},
		} {
			t.Run(tc.name, func(t *testing.T) {
				err := tc.call()
				require.ErrorIs(t, err, ErrBasicServiceSetNotInitialized)
				require.ErrorIs(t, err, ErrCore)
			})
		}
	})

	t.Run("NewBasicServiceSet_NilRaw", func(t *testing.T) {
		t.Parallel()

		require.Nil(t, NewBasicServiceSet(nil))
	})
}
