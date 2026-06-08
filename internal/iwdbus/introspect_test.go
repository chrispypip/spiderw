//go:build unit

package iwdbus

import (
	"context"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
	"github.com/stretchr/testify/require"
)

const signalTimeoutMs = 500

func TestIntrospectedObject(t *testing.T) {
	t.Parallel()

	t.Run("HasInterface", func(t *testing.T) {
		t.Parallel()

		intro := newTestIntro(t)
		intro.ifaces["net.connman.iwd.Adapter"] = &introspect.Interface{}
		intro.ifaces[DBusIntrospectableIface] = &introspect.Interface{}

		require.True(t, intro.HasInterface("net.connman.iwd.Adapter"))
		require.True(t, intro.HasInterface(DBusIntrospectableIface))
		require.False(t, intro.HasInterface("nonexistent.iface"))
	})

	t.Run("Dispatch", func(t *testing.T) {
		t.Run("ExactMatch", func(t *testing.T) {
			t.Parallel()

			intro := newTestIntro(t)
			done := make(chan struct{})
			intro.mu.Lock()
			intro.handlersExact["net.connman.iwd.Adapter.PoweredChanged"] = []signalHandler{
				{fn: func(signal *dbus.Signal) { close(done) }},
			}
			intro.mu.Unlock()

			intro.startDispatcher()
			t.Cleanup(func() { shutdownIntro(t, intro) })
			intro.sigCh <- &dbus.Signal{Name: "net.connman.iwd.Adapter.PoweredChanged"}
			requireSignal(t, done)
		})

		t.Run("WildcardMatch", func(t *testing.T) {
			t.Parallel()

			intro := newTestIntro(t)
			done := make(chan struct{})
			intro.mu.Lock()
			intro.handlersWildcard["PoweredChanged"] = []signalHandler{
				{fn: func(signal *dbus.Signal) { close(done) }},
			}
			intro.mu.Unlock()

			intro.startDispatcher()
			t.Cleanup(func() { shutdownIntro(t, intro) })
			intro.sigCh <- &dbus.Signal{Name: "net.connman.iwd.Adapter.PoweredChanged"}
			requireSignal(t, done)
		})

		t.Run("ExactAndWildcardBothFire", func(t *testing.T) {
			t.Parallel()

			intro := newTestIntro(t)
			exact := make(chan struct{})
			wild := make(chan struct{})
			intro.mu.Lock()
			intro.handlersExact["iface.Member"] = []signalHandler{{fn: func(signal *dbus.Signal) { close(exact) }}}
			intro.handlersWildcard["Member"] = []signalHandler{{fn: func(signal *dbus.Signal) { close(wild) }}}
			intro.mu.Unlock()

			intro.startDispatcher()
			t.Cleanup(func() { shutdownIntro(t, intro) })
			intro.sigCh <- &dbus.Signal{Name: "iface.Member"}

			timeout := time.After(signalTimeoutMs * time.Millisecond)
			gotExact := false
			gotWild := false
			for !gotExact || !gotWild {
				select {
				case <-exact:
					gotExact = true
				case <-wild:
					gotWild = true
				case <-timeout:
					t.Fatalf("did not receive exact=%v wild=%v", gotExact, gotWild)
				}
			}
		})

		t.Run("RegisteredWildcardDispatch", func(t *testing.T) {
			t.Run("InterfaceWildcardMatchesSameInterface", func(t *testing.T) {
				t.Parallel()

				intro := newTestIntro(t)
				t.Cleanup(func() { shutdownIntro(t, intro) })

				calls := 0
				err := intro.RegisterSignalHandler("net.connman.iwd.Adapter", "*", func(*dbus.Signal) {
					calls++
				})
				require.NoError(t, err)

				handlers := intro.resolveHandlers(&dbus.Signal{Name: "net.connman.iwd.Adapter.PoweredChanged"})
				require.Len(t, handlers, 1)

				invokeSignalHandlers(&dbus.Signal{Name: "net.connman.iwd.Adapter.PoweredChanged"}, handlers)
				require.Equal(t, 1, calls)
			})

			t.Run("InterfaceWildcardIgnoresOtherInterfaces", func(t *testing.T) {
				t.Parallel()

				intro := newTestIntro(t)
				t.Cleanup(func() { shutdownIntro(t, intro) })

				err := intro.RegisterSignalHandler("net.connman.iwd.Adapter", "*", func(*dbus.Signal) {})
				require.NoError(t, err)

				handlers := intro.resolveHandlers(&dbus.Signal{Name: "net.connman.iwd.Device.PoweredChanged"})
				require.Empty(t, handlers)
			})

			t.Run("MemberWildcardMatchesAcrossInterfaces", func(t *testing.T) {
				t.Parallel()

				intro := newTestIntro(t)
				t.Cleanup(func() { shutdownIntro(t, intro) })

				calls := 0
				err := intro.RegisterSignalHandler("*", "PoweredChanged", func(*dbus.Signal) {
					calls++
				})
				require.NoError(t, err)

				adapterHandlers := intro.resolveHandlers(&dbus.Signal{Name: "net.connman.iwd.Adapter.PoweredChanged"})
				require.Len(t, adapterHandlers, 1)
				invokeSignalHandlers(&dbus.Signal{Name: "net.connman.iwd.Adapter.PoweredChanged"}, adapterHandlers)

				deviceHandlers := intro.resolveHandlers(&dbus.Signal{Name: "net.connman.iwd.Device.PoweredChanged"})
				require.Len(t, deviceHandlers, 1)
				invokeSignalHandlers(&dbus.Signal{Name: "net.connman.iwd.Device.PoweredChanged"}, deviceHandlers)

				require.Equal(t, 2, calls)
			})

			t.Run("GlobalWildcardMatchesEverything", func(t *testing.T) {
				t.Parallel()

				intro := newTestIntro(t)
				t.Cleanup(func() { shutdownIntro(t, intro) })

				calls := 0
				err := intro.RegisterSignalHandler("*", "*", func(*dbus.Signal) {
					calls++
				})
				require.NoError(t, err)

				adapterHandlers := intro.resolveHandlers(&dbus.Signal{Name: "net.connman.iwd.Adapter.PoweredChanged"})
				require.Len(t, adapterHandlers, 1)
				invokeSignalHandlers(&dbus.Signal{Name: "net.connman.iwd.Adapter.PoweredChanged"}, adapterHandlers)

				deviceHandlers := intro.resolveHandlers(&dbus.Signal{Name: "net.connman.iwd.Device.ModeChanged"})
				require.Len(t, deviceHandlers, 1)
				invokeSignalHandlers(&dbus.Signal{Name: "net.connman.iwd.Device.ModeChanged"}, deviceHandlers)

				require.Equal(t, 2, calls)
			})

			t.Run("UnsubscribeRemovesOnlyRegisteredWildcardHandler", func(t *testing.T) {
				t.Parallel()

				intro := newTestIntro(t)
				t.Cleanup(func() { shutdownIntro(t, intro) })

				globalCalls := 0
				memberCalls := 0
				interfaceCalls := 0

				_, err := intro.RegisterSignalHandlerWithUnsubscribe("*", "*", func(*dbus.Signal) {
					globalCalls++
				})
				require.NoError(t, err)

				_, err = intro.RegisterSignalHandlerWithUnsubscribe("*", "PoweredChanged", func(*dbus.Signal) {
					memberCalls++
				})
				require.NoError(t, err)

				unsubscribeInterface, err := intro.RegisterSignalHandlerWithUnsubscribe("net.connman.iwd.Adapter", "*", func(*dbus.Signal) {
					interfaceCalls++
				})
				require.NoError(t, err)
				require.NoError(t, unsubscribeInterface.Unsubscribe())

				sig := &dbus.Signal{Name: "net.connman.iwd.Adapter.PoweredChanged"}
				handlers := intro.resolveHandlers(sig)
				require.Len(t, handlers, 2)

				invokeSignalHandlers(sig, handlers)
				require.Equal(t, 1, globalCalls)
				require.Equal(t, 1, memberCalls)
				require.Zero(t, interfaceCalls)
			})
		})

		t.Run("IgnoresMalformed", func(t *testing.T) {
			t.Parallel()

			intro := newTestIntro(t)
			intro.startDispatcher()
			intro.sigCh <- nil
			intro.sigCh <- &dbus.Signal{Name: ""}
			intro.sigCh <- &dbus.Signal{Name: "no_dots_here"}
			t.Cleanup(func() { shutdownIntro(t, intro) })
			// If no panic, test passes.
		})

		t.Run("NoMatch", func(t *testing.T) {
			t.Parallel()

			intro := newTestIntro(t)
			intro.startDispatcher()
			intro.sigCh <- &dbus.Signal{Name: "net.connman.iwd.Adapter.SomethingElse"}
			t.Cleanup(func() { shutdownIntro(t, intro) })
			// If no panic/deadlock, test passes.
		})

		t.Run("HandlerPanicSafe", func(t *testing.T) {
			t.Parallel()

			intro := newTestIntro(t)
			panicDone := make(chan struct{})
			intro.mu.Lock()
			intro.handlersExact["iface.Member"] = []signalHandler{
				{fn: func(signal *dbus.Signal) {
					close(panicDone)
					panic("test panic")
				}},
			}
			intro.mu.Unlock()

			intro.startDispatcher()
			t.Cleanup(func() { shutdownIntro(t, intro) })
			intro.sigCh <- &dbus.Signal{Name: "iface.Member"}
			requireSignal(t, panicDone)
			// If dispatcher survived, test passes.
		})

		t.Run("StopsOnCancel", func(t *testing.T) {
			t.Parallel()

			intro := newTestIntro(t)
			intro.startDispatcher()
			t.Cleanup(func() { shutdownIntro(t, intro) })

			intro.cancel()
			time.Sleep(50 * time.Millisecond)
		})
	})

	t.Run("Close", func(t *testing.T) {
		t.Run("Idempotent", func(t *testing.T) {
			intro := newTestIntro(t)
			intro.startDispatcher()
			// No cleanup: Close is called explicitly twice.

			err1 := intro.Close()
			err2 := intro.Close() // MUST NOT panic or crash

			require.NoError(t, err1)
			require.NoError(t, err2)
		})
	})
}

func TestIntrospect_ParseChildNames(t *testing.T) {
	t.Parallel()

	t.Run("InvalidXML", func(t *testing.T) {
		t.Parallel()
		_, err := parseIntrospectionChildNames("<not-xml")
		require.Error(t, err)
	})

	t.Run("EmptyNode", func(t *testing.T) {
		t.Parallel()
		out, err := parseIntrospectionChildNames(`<node></node>`)
		require.NoError(t, err)
		require.Empty(t, out)
	})

	t.Run("ImmediateChildrenOnly", func(t *testing.T) {
		t.Parallel()
		out, err := parseIntrospectionChildNames(`
            <node>
                <node name="phy0"/>
                <node name="phy1"/>
                <node name="nested">
                    <node name="child"/>
                </node>
            </node>`)
		require.NoError(t, err)
		require.Equal(t, []string{"phy0", "phy1", "nested"}, out)
	})

	t.Run("TrimWhitespaceAndIgnoreEmpty", func(t *testing.T) {
		t.Parallel()
		out, err := parseIntrospectionChildNames(
			"<node>" +
				"<node name=\"  phy0  \"/>" +
				"<node name=\"\t\"/>" +
				"<node name=\"\"/>" +
				"<node name=\"phy1\"/>" +
				"</node>",
		)
		require.NoError(t, err)
		require.Equal(t, []string{"phy0", "phy1"}, out)
	})
}

func TestIntrospect_ChildNames_Guards(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	_, err := introspectChildNames(ctx, nil, "svc", dbus.ObjectPath("/x"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "nil dbus conn")
}

func newTestIntro(t *testing.T) *IntrospectedObject {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	return &IntrospectedObject{
		ctx:              ctx,
		cancel:           cancel,
		ifaces:           map[string]*introspect.Interface{},
		sigCh:            make(chan *dbus.Signal, 16),
		handlersExact:    map[string][]signalHandler{},
		handlersWildcard: map[string][]signalHandler{},
	}
}

func shutdownIntro(t *testing.T, intro *IntrospectedObject) {
	t.Helper()
	require.NoError(t, intro.Close())
}

func requireSignal(t *testing.T, ch <-chan struct{}) {
	t.Helper()

	select {
	case <-ch:
		return
	case <-time.After(signalTimeoutMs * time.Millisecond):
		t.Fatal("timeout waiting for signal")
	}
}

func invokeSignalHandlers(sig *dbus.Signal, handlers []func(*dbus.Signal)) {
	for _, handler := range handlers {
		handler(sig)
	}
}
