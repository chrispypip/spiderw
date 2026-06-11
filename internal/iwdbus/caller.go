package iwdbus

import (
	"context"

	"github.com/godbus/dbus/v5"
)

// caller is the minimal dependency that Adapter, Daemon, Station, etc.
// need from an IntrospectedObject.
//
// This makes D-Bus IO mockable in unit tests.
type caller interface {
	Call(ctx context.Context, iface, method string, args ...interface{}) ([]interface{}, error)
	GetProperty(ctx context.Context, iface, prop string) (interface{}, error)
	GetAll(ctx context.Context, iface string) (map[string]dbus.Variant, error)
	SetProperty(ctx context.Context, iface, prop string, val interface{}) error
	HasInterface(name string) bool
}

type signalSource interface {
	RegisterSignalHandler(iface, member string, handler func(*dbus.Signal)) error
	RegisterSignalHandlerWithUnsubscribe(iface, member string, handler func(*dbus.Signal)) (UnsubscribeFunc, error)
}
