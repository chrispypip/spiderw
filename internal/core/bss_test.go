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

func (f *fakeIwdbusBSS) GetAddress(context.Context) (string, error) {
	return f.address, f.err
}

func (f *fakeIwdbusBSS) GetProperties(context.Context) (*iwdbus.BasicServiceSetProperties, error) {
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

	t.Run("NilReceiver", func(t *testing.T) {
		t.Parallel()

		var b *BasicServiceSet

		_, err := b.Address(context.Background())
		require.Error(t, err)
		require.ErrorIs(t, err, ErrBasicServiceSetNotInitialized)
	})

	t.Run("NewBasicServiceSet_NilRaw", func(t *testing.T) {
		t.Parallel()

		require.Nil(t, NewBasicServiceSet(nil))
	})
}
