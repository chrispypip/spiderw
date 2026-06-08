//go:build regression

package iwdbus

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
	"github.com/stretchr/testify/require"
)

func TestRegression_Iwdbus_IntrospectedObject_CloseDuringEmit(t *testing.T) {
	// Intentionally not parallel: this test stresses goroutine interleavings
	// and relies on global scheduling behavior.
	const iterations = 50

	for iter := range iterations {
		t.Run("iter_"+itoa(iter), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			intro := &IntrospectedObject{
				Conn:             nil, // hermetic: no real DBus
				BusName:          "test.bus",
				Path:             dbus.ObjectPath("/test/path"),
				ctx:              ctx,
				cancel:           cancel,
				ifaces:           map[string]*introspect.Interface{},
				sigCh:            make(chan *dbus.Signal, dbusSignalCapacity),
				handlersExact:    map[string][]signalHandler{},
				handlersWildcard: map[string][]signalHandler{},
			}

			const handlerCount = 8
			var calls atomic.Int64

			for range handlerCount {
				err := intro.RegisterSignalHandler("test.iface", "TestSignal", func(*dbus.Signal) {
					calls.Add(1)
				})
				require.NoError(t, err)
			}

			stop := make(chan struct{})
			go func() {
				sig := &dbus.Signal{
					Name: "test.iface.TestSignal",
					Path: intro.Path,
					Body: []interface{}{"x"},
				}
				for {
					select {
					case <-stop:
						return
					default:
						select {
						case intro.sigCh <- sig:
						default:
							time.Sleep(50 * time.Microsecond)
						}
					}
				}
			}()

			// Ensure dispatcher is active
			deadline := time.Now().Add(200 * time.Millisecond)
			for time.Now().Before(deadline) {
				if calls.Load() > 0 {
					break
				}
				time.Sleep(1 * time.Millisecond)
			}
			if calls.Load() == 0 {
				close(stop)
				t.Fatalf("dispatcher did not process any signals")
			}

			// Close while signals are actively being emitted.
			_ = intro.Close()

			// Stop emitter after Close returns.
			close(stop)

			// Assert no handler invocations after Close returns.
			c1 := calls.Load()
			time.Sleep(50 * time.Millisecond)
			c2 := calls.Load()

			if c2 != c1 {
				t.Fatalf("handler calls continued after Close returned: before=%d after=%d", c1, c2)
			}

			// Close should be idempotent and safe.
			_ = intro.Close()
		})
	}
}

// tiny helper to avoid fmt.Sprintf allocations in tight loops
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [20]byte
	n := len(b)
	for i > 0 {
		n--
		b[n] = byte('0' + i%10)
		i /= 10
	}
	return string(b[n:])
}
