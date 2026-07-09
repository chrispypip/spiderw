//go:build unit

package connect

import (
	"context"
	"testing"

	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/require"
)

// TestWiring_Resolver pins how a Wiring selects its friendly-reference resolver.
// The conn-backed resolver's Resolve (which dials the connection) is exercised by
// the integration suite; here we lock the nil-safety and override wiring that
// keeps fake-backed tests from touching a placeholder connection -- the exact
// contract that, when missing, panicked the race suite.
func TestWiring_Resolver(t *testing.T) {
	t.Parallel()

	t.Run("NilWiring", func(t *testing.T) {
		require.Nil(t, (*Wiring)(nil).Resolver())
	})

	t.Run("NilConnYieldsNilResolver", func(t *testing.T) {
		// No connection => nil resolver, so callers stay nil-safe.
		require.Nil(t, (&Wiring{}).Resolver())
	})

	t.Run("OverrideWins", func(t *testing.T) {
		// A test override is returned verbatim, even alongside a connection,
		// bypassing the conn-backed resolver entirely.
		w := &Wiring{Conn: &dbus.Conn{}, ResolverOverride: NoResolver{}}
		require.Equal(t, NoResolver{}, w.Resolver())
	})

	t.Run("ConnBacked", func(t *testing.T) {
		// A connection (and no override) yields a non-nil conn-backed resolver. We
		// deliberately do not call Resolve: it would dial the placeholder Conn.
		r := (&Wiring{Conn: &dbus.Conn{}}).Resolver()
		require.NotNil(t, r)
		require.IsType(t, &wiringResolver{}, r)
	})
}

// TestNoResolver_Resolve verifies NoResolver resolves nothing: a nil Tree and no
// error, which leaves every bundle reference path-only.
func TestNoResolver_Resolve(t *testing.T) {
	t.Parallel()

	tree, err := NoResolver{}.Resolve(context.Background())
	require.NoError(t, err)
	require.Nil(t, tree)
}
