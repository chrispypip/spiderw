//go:build unit

package iwdbus

import (
	"context"
	"fmt"

	"github.com/godbus/dbus/v5"
)

type fakeCaller struct {
	callFn     func(ctx context.Context, iface, method string, args ...interface{}) ([]interface{}, error)
	getPropFn  func(ctx context.Context, iface, prop string) (interface{}, error)
	getAllFn   func(ctx context.Context, iface string) (map[string]dbus.Variant, error)
	setPropFn  func(ctx context.Context, iface, prop string, value interface{}) error
	hasIfaceFn func(name string) bool
}

// Call invokes a fake D-Bus method implementation.
func (f *fakeCaller) Call(ctx context.Context, iface, method string, args ...interface{}) ([]interface{}, error) {
	if f.callFn == nil {
		return nil, fmt.Errorf("unexpected Call(%s.%s)", iface, method)
	}
	return f.callFn(ctx, iface, method, args...)
}

// GetProperty returns a fake D-Bus property value.
func (f *fakeCaller) GetProperty(ctx context.Context, iface, prop string) (interface{}, error) {
	if f.getPropFn == nil {
		return nil, fmt.Errorf("unexpected GetProperty(%s.%s)", iface, prop)
	}

	return f.getPropFn(ctx, iface, prop)
}

// GetAll returns all fake D-Bus property values for iface.
func (f *fakeCaller) GetAll(ctx context.Context, iface string) (map[string]dbus.Variant, error) {
	if f.getAllFn == nil {
		return nil, fmt.Errorf("unexpected GetAll(%s)", iface)
	}
	return f.getAllFn(ctx, iface)
}

// SetProperty sets a fake D-Bus property value.
func (f *fakeCaller) SetProperty(ctx context.Context, iface, prop string, value interface{}) error {
	if f.setPropFn == nil {
		return fmt.Errorf("unexpected SetProperty(%s.%s)", iface, prop)
	}
	return f.setPropFn(ctx, iface, prop, value)
}

// HasInterface reports whether the fake object exposes iface.
func (f *fakeCaller) HasInterface(name string) bool {
	return f.hasIfaceFn(name)
}
