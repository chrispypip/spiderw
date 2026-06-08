//go:build race

package iwdbus

import (
	"context"
	"sync"
	"testing"

	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/require"
)

func TestRace_Iwdbus_Dispatch_CloseEmit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	intro := &IntrospectedObject{
		Conn:             nil,
		BusName:          "net.connman.iwd",
		Path:             "/net/connman/iwd/0",
		ctx:              ctx,
		sigCh:            make(chan *dbus.Signal, 256),
		handlersExact:    map[string][]signalHandler{},
		handlersWildcard: map[string][]signalHandler{},
	}

	go intro.startDispatcher()

	_ = intro.RegisterSignalHandler("*", "*", func(*dbus.Signal) {})

	var wg sync.WaitGroup

	for i := range 100 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			intro.sigCh <- &dbus.Signal{
				Name: "net.connman.iwd.Adapter.PoweredChanged",
				Body: []interface{}{i},
			}
		}(i)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		cancel()
		_ = intro.Close()
	}()

	wg.Wait()
}

func TestRace_Iwdbus_ParseIntrospectionChildNames_Concurrent(t *testing.T) {
	// This helper must be safe to call concurrently and must never panic.
	xml := `<node><node name="phy0"/><node name="phy1"/><node name="x"/></node>`

	const N = 250
	var wg sync.WaitGroup
	wg.Add(N)

	errCh := make(chan error, N)
	for range N {
		go func() {
			defer wg.Done()
			_, err := parseIntrospectionChildNames(xml)
			errCh <- err
		}()
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		require.NoError(t, err)
	}
}
