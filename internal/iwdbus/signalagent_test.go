//go:build unit

package iwdbus

import (
	"testing"

	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/require"
)

func TestSignalLevelAgent_Iwdbus(t *testing.T) {
	t.Parallel()

	t.Run("Object", func(t *testing.T) {
		t.Parallel()
		t.Run("Changed", testSignalLevelAgentObject_Changed)
		t.Run("Changed_NilSafe", testSignalLevelAgentObject_Changed_NilSafe)
		t.Run("Release", testSignalLevelAgentObject_Release)
		t.Run("Release_NilSafe", testSignalLevelAgentObject_Release_NilSafe)
	})

	t.Run("Export", func(t *testing.T) {
		t.Parallel()
		t.Run("NilConn", testExportSignalLevelAgent_NilConn)
		t.Run("InvalidPath", testExportSignalLevelAgent_InvalidPath)
	})
}

func testSignalLevelAgentObject_Changed(t *testing.T) {
	t.Parallel()

	var (
		called    bool
		gotDevice dbus.ObjectPath
		gotLevel  uint8
	)
	a := &signalLevelAgentObject{handler: SignalLevelAgentHandler{
		Changed: func(device dbus.ObjectPath, level uint8) {
			called = true
			gotDevice = device
			gotLevel = level
		},
	}}

	derr := a.Changed("/net/connman/iwd/0/3", 2)
	require.Nil(t, derr)
	require.True(t, called)
	require.Equal(t, dbus.ObjectPath("/net/connman/iwd/0/3"), gotDevice)
	require.Equal(t, uint8(2), gotLevel)
}

func testSignalLevelAgentObject_Changed_NilSafe(t *testing.T) {
	t.Parallel()
	a := &signalLevelAgentObject{handler: SignalLevelAgentHandler{}}
	require.Nil(t, a.Changed("/net/connman/iwd/0/3", 1))
}

func testSignalLevelAgentObject_Release(t *testing.T) {
	t.Parallel()

	var called bool
	a := &signalLevelAgentObject{handler: SignalLevelAgentHandler{
		Release: func() { called = true },
	}}

	require.Nil(t, a.Release())
	require.True(t, called)
}

func testSignalLevelAgentObject_Release_NilSafe(t *testing.T) {
	t.Parallel()
	a := &signalLevelAgentObject{handler: SignalLevelAgentHandler{}}
	require.Nil(t, a.Release())
}

func testExportSignalLevelAgent_NilConn(t *testing.T) {
	t.Parallel()
	_, err := ExportSignalLevelAgent(nil, "/spiderw/signalagent", SignalLevelAgentHandler{})
	require.Error(t, err)
	require.ErrorIs(t, err, ErrDBusConnection)
}

func testExportSignalLevelAgent_InvalidPath(t *testing.T) {
	t.Parallel()
	// The path guard rejects an invalid object path before any conn method is
	// used, so a bare &dbus.Conn{} is never dereferenced.
	_, err := ExportSignalLevelAgent(&dbus.Conn{}, "not a valid path", SignalLevelAgentHandler{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid signal level agent object path")
}
