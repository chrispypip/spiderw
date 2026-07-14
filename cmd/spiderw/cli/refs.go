package cli

import (
	"context"
	"strings"

	"github.com/chrispypip/spiderw"
)

// nameRef is the CLI view of a resolved, name-bearing object reference (network,
// device, adapter, known network). Human output shows the friendly Name, falling
// back to the object path when it could not be resolved; JSON exposes both.
type nameRef struct {
	Name string `json:"Name"`
	Path string `json:"Path"`
}

func (r nameRef) readable() string {
	if r.Name != "" {
		return r.Name
	}
	return r.Path
}

// addrRef is the CLI view of a resolved BSS reference: it renders the Address
// (BSSID) rather than a Name.
type addrRef struct {
	Address string `json:"Address"`
	Path    string `json:"Path"`
}

func (r addrRef) readable() string {
	if r.Address != "" {
		return r.Address
	}
	return r.Path
}

func toNameRef(name, path string) nameRef { return nameRef{Name: name, Path: path} }
func toAddrRef(addr, path string) addrRef { return addrRef{Address: addr, Path: path} }

func toAddrRefs(rs []spiderw.BasicServiceSetRef) []addrRef {
	out := make([]addrRef, 0, len(rs))
	for _, r := range rs {
		out = append(out, toAddrRef(r.Address, r.Path))
	}
	return out
}

// readableAddrRefs renders a list of BSS refs as a comma-joined string, or "-"
// when empty.
func readableAddrRefs(rs []addrRef) string {
	if len(rs) == 0 {
		return "-"
	}
	parts := make([]string, 0, len(rs))
	for _, r := range rs {
		parts = append(parts, r.readable())
	}
	return strings.Join(parts, ", ")
}

// optionalPathText renders an optional object path for the human-readable
// monitor output. iwd reports "no such object" as a null path, which the library
// surfaces as nil, so the absent case gets a word rather than an empty string.
func optionalPathText(path *string, absent string) string {
	if path == nil || *path == "" {
		return absent
	}
	return *path
}

// pathListText renders a list of object paths for the human-readable monitor
// output. An empty list is explicit rather than blank.
func pathListText(paths []string) string {
	if len(paths) == 0 {
		return "none"
	}
	return strings.Join(paths, ", ")
}

// refPath extracts the object path from an optional NetworkRef, preserving nil.
// The monitor commands stream raw paths (matching the subscribe callbacks), while
// Properties() hands back resolved refs.
func refPath(ref *spiderw.NetworkRef) *string {
	if ref == nil {
		return nil
	}
	return &ref.Path
}

// bssRefPath extracts the object path from an optional BasicServiceSetRef.
func bssRefPath(ref *spiderw.BasicServiceSetRef) *string {
	if ref == nil {
		return nil
	}
	return &ref.Path
}

// bssRefPaths extracts the object paths from a BasicServiceSetRef list.
func bssRefPaths(refs []spiderw.BasicServiceSetRef) []string {
	if len(refs) == 0 {
		return nil
	}
	out := make([]string, 0, len(refs))
	for _, ref := range refs {
		out = append(out, ref.Path)
	}
	return out
}

// monitorResolver turns the raw object paths a subscription delivers into the
// friendly identifiers the monitor prints — an SSID for a network, a MAC for a
// BSS. `status` already renders resolved refs, so `monitor` matches it rather than
// printing bare paths.
//
// Resolution is best-effort: a path that cannot be resolved (the object may
// already be gone by the time the signal is handled, which is common on a
// disconnect) falls back to the path itself, and never fails the stream.
type monitorResolver struct {
	client clientAPI
}

// networkRef resolves a network path to its SSID.
func (r monitorResolver) networkRef(ctx context.Context, path *string) *nameRef {
	if path == nil || *path == "" {
		return nil
	}
	ref := nameRef{Path: *path}
	if r.client != nil {
		if n, err := r.client.Network(ctx, *path); err == nil && n != nil {
			if name, err := n.Name(ctx); err == nil {
				ref.Name = name
			}
		}
	}
	return &ref
}

// bssRef resolves a BSS path to its MAC address.
func (r monitorResolver) bssRef(ctx context.Context, path *string) *addrRef {
	if path == nil || *path == "" {
		return nil
	}
	ref := addrRef{Path: *path}
	if r.client != nil {
		if b, err := r.client.BasicServiceSet(ctx, *path); err == nil && b != nil {
			if addr, err := b.Address(ctx); err == nil {
				ref.Address = addr
			}
		}
	}
	return &ref
}

// bssRefs resolves a list of BSS paths to their MAC addresses.
func (r monitorResolver) bssRefs(ctx context.Context, paths []string) []addrRef {
	if len(paths) == 0 {
		return nil
	}
	out := make([]addrRef, 0, len(paths))
	for _, p := range paths {
		if ref := r.bssRef(ctx, &p); ref != nil {
			out = append(out, *ref)
		}
	}
	return out
}

// knownNetworkRef resolves a known-network path to its name.
func (r monitorResolver) knownNetworkRef(ctx context.Context, path *string) *nameRef {
	if path == nil || *path == "" {
		return nil
	}
	ref := nameRef{Path: *path}
	if r.client != nil {
		if k, err := r.client.KnownNetwork(ctx, *path); err == nil && k != nil {
			if name, err := k.Name(ctx); err == nil {
				ref.Name = name
			}
		}
	}
	return &ref
}

// readableNameRef renders an optional resolved ref, or a word when it is absent.
func readableNameRef(ref *nameRef, absent string) string {
	if ref == nil {
		return absent
	}
	return ref.readable()
}

// readableAddrRef renders an optional resolved BSS ref, or a word when absent.
func readableAddrRef(ref *addrRef, absent string) string {
	if ref == nil {
		return absent
	}
	return ref.readable()
}

// netRefOf and bssAddrRefOf convert the already-resolved refs a Properties() read
// returns into the monitor's optional ref shape, so the seed line and the streamed
// lines render identically.
func netRefOf(ref *spiderw.NetworkRef) *nameRef {
	if ref == nil {
		return nil
	}
	r := toNameRef(ref.Name, ref.Path)
	return &r
}

func bssAddrRefOf(ref *spiderw.BasicServiceSetRef) *addrRef {
	if ref == nil {
		return nil
	}
	r := toAddrRef(ref.Address, ref.Path)
	return &r
}
