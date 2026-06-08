//go:build unit || stress || race

package iwdbus

import (
	"context"
	"testing"

	"github.com/godbus/dbus/v5"
)

type fakeSignalSource struct {
	intro *IntrospectedObject
}

func newFakeSignalSource(t *testing.T) *fakeSignalSource {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	intro := &IntrospectedObject{
		Conn:             nil,
		BusName:          "net.connman.iwd",
		Path:             "/net/connman/iwd/phy0",
		ctx:              ctx,
		sigCh:            make(chan *dbus.Signal, 1024),
		handlersExact:    map[string][]signalHandler{},
		handlersWildcard: map[string][]signalHandler{},
	}

	intro.startDispatcher()
	return &fakeSignalSource{intro: intro}
}

func (f *fakeSignalSource) RegisterSignalHandler(iface, member string, handler func(*dbus.Signal)) error {
	return f.intro.RegisterSignalHandler(iface, member, handler)
}

func (f *fakeSignalSource) RegisterSignalHandlerWithUnsubscribe(iface, member string, handler func(*dbus.Signal)) (UnsubscribeFunc, error) {
	return f.intro.RegisterSignalHandlerWithUnsubscribe(iface, member, handler)
}

func (f *fakeSignalSource) emit(iface, member string, body ...any) {
	f.intro.sigCh <- &dbus.Signal{
		Name: iface + "." + member,
		Path: f.intro.Path,
		Body: body,
	}
}
