package connect

import (
	"context"

	"github.com/godbus/dbus/v5"

	"github.com/chrispypip/spiderw/internal/iwdbus"
)

// Resolver fetches a snapshot of iwd's objects so an object path can be resolved
// to the human-friendly identifier iwd exposes on that object. The public layer
// uses it to enrich Properties bundles without importing iwdbus directly.
type Resolver interface {
	Resolve(ctx context.Context) (Tree, error)
}

// Tree resolves an object path to a friendly identifier, returning "" for an
// unknown path or one lacking the expected interface/property.
type Tree interface {
	NetworkName(path string) string
	BSSAddress(path string) string
	DeviceName(path string) string
	AdapterName(path string) string
	KnownNetworkName(path string) string
}

// Resolver returns the friendly-reference resolver: a test override when set,
// otherwise one backed by the wiring's D-Bus connection, or nil when the wiring
// has no connection (so callers stay nil-safe).
func (w *Wiring) Resolver() Resolver {
	if w == nil {
		return nil
	}
	if w.ResolverOverride != nil {
		return w.ResolverOverride
	}
	if w.Conn == nil {
		return nil
	}
	return &wiringResolver{conn: w.Conn}
}

// NoResolver is a Resolver that resolves nothing, for fake-backed tests whose
// placeholder Conn cannot service a GetManagedObjects call. Its nil Tree leaves
// every bundle ref path-only.
type NoResolver struct{}

// Resolve implements Resolver.
func (NoResolver) Resolve(context.Context) (Tree, error) { return nil, nil }

type wiringResolver struct {
	conn *dbus.Conn
}

func (r *wiringResolver) Resolve(ctx context.Context) (Tree, error) {
	t, err := iwdbus.FetchObjectTree(ctx, r.conn)
	if err != nil {
		return nil, err
	}
	return objectTree{t: t}, nil
}

// objectTree adapts *iwdbus.ObjectTree to the Tree interface, collapsing the
// (value, found) lookups to "" on a miss.
type objectTree struct {
	t *iwdbus.ObjectTree
}

func (o objectTree) NetworkName(path string) string { s, _ := o.t.NetworkName(path); return s }
func (o objectTree) BSSAddress(path string) string  { s, _ := o.t.BSSAddress(path); return s }
func (o objectTree) DeviceName(path string) string  { s, _ := o.t.DeviceName(path); return s }
func (o objectTree) AdapterName(path string) string { s, _ := o.t.AdapterName(path); return s }
func (o objectTree) KnownNetworkName(path string) string {
	s, _ := o.t.KnownNetworkName(path)
	return s
}
