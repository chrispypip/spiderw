package spiderw

import (
	"context"

	"github.com/chrispypip/spiderw/internal/connect"
)

// resolveTree fetches an object-tree snapshot used to enrich the path-typed
// fields of a Properties bundle with their friendly identifiers. A nil resolver
// (e.g. an object built in a unit test) yields a nil tree and no error, so the
// bundle degrades to path-only refs. A fetch failure propagates to the caller.
func resolveTree(ctx context.Context, r connect.Resolver) (connect.Tree, error) {
	if r == nil {
		return nil, nil
	}
	return r.Resolve(ctx)
}

// The ref builders below fill Path always and the friendly identifier
// best-effort: a nil tree or an unknown path leaves Name/Address "".

func networkRefOf(tree connect.Tree, path string) NetworkRef {
	ref := NetworkRef{Path: path}
	if tree != nil {
		ref.Name = tree.NetworkName(path)
	}
	return ref
}

func bssRefOf(tree connect.Tree, path string) BasicServiceSetRef {
	ref := BasicServiceSetRef{Path: path}
	if tree != nil {
		ref.Address = tree.BSSAddress(path)
	}
	return ref
}

func deviceRefOf(tree connect.Tree, path string) DeviceRef {
	ref := DeviceRef{Path: path}
	if tree != nil {
		ref.Name = tree.DeviceName(path)
	}
	return ref
}

func adapterRefOf(tree connect.Tree, path string) AdapterRef {
	ref := AdapterRef{Path: path}
	if tree != nil {
		ref.Name = tree.AdapterName(path)
	}
	return ref
}
