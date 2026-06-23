package spiderw

import (
	"context"

	"github.com/chrispypip/spiderw/internal/core"
	"github.com/chrispypip/spiderw/internal/logging"
)

// BasicServiceSetProperties is a snapshot of all BSS properties read in a single
// D-Bus call. The iwd BasicServiceSet interface exposes only Address.
type BasicServiceSetProperties struct {
	// Address is the BSS's hardware (BSSID) address.
	Address string
}

// BasicServiceSet provides high-level operations for a specific iwd basic
// service set (BSS) object.
//
// A BSS is a single access point or peer that the radio can see; iwd exposes it
// as a read-only object whose only property is its Address (BSSID).
type BasicServiceSet struct {
	core core.BasicServiceSetIface
	path string
}

func newBasicServiceSet(c core.BasicServiceSetIface, path string) *BasicServiceSet {
	if c == nil {
		return nil
	}
	return &BasicServiceSet{core: c, path: path}
}

// Path returns the D-Bus object path the BSS was constructed from.
//
// Path is static object identity, not an iwd property: it requires no D-Bus
// round-trip and never fails. Path returns "" for a nil receiver.
func (b *BasicServiceSet) Path() string {
	if b == nil {
		return ""
	}
	return b.path
}

func (b *BasicServiceSet) coreBSS(ctx context.Context, op string) (core.BasicServiceSetIface, error) {
	if b == nil || b.core == nil {
		logging.FromContext(ctx).Error(ctx, "basic service set wrapper uninitialized", "op", op)
		return nil, wrapPublicError(op, ErrInternal)
	}
	return b.core, nil
}

// Address returns the BSS's hardware (BSSID) address.
func (b *BasicServiceSet) Address(ctx context.Context) (string, error) {
	return delegate(ctx, "BasicServiceSet.Address", b.coreBSS, func(ctx context.Context, c core.BasicServiceSetIface) (string, error) {
		return c.Address(ctx)
	})
}

// Properties reads every BSS property in a single D-Bus call
// (Properties.GetAll). The iwd BasicServiceSet interface exposes only Address.
func (b *BasicServiceSet) Properties(ctx context.Context) (*BasicServiceSetProperties, error) {
	return delegate(ctx, "BasicServiceSet.Properties", b.coreBSS, func(ctx context.Context, c core.BasicServiceSetIface) (*BasicServiceSetProperties, error) {
		cp, err := c.Properties(ctx)
		if err != nil {
			return nil, err
		}
		return &BasicServiceSetProperties{Address: cp.Address}, nil
	})
}
