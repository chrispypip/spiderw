package cli

import (
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
