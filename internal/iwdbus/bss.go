package iwdbus

import (
	"context"
	"fmt"

	"github.com/godbus/dbus/v5"
)

// IwdBasicServiceSetIface is the fully qualified D-Bus interface name for iwd
// basic service sets (BSSes).
const IwdBasicServiceSetIface = IwdService + ".BasicServiceSet"

// BasicServiceSet wraps an iwd BasicServiceSet object using runtime
// introspection.
//
// The iwd BasicServiceSet interface exposes a single read-only property,
// Address, and has no methods or signals, so this wrapper holds only a caller.
type BasicServiceSet struct {
	call caller
}

// NewBasicServiceSet creates a BasicServiceSet for the given iwd object path.
func NewBasicServiceSet(ctx context.Context, conn *dbus.Conn, path dbus.ObjectPath) (*BasicServiceSet, error) {
	intro, err := NewIntrospectedObject(ctx, conn, IwdService, path)
	if err != nil {
		return nil, WrapIntrospection(string(path), err)
	}
	if !intro.HasInterface(IwdBasicServiceSetIface) {
		_ = intro.Close()
		return nil, fmt.Errorf("object %s does not implement %s", path, IwdBasicServiceSetIface)
	}
	return &BasicServiceSet{
		call: caller(intro),
	}, nil
}

// GetAddress reads the Address (BSSID) property.
func (b *BasicServiceSet) GetAddress(ctx context.Context) (string, error) {
	if err := b.ensureInitialized(); err != nil {
		return "", WrapConnection("BasicServiceSet.ensureInitialized", err)
	}

	value, err := b.call.GetProperty(ctx, IwdBasicServiceSetIface, "Address")
	if err != nil {
		return "", WrapProperty(IwdBasicServiceSetIface, "Address", err)
	}

	s, ok := value.(string)
	if !ok {
		return "", WrapVariant("Address", fmt.Errorf("expected string, got %T", value))
	}
	return s, nil
}

// BasicServiceSetProperties holds every BSS property read in a single
// Properties.GetAll call. The iwd BasicServiceSet interface exposes only
// Address, which is required.
type BasicServiceSetProperties struct {
	Address string
}

// GetProperties reads every BSS property in a single Properties.GetAll call.
// Address is required; a missing one is an error.
func (b *BasicServiceSet) GetProperties(ctx context.Context) (*BasicServiceSetProperties, error) {
	if err := b.ensureInitialized(); err != nil {
		return nil, WrapConnection("BasicServiceSet.ensureInitialized", err)
	}

	raw, err := b.call.GetAll(ctx, IwdBasicServiceSetIface)
	if err != nil {
		return nil, WrapProperty(IwdBasicServiceSetIface, "GetAll", err)
	}

	addressV, ok := raw["Address"]
	if !ok {
		return nil, WrapProperty(IwdBasicServiceSetIface, "Address", fmt.Errorf("missing required property"))
	}
	address, ok := addressV.Value().(string)
	if !ok {
		return nil, WrapVariant("Address", fmt.Errorf("expected string, got %T", addressV.Value()))
	}

	return &BasicServiceSetProperties{Address: address}, nil
}

// ensureInitialized verifies that b has been initialized by NewBasicServiceSet.
func (b *BasicServiceSet) ensureInitialized() error {
	if b.call == nil {
		return ErrBasicServiceSetUninitialized
	}
	return nil
}
