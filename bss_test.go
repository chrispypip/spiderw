//go:build unit

package spiderw

import (
	"context"
	"errors"
	"testing"

	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/require"

	"github.com/chrispypip/spiderw/internal/connect"
	"github.com/chrispypip/spiderw/internal/core"
)

type fakeCoreBSS struct {
	address string
	err     error
}

func (f *fakeCoreBSS) Address(ctx context.Context) (string, error) {
	return f.address, f.err
}

func (f *fakeCoreBSS) Properties(ctx context.Context) (*core.BasicServiceSetProperties, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &core.BasicServiceSetProperties{Address: f.address}, nil
}

func TestBasicServiceSet(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("AddressAndProperties", func(t *testing.T) {
		t.Parallel()

		b := newBasicServiceSet(&fakeCoreBSS{address: "11:22:33:44:55:66"}, "/net/connman/iwd/phy0/wlan0/bss")
		require.NotNil(t, b)
		require.Equal(t, "/net/connman/iwd/phy0/wlan0/bss", b.Path())

		addr, err := b.Address(ctx)
		require.NoError(t, err)
		require.Equal(t, "11:22:33:44:55:66", addr)

		props, err := b.Properties(ctx)
		require.NoError(t, err)
		require.Equal(t, "11:22:33:44:55:66", props.Address)
	})

	t.Run("NilReceiverMapsToInternal", func(t *testing.T) {
		t.Parallel()

		var b *BasicServiceSet
		require.Equal(t, "", b.Path())

		_, err := b.Address(ctx)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrInternal)
	})

	t.Run("NewNilCore", func(t *testing.T) {
		t.Parallel()
		require.Nil(t, newBasicServiceSet(nil, "/path"))
	})
}

func TestClientAllBasicServiceSets(t *testing.T) {
	ctx := context.Background()

	newClient := func(
		refs []core.BasicServiceSetRef,
		daemonErr error,
		factory func(ctx context.Context, path string) (core.BasicServiceSetIface, error),
	) *Client {
		fakeDaemon := &fakeCoreDaemon{}
		fakeDaemon.setBasicServiceSets(refs)
		if daemonErr != nil {
			fakeDaemon.setErr(daemonErr)
		}
		if factory == nil {
			factory = func(ctx context.Context, path string) (core.BasicServiceSetIface, error) {
				return &fakeCoreBSS{address: path}, nil
			}
		}
		wire := &connect.Wiring{
			Conn:                   &dbus.Conn{},
			Daemon:                 fakeDaemon,
			Cleanup:                func() error { return nil },
			BasicServiceSetFactory: factory,
		}
		return &Client{
			daemon:  newDaemon(fakeDaemon),
			wire:    wire,
			cleanup: wire.Cleanup,
		}
	}

	t.Run("Success", func(t *testing.T) {
		refs := []core.BasicServiceSetRef{
			{Path: "/net/connman/iwd/phy0/wlan0/bss0", Address: "11:22:33:44:55:66"},
			{Path: "/net/connman/iwd/phy0/wlan0/bss1", Address: "aa:bb:cc:dd:ee:ff"},
		}
		c := newClient(refs, nil, nil)

		bsses, err := c.AllBasicServiceSets(ctx)
		require.NoError(t, err)
		require.Len(t, bsses, len(refs))

		// Order is preserved and each handle is live: the fake reports the path it
		// was constructed from as its address.
		for i, b := range bsses {
			require.NotNil(t, b)
			require.Equal(t, refs[i].Path, b.Path())
			addr, err := b.Address(ctx)
			require.NoError(t, err)
			require.Equal(t, refs[i].Path, addr)
		}
	})

	t.Run("Empty", func(t *testing.T) {
		c := newClient(nil, nil, nil)

		bsses, err := c.AllBasicServiceSets(ctx)
		require.NoError(t, err)
		require.NotNil(t, bsses)
		require.Empty(t, bsses)
	})

	t.Run("EnumerationErrorMapsToPublicError", func(t *testing.T) {
		base := errors.New("enumeration failed")
		c := newClient(nil, base, nil)

		bsses, err := c.AllBasicServiceSets(ctx)
		require.Nil(t, bsses)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrInternal)
		require.ErrorIs(t, err, base)
	})

	t.Run("Closed", func(t *testing.T) {
		refs := []core.BasicServiceSetRef{{Path: "/net/connman/iwd/phy0/wlan0/bss0", Address: "11:22:33:44:55:66"}}
		c := newClient(refs, nil, nil)
		require.NoError(t, c.Close())

		bsses, err := c.AllBasicServiceSets(ctx)
		require.Nil(t, bsses)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrInvalidState)
	})

	t.Run("NilReceiver", func(t *testing.T) {
		var c *Client
		bsses, err := c.AllBasicServiceSets(ctx)
		require.Nil(t, bsses)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrInternal)
	})
}
