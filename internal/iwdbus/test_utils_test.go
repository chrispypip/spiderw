//go:build unit || stress || race || regression

package iwdbus

import (
	"context"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/require"
)

const (
	signalTimeout = 1 * time.Second
	pollInterval  = 5 * time.Millisecond
)

// requireFired asserts that ch receives a value within signalTimeout.
func requireFired(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	require.Eventually(t, func() bool {
		select {
		case <-ch:
			return true
		default:
			return false
		}
	}, signalTimeout, pollInterval)
}

// requireNotFired asserts that ch stays empty for the duration of signalTimeout.
func requireNotFired(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	require.Eventually(t, func() bool {
		select {
		case <-ch:
			return false
		default:
			return true
		}
	}, signalTimeout, pollInterval)
}

type fakeSignalSource struct {
	intro *IntrospectedObject
}

func newFakeSignalSource(t *testing.T) *fakeSignalSource {
	t.Helper()
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
