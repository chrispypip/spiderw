//go:build unit

package iwdbus

import (
	"testing"

	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/require"
)

// These pure normalization helpers accept the several concrete forms a D-Bus
// property value can take (typed, string, variant, array). The Get* method
// tests only exercise the single form the mock returns, so each helper's
// alternate-form and validation branches are covered directly here.

func TestParseNetworkObjectPath(t *testing.T) {
	t.Parallel()

	t.Run("typed valid", func(t *testing.T) {
		got, err := parseNetworkObjectPath("Device", dbus.ObjectPath("/net/connman/iwd/0"))
		require.NoError(t, err)
		require.Equal(t, dbus.ObjectPath("/net/connman/iwd/0"), got)
	})
	t.Run("typed invalid", func(t *testing.T) {
		_, err := parseNetworkObjectPath("Device", dbus.ObjectPath("bad"))
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid object path")
	})
	t.Run("string valid", func(t *testing.T) {
		got, err := parseNetworkObjectPath("Device", "/net/connman/iwd/0")
		require.NoError(t, err)
		require.Equal(t, dbus.ObjectPath("/net/connman/iwd/0"), got)
	})
	t.Run("string invalid", func(t *testing.T) {
		_, err := parseNetworkObjectPath("Device", "bad")
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid object path")
	})
	t.Run("wrong type", func(t *testing.T) {
		_, err := parseNetworkObjectPath("Device", 42)
		require.Error(t, err)
		require.Contains(t, err.Error(), "expected object path")
	})
}

func TestParseOptionalObjectPath(t *testing.T) {
	t.Parallel()

	t.Run("nil yields nil", func(t *testing.T) {
		got, err := parseOptionalObjectPath(nil)
		require.NoError(t, err)
		require.Nil(t, got)
	})
	t.Run("typed path", func(t *testing.T) {
		got, err := parseOptionalObjectPath(dbus.ObjectPath("/net/connman/iwd/0/net"))
		require.NoError(t, err)
		require.NotNil(t, got)
		require.Equal(t, "/net/connman/iwd/0/net", *got)
	})
	t.Run("string path", func(t *testing.T) {
		got, err := parseOptionalObjectPath("/net/connman/iwd/0/net")
		require.NoError(t, err)
		require.Equal(t, "/net/connman/iwd/0/net", *got)
	})
	t.Run("variant unwraps", func(t *testing.T) {
		got, err := parseOptionalObjectPath(dbus.MakeVariant(dbus.ObjectPath("/net/connman/iwd/0/net")))
		require.NoError(t, err)
		require.Equal(t, "/net/connman/iwd/0/net", *got)
	})
	t.Run("root sentinel yields nil", func(t *testing.T) {
		got, err := parseOptionalObjectPath(dbus.ObjectPath("/"))
		require.NoError(t, err)
		require.Nil(t, got)
	})
	t.Run("empty yields nil", func(t *testing.T) {
		got, err := parseOptionalObjectPath("")
		require.NoError(t, err)
		require.Nil(t, got)
	})
	t.Run("invalid path", func(t *testing.T) {
		_, err := parseOptionalObjectPath("bad")
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid object path")
	})
	t.Run("wrong type", func(t *testing.T) {
		_, err := parseOptionalObjectPath(42)
		require.Error(t, err)
		require.Contains(t, err.Error(), "expected object path")
	})
}

func TestParseObjectPathList(t *testing.T) {
	t.Parallel()

	t.Run("typed slice", func(t *testing.T) {
		got, err := parseObjectPathList([]dbus.ObjectPath{"/a", "/b"})
		require.NoError(t, err)
		require.Equal(t, []string{"/a", "/b"}, got)
	})
	t.Run("typed slice invalid element", func(t *testing.T) {
		_, err := parseObjectPathList([]dbus.ObjectPath{"/a", "bad"})
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid object path")
	})
	t.Run("interface slice", func(t *testing.T) {
		got, err := parseObjectPathList([]interface{}{dbus.ObjectPath("/a"), "/b"})
		require.NoError(t, err)
		require.Equal(t, []string{"/a", "/b"}, got)
	})
	t.Run("interface slice invalid element", func(t *testing.T) {
		_, err := parseObjectPathList([]interface{}{42})
		require.Error(t, err)
		require.Contains(t, err.Error(), "expected object path")
	})
	t.Run("wrong type", func(t *testing.T) {
		_, err := parseObjectPathList("not-an-array")
		require.Error(t, err)
		require.Contains(t, err.Error(), "expected object path array")
	})
}

func TestParseStationObjectPath(t *testing.T) {
	t.Parallel()

	t.Run("nil yields nil", func(t *testing.T) {
		got, err := parseStationObjectPath("ConnectedNetwork", nil)
		require.NoError(t, err)
		require.Nil(t, got)
	})
	t.Run("typed path", func(t *testing.T) {
		got, err := parseStationObjectPath("ConnectedNetwork", dbus.ObjectPath("/net/0"))
		require.NoError(t, err)
		require.Equal(t, "/net/0", *got)
	})
	t.Run("string path", func(t *testing.T) {
		got, err := parseStationObjectPath("ConnectedNetwork", "/net/0")
		require.NoError(t, err)
		require.Equal(t, "/net/0", *got)
	})
	t.Run("variant unwraps", func(t *testing.T) {
		got, err := parseStationObjectPath("ConnectedNetwork", dbus.MakeVariant(dbus.ObjectPath("/net/0")))
		require.NoError(t, err)
		require.Equal(t, "/net/0", *got)
	})
	t.Run("root sentinel yields nil", func(t *testing.T) {
		got, err := parseStationObjectPath("ConnectedNetwork", dbus.ObjectPath("/"))
		require.NoError(t, err)
		require.Nil(t, got)
	})
	t.Run("invalid path", func(t *testing.T) {
		_, err := parseStationObjectPath("ConnectedNetwork", "bad")
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid object path")
	})
	t.Run("wrong type", func(t *testing.T) {
		_, err := parseStationObjectPath("ConnectedNetwork", 42)
		require.Error(t, err)
		require.Contains(t, err.Error(), "expected object path")
	})
}

func TestParseStationAffinities(t *testing.T) {
	t.Parallel()

	t.Run("nil yields nil", func(t *testing.T) {
		got, err := parseStationAffinities(nil)
		require.NoError(t, err)
		require.Nil(t, got)
	})
	t.Run("variant unwraps", func(t *testing.T) {
		got, err := parseStationAffinities(dbus.MakeVariant([]dbus.ObjectPath{"/a"}))
		require.NoError(t, err)
		require.Equal(t, []string{"/a"}, got)
	})
	t.Run("typed slice", func(t *testing.T) {
		got, err := parseStationAffinities([]dbus.ObjectPath{"/a", "/b"})
		require.NoError(t, err)
		require.Equal(t, []string{"/a", "/b"}, got)
	})
	t.Run("typed slice empty non-nil", func(t *testing.T) {
		got, err := parseStationAffinities([]dbus.ObjectPath{})
		require.NoError(t, err)
		require.NotNil(t, got)
		require.Empty(t, got)
	})
	t.Run("typed slice invalid element", func(t *testing.T) {
		_, err := parseStationAffinities([]dbus.ObjectPath{"bad"})
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid object path")
	})
	t.Run("interface slice", func(t *testing.T) {
		got, err := parseStationAffinities([]interface{}{dbus.ObjectPath("/a")})
		require.NoError(t, err)
		require.Equal(t, []string{"/a"}, got)
	})
	t.Run("interface slice wrong element type", func(t *testing.T) {
		_, err := parseStationAffinities([]interface{}{"/a"})
		require.Error(t, err)
		require.Contains(t, err.Error(), "expected object path")
	})
	t.Run("interface slice invalid element", func(t *testing.T) {
		_, err := parseStationAffinities([]interface{}{dbus.ObjectPath("bad")})
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid object path")
	})
	t.Run("wrong type", func(t *testing.T) {
		_, err := parseStationAffinities(42)
		require.Error(t, err)
		require.Contains(t, err.Error(), "expected object path array")
	})
}

func TestParseNetworkType_WrongType(t *testing.T) {
	t.Parallel()
	_, err := parseNetworkType(42)
	require.Error(t, err)
	require.Contains(t, err.Error(), "expected string")
}

func TestParseSupportedModes(t *testing.T) {
	t.Parallel()

	t.Run("string slice valid", func(t *testing.T) {
		got, err := parseSupportedModes([]string{"station", "ap"})
		require.NoError(t, err)
		require.Equal(t, []Mode{ModeStation, ModeAP}, got)
	})
	t.Run("string slice invalid mode", func(t *testing.T) {
		_, err := parseSupportedModes([]string{"bogus"})
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid mode")
	})
	t.Run("interface slice valid", func(t *testing.T) {
		got, err := parseSupportedModes([]interface{}{"station", "ad-hoc"})
		require.NoError(t, err)
		require.Equal(t, []Mode{ModeStation, ModeAdHoc}, got)
	})
	t.Run("interface slice wrong element type", func(t *testing.T) {
		_, err := parseSupportedModes([]interface{}{42})
		require.Error(t, err)
		require.Contains(t, err.Error(), "expected string")
	})
	t.Run("interface slice invalid mode", func(t *testing.T) {
		_, err := parseSupportedModes([]interface{}{"bogus"})
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid mode")
	})
	t.Run("wrong type", func(t *testing.T) {
		_, err := parseSupportedModes(42)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unexpected type")
	})
}

func TestParseOptionalString_VariantNil(t *testing.T) {
	t.Parallel()
	// A variant whose inner value is nil (the zero Variant) collapses to (nil, nil).
	got, err := parseOptionalString(dbus.Variant{})
	require.NoError(t, err)
	require.Nil(t, got)
}

func TestSplitSignalName(t *testing.T) {
	t.Parallel()

	t.Run("dotted name splits into iface and member", func(t *testing.T) {
		iface, member := splitSignalName("net.connman.iwd.Station.ScanningChanged")
		require.Equal(t, "net.connman.iwd.Station", iface)
		require.Equal(t, "ScanningChanged", member)
	})

	t.Run("no dot returns name and empty member", func(t *testing.T) {
		iface, member := splitSignalName("Bare")
		require.Equal(t, "Bare", iface)
		require.Equal(t, "", member)
	})
}

func TestParseObjectPath_Device(t *testing.T) {
	t.Parallel()

	t.Run("typed valid", func(t *testing.T) {
		got, err := parseObjectPath(dbus.ObjectPath("/net/connman/iwd/0"))
		require.NoError(t, err)
		require.Equal(t, dbus.ObjectPath("/net/connman/iwd/0"), got)
	})
	t.Run("typed invalid", func(t *testing.T) {
		_, err := parseObjectPath(dbus.ObjectPath("bad"))
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid object path")
	})
	t.Run("string valid", func(t *testing.T) {
		got, err := parseObjectPath("/net/connman/iwd/0")
		require.NoError(t, err)
		require.Equal(t, dbus.ObjectPath("/net/connman/iwd/0"), got)
	})
	t.Run("string invalid", func(t *testing.T) {
		_, err := parseObjectPath("bad")
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid object path")
	})
	t.Run("wrong type", func(t *testing.T) {
		_, err := parseObjectPath(42)
		require.Error(t, err)
		require.Contains(t, err.Error(), "expected object path")
	})
}
