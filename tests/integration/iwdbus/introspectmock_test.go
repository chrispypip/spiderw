//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/require"

	"github.com/chrispypip/spiderw/internal/iwdbus"
	"github.com/chrispypip/spiderw/tests/testutil/iwdmock"
)

func TestIntrospectMock_NewIntrospectedObject(t *testing.T) {
	tmpDir := t.TempDir()

	iwdmock.StartMockNormal(t, tmpDir)

	_, obj := newTestDBus(t, "net.connman.iwd", "/net/connman/iwd")
	require.True(t, obj.HasInterface("net.connman.iwd.Daemon"))
}

func TestIntrospectMock_Call_GetInfo(t *testing.T) {
	tmpDir := t.TempDir()

	iwdmock.StartMockNormal(t, tmpDir)

	ctx := context.Background()
	_, obj := newTestDBus(t, "net.connman.iwd", "/net/connman/iwd")

	body, err := obj.Call(ctx, "net.connman.iwd.Daemon", "GetInfo")
	require.NoError(t, err)
	require.NotEmpty(t, body)

	_, ok := body[0].(map[string]dbus.Variant)
	require.True(t, ok)
}

func TestIntrospectMock_GetProperty(t *testing.T) {
	tmpDir := t.TempDir()

	iwdmock.StartMockNormal(t, tmpDir)

	_, obj := newTestDBus(t, "net.connman.iwd", "/net/connman/iwd/0")

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	require.NotNil(t, ctx)
	t.Cleanup(func() { cancel() })

	prop, err := obj.GetProperty(ctx, "net.connman.iwd.Adapter", "Powered")
	require.NoError(t, err)

	powered, ok := prop.(bool)
	require.True(t, ok)
	require.True(t, powered)
}

func TestIntrospectMock_SetProperty(t *testing.T) {
	tmpDir := t.TempDir()

	iwdmock.StartMockNormal(t, tmpDir)

	ctx := context.Background()
	_, obj := newTestDBus(t, "net.connman.iwd", "/net/connman/iwd/0")

	err := obj.SetProperty(ctx, "net.connman.iwd.Adapter", "Powered", false)
	require.NoError(t, err)

	powered, err := obj.GetProperty(ctx, "net.connman.iwd.Adapter", "Powered")
	require.NoError(t, err, "GetProperty failed")

	value, ok := powered.(bool)
	require.True(t, ok)
	require.False(t, value)
}

func TestIntrospectMock_SignalHandling(t *testing.T) {
	tmpDir := t.TempDir()

	iwdmock.StartMockNormal(t, tmpDir)

	conn, obj := newTestDBus(t, "net.connman.iwd", "/net/connman/iwd/0")

	fired := make(chan struct{}, 1)
	err := obj.RegisterSignalHandler("net.connman.iwd.Adapter", "PoweredChanged",
		func(sig *dbus.Signal) {
			if sig.Body[0] == true {
				fired <- struct{}{}
			}
		})
	require.NoError(t, err, "failed to register signal handler")

	err = conn.Emit("/net/connman/iwd/0", "net.connman.iwd.Adapter.PoweredChanged", true)
	require.NoError(t, err, "Emit failed")

	requireFired(t, fired)
}

func TestIntrospectMock_WildcardSignalHandling(t *testing.T) {
	tmpDir := t.TempDir()

	iwdmock.StartMockNormal(t, tmpDir)

	conn, obj := newTestDBus(t, "net.connman.iwd", "/net/connman/iwd/0")

	fired := make(chan struct{}, 1)
	err := obj.RegisterSignalHandler("*", "PoweredChanged", func(sig *dbus.Signal) {
		fired <- struct{}{}
	})
	require.NoError(t, err)

	err = conn.Emit("/net/connman/iwd/0", "net.connman.iwd.Adapter.PoweredChanged", true)
	require.NoError(t, err)

	requireFired(t, fired)
}

func TestIntrospectMock_CloseStopsSignals(t *testing.T) {
	tmpDir := t.TempDir()

	iwdmock.StartMockNormal(t, tmpDir)

	_, obj := newTestDBus(t, "net.connman.iwd", "/net/connman/iwd/0")

	fired := make(chan struct{}, 1)
	err := obj.RegisterSignalHandler("*", "PoweredChanged", func(sig *dbus.Signal) {
		close(fired)
	})
	require.NoError(t, err)
	require.NoError(t, obj.Close())

	// Emit from a NEW dbus connection, not the one obj closed.
	conn2, err := dbus.SessionBus()
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn2.Close() })

	err = conn2.Emit("/net/connman/iwd/0", "net.connman.iwd.Adapter.PoweredChanged", true)
	require.NoError(t, err)

	requireNotFired(t, fired)
}

func TestIntrospectMock_MultipleExactHandlers(t *testing.T) {
	tmpDir := t.TempDir()

	iwdmock.StartMockNormal(t, tmpDir)

	conn, obj := newTestDBus(t, "net.connman.iwd", "/net/connman/iwd/0")

	h1 := make(chan struct{}, 1)
	h2 := make(chan struct{}, 1)

	err := obj.RegisterSignalHandler("net.connman.iwd.Adapter", "PoweredChanged", func(*dbus.Signal) {
		h1 <- struct{}{}
	})
	require.NoError(t, err)

	err = obj.RegisterSignalHandler("net.connman.iwd.Adapter", "PoweredChanged", func(*dbus.Signal) {
		h2 <- struct{}{}
	})
	require.NoError(t, err)

	err = conn.Emit("/net/connman/iwd/0", "net.connman.iwd.Adapter.PoweredChanged", true)
	require.NoError(t, err)

	requireFired(t, h1)
	requireFired(t, h2)
}

func TestIntrospectMock_WildcardIgnoresOtherMembers(t *testing.T) {
	tmpDir := t.TempDir()

	iwdmock.StartMockNormal(t, tmpDir)

	conn, obj := newTestDBus(t, "net.connman.iwd", "/net/connman/iwd/0")

	fired := make(chan struct{}, 1)

	err := obj.RegisterSignalHandler("*", "PoweredChanged", func(*dbus.Signal) {
		fired <- struct{}{}
	})
	require.NoError(t, err)

	err = conn.Emit("/net/connman/iwd/0", "net.connman.iwd.Adapter.OtherSignal", true)
	require.NoError(t, err)

	requireNotFired(t, fired)
}

func TestIntrospectMock_SignalBodyContent(t *testing.T) {
	tmpDir := t.TempDir()

	iwdmock.StartMockNormal(t, tmpDir)

	conn, obj := newTestDBus(t, "net.connman.iwd", "/net/connman/iwd/0")

	fired := make(chan struct{}, 1)

	err := obj.RegisterSignalHandler("net.connman.iwd.Adapter", "PoweredChanged", func(sig *dbus.Signal) {
		require.Equal(t, true, sig.Body[0])
		fired <- struct{}{}
	})
	require.NoError(t, err)

	err = conn.Emit("/net/connman/iwd/0", "net.connman.iwd.Adapter.PoweredChanged", true)
	require.NoError(t, err)

	requireFired(t, fired)
}

func TestIntrospectMock_PathMismatchDoesNotFire(t *testing.T) {
	tmpDir := t.TempDir()

	iwdmock.StartMockNormal(t, tmpDir)

	conn, obj := newTestDBus(t, "net.connman.iwd", "/net/connman/iwd/0")

	fired := make(chan struct{}, 1)

	err := obj.RegisterSignalHandler("net.connman.iwd.Adapter", "PoweredChanged", func(*dbus.Signal) {
		fired <- struct{}{}
	})
	require.NoError(t, err)

	// Emit on WRONG path
	err = conn.Emit("/net/connman/iwd/1", "net.connman.iwd.Adapter.PoweredChanged", true)
	require.NoError(t, err)

	requireNotFired(t, fired)
}

func TestIntrospectMock_SignalStorm(t *testing.T) {
	// NOTE: This test depends on the DBus daemon delivering all signals. This is
	// true for our mock and small volumes, but not guaranteed by DBus spec.

	tmpDir := t.TempDir()

	iwdmock.StartMockNormal(t, tmpDir)

	conn, obj := newTestDBus(t, "net.connman.iwd", "/net/connman/iwd/0")

	const N = 50
	recv := make(chan struct{}, N)

	err := obj.RegisterSignalHandler("net.connman.iwd.Adapter", "PoweredChanged", func(*dbus.Signal) {
		recv <- struct{}{}
	})
	require.NoError(t, err)

	for range N {
		err := conn.Emit("/net/connman/iwd/0", "net.connman.iwd.Adapter.PoweredChanged", true)
		require.NoError(t, err)
	}

	require.Eventually(t, func() bool { return len(recv) == N },
		2*time.Second,
		10*time.Millisecond,
		"only go %d/%d signals", len(recv), N,
	)
}

func TestIntrospectMock_ConcurrentRegistration(t *testing.T) {
	tmpDir := t.TempDir()

	iwdmock.StartMockNormal(t, tmpDir)

	_, obj := newTestDBus(t, "net.connman.iwd", "/net/connman/iwd/0")

	const N = 100
	done := make(chan struct{})

	for i := range N {
		go func(i int) {
			_ = obj.RegisterSignalHandler("*", fmt.Sprintf("M%d", i), func(*dbus.Signal) {})
			if i == N-1 {
				close(done)
			}
		}(i)
	}

	requireFired(t, done)
}

func TestIntrospectMock_HandlerPanicDoesNotCrashDispatcher(t *testing.T) {
	tmpDir := t.TempDir()

	iwdmock.StartMockNormal(t, tmpDir)

	conn, obj := newTestDBus(t, "net.connman.iwd", "/net/connman/iwd/0")

	fired := make(chan struct{})

	err := obj.RegisterSignalHandler("*", "PoweredChanged", func(*dbus.Signal) {
		close(fired)
		panic("test panic")
	})
	require.NoError(t, err)

	_ = conn.Emit("/net/connman/iwd/0", "net.connman.iwd.Adapter.PoweredChanged", true)
	requireFired(t, fired)

	// If dispatcher is alive, this next emit won't crash
	_ = conn.Emit("/net/connman/iwd/0", "net.connman.iwd.Adapter.PoweredChanged", true)
}

func newTestDBus(t *testing.T, busName string, path dbus.ObjectPath) (*dbus.Conn, *iwdbus.IntrospectedObject) {
	t.Helper()

	conn, err := dbus.SessionBus()
	require.NoError(t, err)

	obj, err := iwdbus.NewIntrospectedObject(context.Background(), conn, busName, path)
	t.Cleanup(func() {
		_ = obj.Close()
		_ = conn.Close()
	})
	require.NoError(t, err)

	return conn, obj
}
